/*
Copyright 2026 Amidaware Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"archive/zip"
	"compress/flate"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

const (
	defaultExtractMaxFiles     = 10_000
	defaultExtractMaxSizeBytes = int64(4 * 1024 * 1024 * 1024)
	defaultExtractMaxDepth     = 32
	zipFlagEncrypted           = 0x1
	extractJournalPrefix       = "trmm-extract-"
	extractProgressHeartbeat   = 60 * time.Second
	extractCancelWait          = 3 * time.Second
	extractDoneRetryWindow = 5 * time.Minute
)

var extractProgressInterval = 2 * time.Second

var (
	errExtractNoSpace = errors.New("Not enough free disk space at the destination.")
	errExtractCorrupt = errors.New("The ZIP is damaged: an entry failed its integrity check.")
)

var windowsReservedNames = func() map[string]struct{} {
	m := map[string]struct{}{"CON": {}, "PRN": {}, "AUX": {}, "NUL": {}}
	for i := 0; i <= 9; i++ {
		m[fmt.Sprintf("COM%d", i)] = struct{}{}
		m[fmt.Sprintf("LPT%d", i)] = struct{}{}
	}
	return m
}()

type extractEntry struct {
	file  *zip.File
	name  string
	segs  []string
	isDir bool
}

// extractProgress is written by the extractor and read by the progress
// reporter and the cancel handler.
type extractProgress struct {
	written    atomic.Int64
	total      atomic.Int64
	filesDone  atomic.Int64
	filesTotal atomic.Int64
	extracted  atomic.Int64
	skipped    atomic.Int64
}

type extractJob struct {
	ctx      context.Context
	cancel   context.CancelFunc
	progress *extractProgress
	done     chan struct{}
}

var (
	extractSlots  = make(chan struct{}, 2)
	extractJobsMu sync.Mutex
	extractJobs   = map[string]*extractJob{}
)

func registerExtractJob(sessionID string) *extractJob {
	ctx, cancel := context.WithCancel(context.Background())
	job := &extractJob{ctx: ctx, cancel: cancel, progress: &extractProgress{}, done: make(chan struct{})}
	extractJobsMu.Lock()
	if prev, ok := extractJobs[sessionID]; ok {
		prev.cancel()
	}
	extractJobs[sessionID] = job
	extractJobsMu.Unlock()
	return job
}

func lookupExtractJob(sessionID string) *extractJob {
	extractJobsMu.Lock()
	defer extractJobsMu.Unlock()
	return extractJobs[sessionID]
}

func unregisterExtractJob(sessionID string, job *extractJob) {
	extractJobsMu.Lock()
	if extractJobs[sessionID] == job {
		delete(extractJobs, sessionID)
	}
	extractJobsMu.Unlock()
	job.cancel()
}

type extractOptions struct {
	policy     string
	limits     archiveLimits
	journalDir string
	// partialTag makes partial names unique to the session.
	partialTag   string
	progress     *extractProgress
	trackPartial func(partial string)
	goos         string
}

type extractResult struct {
	files    int
	skipped  int
	warnings []string
}

type countingWriter struct {
	w io.Writer
	n *atomic.Int64
}

func (c countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n.Add(int64(n))
	return n, err
}

func formatExtractSize(n int64) string {
	const gib, mib = int64(1 << 30), int64(1 << 20)
	switch {
	case n >= gib && n%gib == 0:
		return fmt.Sprintf("%d GB", n/gib)
	case n >= mib && n%mib == 0:
		return fmt.Sprintf("%d MB", n/mib)
	}
	return fmt.Sprintf("%d bytes", n)
}

func extractEntryName(f *zip.File) string {
	if f.NonUTF8 && !utf8.ValidString(f.Name) {
		if decoded, err := charmap.CodePage437.NewDecoder().String(f.Name); err == nil {
			return decoded
		}
	}
	return f.Name
}

// extractEntryModTime returns the entry's modified time. archive/zip reads a
// DOS-only timestamp (no extended or NTFS time field, as written by Windows'
// own zip tools) as UTC, but DOS times are local wall-clock time.
func extractEntryModTime(f *zip.File) time.Time {
	mod := f.Modified
	if mod.IsZero() {
		return mod
	}
	for extra := f.Extra; len(extra) >= 4; {
		id := binary.LittleEndian.Uint16(extra)
		size := int(binary.LittleEndian.Uint16(extra[2:]))
		if id == 0x5455 || id == 0x000a {
			return mod
		}
		// Info-ZIP Unix fields (macOS ditto/Finder writes 0x5855) also carry
		// the real time, which archive/zip uses.
		if (id == 0x5855 || id == 0x000d) && size >= 8 {
			return mod
		}
		if 4+size > len(extra) {
			break
		}
		extra = extra[4+size:]
	}
	return time.Date(mod.Year(), mod.Month(), mod.Day(), mod.Hour(), mod.Minute(), mod.Second(), 0, time.Local)
}

// extractEntryPerm keeps the permission bits a Unix or macOS zip tool
// recorded (so a private 0600 file stays private), without setuid, setgid or
// sticky. Other tools only record read-only/writable, so those get 0644, or
// 0755 when marked executable.
func extractEntryPerm(f *zip.File) os.FileMode {
	switch f.CreatorVersion >> 8 {
	case 3, 19: // unix, macOS
		return f.Mode().Perm()
	}
	if f.Mode()&0o111 != 0 {
		return 0o755
	}
	return 0o644
}

func isWindowsReservedName(name string) bool {
	base := strings.TrimRight(strings.SplitN(name, ".", 2)[0], " ")
	_, ok := windowsReservedNames[strings.ToUpper(base)]
	return ok
}

func validateExtractSegment(seg, goos string) error {
	if seg == "." || seg == ".." {
		return fmt.Errorf("path traversal")
	}
	if hasTrailingSpaceOrPeriod(seg, goos) {
		return fmt.Errorf("name ends with a space or a period")
	}
	if goos == "windows" && isWindowsReservedName(seg) {
		return fmt.Errorf("name is reserved by Windows")
	}
	if len(seg) > maxFileNameBytes {
		return fmt.Errorf("name is too long")
	}
	for _, r := range seg {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"/\|?*`, r) {
			return fmt.Errorf("invalid name")
		}
	}
	return nil
}

func splitExtractEntryName(raw, goos string) ([]string, bool, error) {
	name := strings.ReplaceAll(raw, `\`, "/")
	name = strings.TrimPrefix(name, "./")
	isDir := strings.HasSuffix(name, "/")
	if strings.HasPrefix(name, "/") {
		return nil, isDir, fmt.Errorf("absolute path")
	}
	if strings.Contains(name, ":") {
		return nil, isDir, fmt.Errorf("drive letter or colon in path")
	}
	var segs []string
	for _, seg := range strings.Split(name, "/") {
		if seg == "" {
			continue
		}
		if err := validateExtractSegment(seg, goos); err != nil {
			return nil, isDir, err
		}
		segs = append(segs, seg)
	}
	if len(segs) == 0 {
		return nil, isDir, fmt.Errorf("empty name")
	}
	return segs, isDir, nil
}

func planZipExtract(
	files []*zip.File, limits archiveLimits, goos string,
) (entries []extractEntry, total int64, fileCount int, warnings []string, err error) {
	fold := goos == "windows" || goos == "darwin"
	key := func(segs []string) string {
		k := path.Join(segs...)
		if fold {
			k = strings.ToLower(k)
		}
		return k
	}
	isDirKey := make(map[string]bool)

	for _, f := range files {
		name := extractEntryName(f)
		mode := f.Mode()
		isDir := mode.IsDir() || strings.HasSuffix(strings.ReplaceAll(name, `\`, "/"), "/")
		if !isDir && f.Flags&zipFlagEncrypted != 0 {
			return nil, 0, 0, warnings, fmt.Errorf("This ZIP is password-protected, which isn't supported.")
		}
		segs, _, perr := splitExtractEntryName(name, goos)
		if perr != nil {
			warnings = appendArchiveWarning(warnings, fmt.Sprintf("skipped %q: %v", name, perr))
			continue
		}
		display := path.Join(segs...)
		if mode&os.ModeSymlink != 0 {
			warnings = appendArchiveWarning(warnings, fmt.Sprintf("skipped symlink %s", display))
			continue
		}
		if !isDir && !mode.IsRegular() {
			warnings = appendArchiveWarning(warnings, fmt.Sprintf("skipped %s: not a regular file", display))
			continue
		}
		if len(segs) > limits.maxDepth {
			return nil, 0, 0, warnings, fmt.Errorf(
				"The ZIP has folders more than %d levels deep, the extraction limit.", limits.maxDepth,
			)
		}
		if !isDir && f.Method != zip.Store && f.Method != zip.Deflate {
			return nil, 0, 0, warnings, fmt.Errorf(
				"The ZIP uses a compression method that isn't supported (only Store and Deflate are).",
			)
		}

		conflict := false
		for i := 1; i < len(segs); i++ {
			if dir, ok := isDirKey[key(segs[:i])]; ok && !dir {
				conflict = true
				break
			}
		}
		k := key(segs)
		if dir, ok := isDirKey[k]; ok {
			if dir && isDir {
				continue
			}
			conflict = true
		}
		if conflict {
			warnings = appendArchiveWarning(warnings, fmt.Sprintf("skipped duplicate %s", display))
			continue
		}
		for i := 1; i < len(segs); i++ {
			isDirKey[key(segs[:i])] = true
		}
		isDirKey[k] = isDir

		if !isDir {
			fileCount++
			if fileCount > limits.maxFiles {
				return nil, 0, 0, warnings, fmt.Errorf(
					"The ZIP has more than %d files, the extraction limit.", limits.maxFiles,
				)
			}
			if f.UncompressedSize64 > uint64(limits.maxSizeBytes-total) {
				return nil, 0, 0, warnings, fmt.Errorf(
					"The ZIP is larger than the %s extraction limit.", formatExtractSize(limits.maxSizeBytes),
				)
			}
			total += int64(f.UncompressedSize64)
		}
		entries = append(entries, extractEntry{file: f, name: display, segs: segs, isDir: isDir})
	}
	return entries, total, fileCount, warnings, nil
}

func nearestExistingDir(p string) string {
	for {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return p
		}
		p = parent
	}
}

func prepareExtractDestination(raw string, needed int64) (string, error) {
	dest, err := validateUploadDestinationPath(raw)
	if err != nil {
		return "", fmt.Errorf("invalid destination path")
	}
	info, err := os.Lstat(dest)
	switch {
	case err == nil:
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return "", fmt.Errorf("The destination is a link, not a folder.")
		}
		if !info.IsDir() {
			return "", fmt.Errorf("The destination exists and is not a folder.")
		}
	case !os.IsNotExist(err):
		return "", fmt.Errorf("unable to access the destination: %s", osErrorReason(err))
	}
	if needed > 0 {
		if usage, uerr := diskUsage(nearestExistingDir(dest)); uerr == nil && usage != nil &&
			int64(usage.Free) < needed+fileTransferDiskSpaceMargin {
			return "", errExtractNoSpace
		}
	}
	if err != nil {
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return "", fmt.Errorf("unable to create the destination: %s", osErrorReason(err))
		}
	}
	return dest, nil
}

func ensureExtractDirs(dest string, segs []string) error {
	cur := dest
	for _, seg := range segs {
		cur = filepath.Join(cur, seg)
		for attempt := 0; ; attempt++ {
			info, err := os.Lstat(cur)
			if err == nil {
				if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
					return fmt.Errorf("%s is a link, not a folder", seg)
				}
				if !info.IsDir() {
					return fmt.Errorf("a file named %s is in the way", seg)
				}
				break
			}
			if !os.IsNotExist(err) {
				return fmt.Errorf("%s", osErrorReason(err))
			}
			if err := os.Mkdir(cur, 0o755); err == nil {
				break
			} else if !os.IsExist(err) || attempt > 0 {
				if isNoSpaceError(err) {
					return errExtractNoSpace
				}
				return fmt.Errorf("%s", osErrorReason(err))
			}
		}
	}
	return nil
}

func isFlateCorrupt(err error) bool {
	var corrupt flate.CorruptInputError
	return errors.As(err, &corrupt)
}

func removeExtractPartial(out *os.File, partial string) {
	if out != nil {
		_ = out.Close()
	}
	_ = os.Remove(partial)
}

func extractZipFile(
	ctx context.Context, e extractEntry, target string, opts extractOptions,
) (skipped bool, warning string, err error) {
	existing := func() (bool, string) {
		info, lerr := os.Lstat(target)
		if lerr != nil {
			if os.IsNotExist(lerr) {
				return false, ""
			}
			return false, osErrorReason(lerr)
		}
		if info.IsDir() {
			return true, "a folder with that name already exists"
		}
		if !info.Mode().IsRegular() {
			return true, "a link or special file with that name already exists"
		}
		return true, ""
	}
	if exists, why := existing(); why != "" {
		return false, why, nil
	} else if exists && opts.policy == uploadConflictPolicySkip {
		return true, "", nil
	}

	partial := siblingWithSuffix(target, "."+opts.partialTag+".partial")
	if opts.trackPartial != nil {
		opts.trackPartial(partial)
	}
	out, err := createExclusiveRegularFile(partial, extractEntryPerm(e.file))
	if err != nil {
		if isNoSpaceError(err) {
			return false, "", errExtractNoSpace
		}
		return false, osErrorReason(err), nil
	}

	rc, err := e.file.Open()
	if err != nil {
		removeExtractPartial(out, partial)
		return false, "", errExtractCorrupt
	}
	declared := int64(e.file.UncompressedSize64)
	n, err := io.Copy(
		countingWriter{w: out, n: &opts.progress.written},
		cancelAwareReader{ctx: ctx, r: io.LimitReader(rc, declared+1)},
	)
	_ = rc.Close()
	// archive/zip itself stops an entry at its declared size with ErrFormat.
	switch {
	case (err == nil && n > declared) || errors.Is(err, zip.ErrFormat):
		err = fmt.Errorf("The ZIP's contents are larger than its headers declare, so extraction was stopped.")
	case archiveCanceledErr(err):
		err = errArchiveCanceled
	case isNoSpaceError(err):
		err = errExtractNoSpace
	case errors.Is(err, zip.ErrChecksum), errors.Is(err, io.ErrUnexpectedEOF), isFlateCorrupt(err):
		err = errExtractCorrupt
	case err != nil:
		err = fmt.Errorf("failed to write %s: %s", e.name, osErrorReason(err))
	}
	if err != nil {
		removeExtractPartial(out, partial)
		return false, "", err
	}
	if err := out.Close(); err != nil {
		removeExtractPartial(nil, partial)
		if isNoSpaceError(err) {
			return false, "", errExtractNoSpace
		}
		return false, "", fmt.Errorf("failed to write %s: %s", e.name, osErrorReason(err))
	}
	if mod := extractEntryModTime(e.file); !mod.IsZero() {
		_ = os.Chtimes(partial, mod, mod)
	}

	// something may have appeared at the target while we were writing
	if exists, why := existing(); why != "" {
		removeExtractPartial(nil, partial)
		return false, why, nil
	} else if exists && opts.policy == uploadConflictPolicySkip {
		removeExtractPartial(nil, partial)
		return true, "", nil
	}
	if err := replaceUploadPartialWithDestination(partial, target, opts.journalDir); err != nil {
		removeExtractPartial(nil, partial)
		return false, osErrorReason(err), nil
	}
	return false, "", nil
}

func extractZip(ctx context.Context, source, dest string, opts extractOptions) (extractResult, error) {
	var res extractResult
	ctx = archiveContext(ctx)
	if opts.goos == "" {
		opts.goos = runtime.GOOS
	}
	if opts.progress == nil {
		opts.progress = &extractProgress{}
	}
	prog := opts.progress

	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return res, fmt.Errorf("The ZIP file was not found.")
		}
		return res, fmt.Errorf("unable to open the ZIP: %s", osErrorReason(err))
	}
	if !info.Mode().IsRegular() {
		return res, fmt.Errorf("The ZIP is not a regular file.")
	}
	zr, err := zip.OpenReader(source)
	if err != nil {
		if os.IsPermission(err) {
			return res, fmt.Errorf("unable to open the ZIP: permission denied")
		}
		return res, fmt.Errorf("The file isn't a valid ZIP.")
	}
	defer zr.Close()

	entries, total, fileCount, warnings, err := planZipExtract(zr.File, opts.limits, opts.goos)
	res.warnings = warnings
	if err != nil {
		return res, err
	}
	prog.total.Store(total)
	prog.filesTotal.Store(int64(fileCount))
	if err := checkArchiveCanceled(ctx); err != nil {
		return res, err
	}

	dest, err = prepareExtractDestination(dest, total)
	if err != nil {
		return res, err
	}

	for _, e := range entries {
		if err := checkArchiveCanceled(ctx); err != nil {
			return res, err
		}
		target := filepath.Join(append([]string{dest}, e.segs...)...)
		if rel, rerr := filepath.Rel(dest, target); rerr != nil || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			res.warnings = appendArchiveWarning(res.warnings, fmt.Sprintf("skipped %s: outside the destination", e.name))
			continue
		}

		if e.isDir {
			if err := ensureExtractDirs(dest, e.segs); err != nil {
				if errors.Is(err, errExtractNoSpace) {
					return res, err
				}
				res.warnings = appendArchiveWarning(res.warnings, fmt.Sprintf("skipped %s: %v", e.name, err))
			}
			continue
		}

		base := prog.written.Load()
		skipped, warning, err := false, "", ensureExtractDirs(dest, e.segs[:len(e.segs)-1])
		if err != nil {
			if errors.Is(err, errExtractNoSpace) {
				return res, err
			}
			warning = err.Error()
		} else {
			skipped, warning, err = extractZipFile(ctx, e, target, opts)
			if err != nil {
				return res, err
			}
		}
		switch {
		case warning != "":
			res.warnings = appendArchiveWarning(res.warnings, fmt.Sprintf("skipped %s: %s", e.name, warning))
		case skipped:
			res.skipped++
			prog.skipped.Add(1)
		default:
			res.files++
			prog.extracted.Add(1)
		}
		prog.written.Store(base + int64(e.file.UncompressedSize64))
		prog.filesDone.Add(1)
	}
	// e.g. no write access at the destination: report it, not "0 files"
	if res.files == 0 && res.skipped == 0 && len(res.warnings) > 0 {
		return res, fmt.Errorf("Nothing could be extracted (%s).", strings.TrimPrefix(res.warnings[0], "skipped "))
	}
	return res, nil
}

func extractPartialTag(sessionID string) string {
	tag := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, sessionID)
	if len(tag) > 8 {
		tag = tag[:8]
	}
	return tag
}

func parseExtractLimits(data map[string]string) archiveLimits {
	limits := archiveLimits{
		maxFiles:     defaultExtractMaxFiles,
		maxSizeBytes: defaultExtractMaxSizeBytes,
		maxDepth:     defaultExtractMaxDepth,
	}
	if v, err := parsePayloadInt64(data, "max_files"); err == nil && v > 0 {
		limits.maxFiles = clampAtMostInt(int(v), defaultExtractMaxFiles)
	}
	if v, err := parsePayloadInt64(data, "max_size_bytes"); err == nil && v > 0 {
		limits.maxSizeBytes = clampAtMostInt64(v, defaultExtractMaxSizeBytes)
	}
	if v, err := parsePayloadInt64(data, "max_depth"); err == nil && v > 0 {
		limits.maxDepth = clampAtMostInt(int(v), defaultExtractMaxDepth)
	}
	return limits
}

func (a *Agent) PrepareFilesExtract(p *NatsMsg) (map[string]interface{}, error) {
	sessionID, err := parsePayloadString(p.Data, "session_id")
	if err != nil {
		return nil, err
	}
	if extractPartialTag(sessionID) == "" {
		return nil, fmt.Errorf("invalid session_id")
	}
	source, err := validateUploadDestinationPath(p.Data["source_path"])
	if err != nil {
		return nil, fmt.Errorf("invalid source_path")
	}
	if !strings.EqualFold(filepath.Ext(source), ".zip") {
		return nil, fmt.Errorf("source_path must be a .zip file")
	}
	dest, err := validateUploadDestinationPath(p.Data["destination_path"])
	if err != nil {
		return nil, fmt.Errorf("invalid destination_path")
	}
	policy := uploadConflictPolicySkip
	if strings.EqualFold(strings.TrimSpace(p.Data["conflict_policy"]), uploadConflictPolicyReplace) {
		policy = uploadConflictPolicyReplace
	}

	job := registerExtractJob(sessionID)
	go a.runExtract(job, sessionID, source, dest, policy, parseExtractLimits(p.Data))
	return map[string]interface{}{"status": "extracting"}, nil
}

func (a *Agent) CancelFilesExtract(p *NatsMsg) (map[string]interface{}, error) {
	sessionID, err := parsePayloadString(p.Data, "session_id")
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{"status": "cancelled"}
	job := lookupExtractJob(sessionID)
	if job == nil {
		return out, nil
	}
	job.cancel()
	// wait briefly so the counts we return are final.
	select {
	case <-job.done:
	case <-time.After(extractCancelWait):
	}
	out["files"] = job.progress.extracted.Load()
	out["skipped"] = job.progress.skipped.Load()
	return out, nil
}

func (a *Agent) runExtract(job *extractJob, sessionID, source, dest, policy string, limits archiveLimits) {
	defer a.recoverFilesRPC("files_extract", nil)
	defer unregisterExtractJob(sessionID, job)

	stopReporter := make(chan struct{})
	reporterDone := make(chan struct{})
	go func() {
		defer close(reporterDone)
		a.reportExtractProgress(job, sessionID, stopReporter)
	}()

	res, err := a.extractWithSlot(job, sessionID, source, dest, policy, limits)
	close(job.done)
	close(stopReporter)
	<-reporterDone

	if archiveCanceledErr(err) || job.ctx.Err() != nil {
		a.Logger.Debugf("file_transfer extract cancelled session=%s files=%d", sessionID, res.files)
		return
	}
	payload := map[string]interface{}{
		"files":       res.files,
		"skipped":     res.skipped,
		"total":       job.progress.total.Load(),
		"files_total": job.progress.filesTotal.Load(),
	}
	if len(res.warnings) > 0 {
		payload["warnings"] = res.warnings
	}
	if err != nil {
		a.Logger.Debugf("file_transfer extract session=%s failed: %v", sessionID, err)
		payload["error"] = err.Error()
	} else {
		a.Logger.Debugf(
			"file_transfer extract session=%s done files=%d skipped=%d warnings=%d",
			sessionID, res.files, res.skipped, len(res.warnings),
		)
	}
	a.reportExtractDone(job.ctx, sessionID, payload)
}


func (a *Agent) extractWithSlot(
	job *extractJob, sessionID, source, dest, policy string, limits archiveLimits,
) (extractResult, error) {
	select {
	case extractSlots <- struct{}{}:
		defer func() { <-extractSlots }()
	case <-job.ctx.Done():
		return extractResult{}, errArchiveCanceled
	}
	defer a.removeExtractJournal(sessionID)
	return extractZip(job.ctx, source, dest, extractOptions{
		policy:     policy,
		limits:     limits,
		journalDir: a.fileTransferStateDir(),
		partialTag: extractPartialTag(sessionID),
		progress:   job.progress,
		trackPartial: func(partial string) {
			a.writeExtractJournal(sessionID, partial)
		},
	})
}

func (a *Agent) reportExtractProgress(job *extractJob, sessionID string, stop <-chan struct{}) {
	url := fmt.Sprintf("/api/v3/file-transfers/%s/extract-progress/", sessionID)
	ticker := time.NewTicker(extractProgressInterval)
	defer ticker.Stop()
	var last [4]int64
	lastPost := time.Now()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		prog := job.progress
		cur := [4]int64{prog.written.Load(), prog.total.Load(), prog.filesDone.Load(), prog.filesTotal.Load()}
		if cur == last && time.Since(lastPost) < extractProgressHeartbeat {
			continue
		}
		reqCtx, cancel := context.WithTimeout(job.ctx, fileTransferFailTimeout)
		resp, err := a.fileTransferHTTP().R().SetContext(reqCtx).SetBody(map[string]interface{}{
			"written":     cur[0],
			"total":       cur[1],
			"files_done":  cur[2],
			"files_total": cur[3],
		}).Post(url)
		cancel()
		if err != nil {
			a.Logger.Debugf("file_transfer extract progress session=%s err=%v", sessionID, err)
			continue
		}
		switch code := resp.StatusCode(); {
		case code == 200:
			last, lastPost = cur, time.Now()
		case code == 404 || code == 409:
			// Cancelled or expired on the server.
			a.Logger.Debugf("file_transfer extract progress session=%s status=%d, stopping", sessionID, code)
			job.cancel()
			return
		default:
			a.Logger.Debugf("file_transfer extract progress session=%s status=%d", sessionID, code)
		}
	}
}

func (a *Agent) reportExtractDone(ctx context.Context, sessionID string, payload map[string]interface{}) {
	url := fmt.Sprintf("/api/v3/file-transfers/%s/extract-done/", sessionID)
	deadline := time.Now().Add(extractDoneRetryWindow)
	backoff := 2 * time.Second
	for attempt := 1; ; attempt++ {
		reqCtx, cancel := context.WithTimeout(context.Background(), fileTransferFailTimeout)
		resp, err := a.fileTransferHTTP().R().SetContext(reqCtx).SetBody(payload).Post(url)
		cancel()
		if err == nil && resp.StatusCode() == 200 {
			return
		}
		if err == nil && resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
			a.Logger.Debugf(
				"file_transfer extract-done callback session=%s status=%d body=%s (not retrying)",
				sessionID, resp.StatusCode(), string(resp.Body()),
			)
			return
		}
		if time.Now().After(deadline) {
			if err != nil {
				a.Logger.Errorf(
					"file_transfer extract-done callback session=%s failed after %d attempts err=%v",
					sessionID, attempt, err,
				)
			} else {
				a.Logger.Errorf(
					"file_transfer extract-done callback session=%s failed after %d attempts status=%d",
					sessionID, attempt, resp.StatusCode(),
				)
			}
			return
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff *= 2; backoff > 15*time.Second {
			backoff = 15 * time.Second
		}
	}
}

func (a *Agent) extractJournalPath(sessionID string) string {
	dir := a.fileTransferStateDir()
	if strings.TrimSpace(dir) == "" || sessionID == "" || filepath.Base(sessionID) != sessionID {
		return ""
	}
	return filepath.Join(dir, extractJournalPrefix+sessionID+".json")
}

func (a *Agent) writeExtractJournal(sessionID, partial string) {
	path := a.extractJournalPath(sessionID)
	if path == "" {
		return
	}
	data, err := json.Marshal(uploadPartialJournal{SessionID: sessionID, Partial: partial})
	if err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		a.Logger.Debugf("file_transfer extract session=%s: partial journal not written: %v", sessionID, err)
	}
}

func (a *Agent) removeExtractJournal(sessionID string) {
	if path := a.extractJournalPath(sessionID); path != "" {
		_ = os.Remove(path)
	}
}

func (a *Agent) sweepOrphanedExtractPartials() int {
	dir := a.fileTransferStateDir()
	if strings.TrimSpace(dir) == "" {
		return 0
	}
	matches, _ := filepath.Glob(filepath.Join(dir, extractJournalPrefix+"*.json"))
	removed := 0
	for _, journalPath := range matches {
		j, ok := readUploadPartialJournal(journalPath)
		if ok && lookupExtractJob(j.SessionID) != nil {
			continue
		}
		if ok && strings.HasSuffix(j.Partial, "."+extractPartialTag(j.SessionID)+".partial") {
			if info, err := os.Lstat(j.Partial); err == nil && info.Mode().IsRegular() {
				if err := os.Remove(j.Partial); err != nil {
					a.Logger.Errorf("file_transfer: failed to remove orphaned extract partial %s: %v", j.Partial, err)
					continue
				}
				removed++
				a.Logger.Debugf("file_transfer: removed orphaned extract partial %s (session=%s)", j.Partial, j.SessionID)
			}
		}
		_ = os.Remove(journalPath)
	}
	return removed
}
