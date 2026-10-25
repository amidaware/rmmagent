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
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultArchiveMaxPaths     = 100
	defaultArchiveMaxFiles     = 10_000
	defaultArchiveMaxSizeBytes = int64(4 * 1024 * 1024 * 1024) // 4 GiB; ZIP64 when exceeded
	defaultArchiveMaxDepth     = 32
	// Prefix used for all temp archive files so a startup sweep can reclaim orphans.
	archiveTempPrefix  = "trmm-archive-"
	maxArchiveWarnings = 50
)

var (
	errArchiveCanceled = errors.New("archive cancelled")

	archiveBuildSlots = make(chan struct{}, 2)
	archiveBuildsMu   sync.Mutex
	archiveBuilds     = map[string]archiveBuild{}
)

// archiveBuild is one registered build; ctx identifies it so a finishing
// build cannot unregister a newer one for the same session.
type archiveBuild struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func registerArchiveBuild(sessionID string) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	archiveBuildsMu.Lock()
	if prev, ok := archiveBuilds[sessionID]; ok && prev.cancel != nil {
		prev.cancel()
	}
	archiveBuilds[sessionID] = archiveBuild{ctx: ctx, cancel: cancel}
	archiveBuildsMu.Unlock()
	return ctx
}

func cancelArchiveBuild(sessionID string) bool {
	archiveBuildsMu.Lock()
	build, ok := archiveBuilds[sessionID]
	archiveBuildsMu.Unlock()
	if !ok || build.cancel == nil {
		return false
	}
	build.cancel()
	return true
}

func unregisterArchiveBuild(sessionID string, ctx context.Context) {
	archiveBuildsMu.Lock()
	if build, ok := archiveBuilds[sessionID]; ok && build.ctx == ctx {
		delete(archiveBuilds, sessionID)
	}
	archiveBuildsMu.Unlock()
}

func archiveContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func checkArchiveCanceled(ctx context.Context) error {
	if archiveContext(ctx).Err() != nil {
		return errArchiveCanceled
	}
	return nil
}

func archiveCanceledErr(err error) bool {
	return err != nil && (errors.Is(err, errArchiveCanceled) || errors.Is(err, context.Canceled))
}

type cancelAwareReader struct {
	ctx context.Context
	r   io.Reader
}

func (r cancelAwareReader) Read(p []byte) (int, error) {
	if err := checkArchiveCanceled(r.ctx); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func ensureArchiveDiskSpace(dir string, needed int64) error {
	return ensureDiskSpace(dir, needed, "to build archive")
}

type archiveLimits struct {
	maxFiles     int
	maxSizeBytes int64
	maxDepth     int
}

type archiveFileEntry struct {
	absPath string
	zipName string
	size    int64
}

func parsePayloadPathsJSON(data map[string]string) ([]string, error) {
	raw, err := parsePayloadString(data, "paths_json")
	if err != nil {
		return nil, err
	}
	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		return nil, fmt.Errorf("invalid paths_json")
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("paths_json must contain at least one path")
	}
	cleaned := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		validated, err := validateUploadDestinationPath(p)
		if err != nil {
			return nil, fmt.Errorf("invalid path %q: %w", p, err)
		}
		key := fsIdentityKey(validated)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, validated)
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("paths_json must contain at least one valid path")
	}
	if len(cleaned) > defaultArchiveMaxPaths {
		return nil, fmt.Errorf("too many paths (max %d)", defaultArchiveMaxPaths)
	}
	return cleaned, nil
}

func parseArchiveLimits(data map[string]string) archiveLimits {
	limits := archiveLimits{
		maxFiles:     defaultArchiveMaxFiles,
		maxSizeBytes: defaultArchiveMaxSizeBytes,
		maxDepth:     defaultArchiveMaxDepth,
	}
	if v, err := parsePayloadInt64(data, "max_files"); err == nil && v > 0 {
		limits.maxFiles = clampAtMostInt(int(v), defaultArchiveMaxFiles)
	}
	if v, err := parsePayloadInt64(data, "max_size_bytes"); err == nil && v > 0 {
		limits.maxSizeBytes = clampAtMostInt64(v, defaultArchiveMaxSizeBytes)
	}
	if v, err := parsePayloadInt64(data, "max_depth"); err == nil && v > 0 {
		limits.maxDepth = clampAtMostInt(int(v), defaultArchiveMaxDepth)
	}
	return limits
}

func sameFileTransferDir(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// fileTransferStateDir is where the journals go. Not /tmp, they need to
// survive a reboot.
func (a *Agent) fileTransferStateDir() string {
	if runtime.GOOS == "windows" || (a != nil && strings.TrimSpace(a.UnixTmpDir) != "") {
		return a.fileTransferTempDir()
	}
	dir := filepath.Join(nixAgentDir, "transfers")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return a.fileTransferTempDir()
	}
	return dir
}

func (a *Agent) fileTransferTempDir() string {
	if runtime.GOOS == "windows" {
		if a != nil && strings.TrimSpace(a.WinTmpDir) != "" {
			return a.WinTmpDir
		}
		return defaultWinTmpDir
	}
	if a != nil && strings.TrimSpace(a.UnixTmpDir) != "" {
		return a.UnixTmpDir
	}
	return os.TempDir()
}

func (a *Agent) ensureFileTransferTempDir() (string, error) {
	dir := a.fileTransferTempDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create file transfer temp dir %s: %w", dir, err)
	}
	return dir, nil
}

func (a *Agent) archiveTempPath(sessionID string) (string, error) {
	safeID := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return '_'
		}
	}, sessionID)
	dir, err := a.ensureFileTransferTempDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%s%s.zip", archiveTempPrefix, safeID)), nil
}

func isArchiveTempName(base string) bool {
	return strings.HasPrefix(base, archiveTempPrefix) && strings.HasSuffix(base, ".zip")
}

func (a *Agent) isArchiveTempPath(p string) bool {
	if p == "" {
		return false
	}
	base := filepath.Base(p)
	if !isArchiveTempName(base) {
		return false
	}
	dir := filepath.Dir(p)
	if sameFileTransferDir(dir, a.fileTransferTempDir()) {
		return true
	}
	return sameFileTransferDir(dir, os.TempDir())
}

func dedupeZipEntryName(name string, used map[string]struct{}) string {
	if _, ok := used[name]; !ok {
		used[name] = struct{}{}
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(filepath.Base(name), ext)
	dir := filepath.Dir(name)
	for i := 2; i < 10_000; i++ {
		candidateBase := fmt.Sprintf("%s_%d%s", base, i, ext)
		candidate := candidateBase
		if dir != "." && dir != "" {
			candidate = filepath.ToSlash(filepath.Join(dir, candidateBase))
		}
		if _, ok := used[candidate]; !ok {
			used[candidate] = struct{}{}
			return candidate
		}
	}
	fallback := fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext)
	used[fallback] = struct{}{}
	return fallback
}

func collectArchiveEntries(
	ctx context.Context,
	roots []string,
	limits archiveLimits,
) (files []archiveFileEntry, dirNames []string, warnings []string, totalBytes int64, err error) {
	usedNames := make(map[string]struct{})
	fileCount := 0
	depthTruncated := false
	ctx = archiveContext(ctx)

	for _, root := range roots {
		if err := checkArchiveCanceled(ctx); err != nil {
			return nil, nil, warnings, totalBytes, err
		}
		info, statErr := os.Lstat(root)
		if statErr != nil {
			return nil, nil, warnings, 0, fmt.Errorf("path not accessible: %s: %w", root, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, nil, warnings, 0, fmt.Errorf("symbolic links cannot be archived: %s", root)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			// fifos block on open and devices never end
			return nil, nil, warnings, 0, fmt.Errorf("not a regular file: %s", root)
		}

		rootBase := filepath.Base(root)
		if rootBase == "" || rootBase == "." || rootBase == string(filepath.Separator) {
			rootBase = "archive"
		}

		if info.IsDir() {
			rootZipPrefix := dedupeZipEntryName(filepath.ToSlash(rootBase), usedNames)
			dirNames = append(dirNames, rootZipPrefix+"/")

			walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
				if err := checkArchiveCanceled(ctx); err != nil {
					return err
				}
				if walkErr != nil {
					warnings = append(warnings, fmt.Sprintf("skipped %s: %v", path, walkErr))
					return nil
				}

				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				if rel == "." {
					return nil
				}

				depth := len(strings.Split(filepath.ToSlash(rel), "/"))
				if depth > limits.maxDepth {
					if !depthTruncated {
						depthTruncated = true
						warnings = append(
							warnings,
							fmt.Sprintf(
								"Some items were omitted because they exceed the maximum archive depth (%d).",
								limits.maxDepth,
							),
						)
					}
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				entryInfo, entryErr := d.Info()
				if entryErr != nil {
					warnings = append(warnings, fmt.Sprintf("skipped %s: %v", path, entryErr))
					return nil
				}
				if entryInfo.Mode()&os.ModeSymlink != 0 {
					warnings = append(warnings, fmt.Sprintf("skipped symlink %s", path))
					return nil
				}
				if !d.IsDir() && !entryInfo.Mode().IsRegular() {
					warnings = append(warnings, fmt.Sprintf("skipped %s: not a regular file", path))
					return nil
				}

				zipRel := filepath.ToSlash(rel)
				zipName := dedupeZipEntryName(rootZipPrefix+"/"+zipRel, usedNames)

				if d.IsDir() {
					dirNames = append(dirNames, zipName+"/")
					return nil
				}

				size := entryInfo.Size()
				if size < 0 {
					return fmt.Errorf("invalid file size for %s", path)
				}
				totalBytes += size
				if totalBytes > limits.maxSizeBytes {
					return fmt.Errorf(
						"archive exceeds maximum size (%d bytes)",
						limits.maxSizeBytes,
					)
				}
				fileCount++
				if fileCount > limits.maxFiles {
					return fmt.Errorf("archive exceeds maximum file count (%d)", limits.maxFiles)
				}

				files = append(files, archiveFileEntry{
					absPath: path,
					zipName: zipName,
					size:    size,
				})
				return nil
			})
			if walkErr != nil {
				return nil, nil, warnings, totalBytes, walkErr
			}
			continue
		}

		size := info.Size()
		if size < 0 {
			return nil, nil, warnings, 0, fmt.Errorf("invalid file size for %s", root)
		}
		totalBytes += size
		if totalBytes > limits.maxSizeBytes {
			return nil, nil, warnings, totalBytes, fmt.Errorf(
				"archive exceeds maximum size (%d bytes)",
				limits.maxSizeBytes,
			)
		}
		fileCount++
		if fileCount > limits.maxFiles {
			return nil, nil, warnings, totalBytes, fmt.Errorf(
				"archive exceeds maximum file count (%d)",
				limits.maxFiles,
			)
		}

		zipName := dedupeZipEntryName(filepath.ToSlash(rootBase), usedNames)
		files = append(files, archiveFileEntry{
			absPath: root,
			zipName: zipName,
			size:    size,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].zipName < files[j].zipName
	})
	sort.Strings(dirNames)

	return files, dirNames, warnings, totalBytes, nil
}

func appendArchiveWarning(warnings []string, msg string) []string {
	if len(warnings) == maxArchiveWarnings {
		return append(warnings, "additional items were skipped")
	}
	if len(warnings) > maxArchiveWarnings {
		return warnings
	}
	return append(warnings, msg)
}

func writeArchiveZip(
	ctx context.Context,
	tempPath string,
	roots []string,
	limits archiveLimits,
) (warnings []string, err error) {
	ctx = archiveContext(ctx)
	if err := checkArchiveCanceled(ctx); err != nil {
		return nil, err
	}
	files, dirNames, warnings, totalBytes, err := collectArchiveEntries(ctx, roots, limits)
	if err != nil {
		return warnings, err
	}
	if err := checkArchiveCanceled(ctx); err != nil {
		return warnings, err
	}
	if len(files) == 0 && len(dirNames) == 0 {
		return warnings, fmt.Errorf("nothing to archive")
	}

	if err := ensureArchiveDiskSpace(filepath.Dir(tempPath), totalBytes); err != nil {
		return warnings, err
	}

	out, err := createExclusiveReplacePrivateFile(tempPath)
	if err != nil {
		return warnings, fmt.Errorf("failed to create archive file: %w", err)
	}

	success := false
	defer func() {
		_ = out.Close()
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	zw := zip.NewWriter(out)
	usedDir := make(map[string]struct{})

	for _, dirName := range dirNames {
		if _, ok := usedDir[dirName]; ok {
			continue
		}
		usedDir[dirName] = struct{}{}
		header := &zip.FileHeader{
			Name:   dirName,
			Method: zip.Store,
		}
		header.SetMode(0o755 | os.ModeDir)
		if _, err := zw.CreateHeader(header); err != nil {
			return warnings, fmt.Errorf("failed to add directory %s: %w", dirName, err)
		}
	}

	addedFiles := 0
	for _, entry := range files {
		if err := checkArchiveCanceled(ctx); err != nil {
			return warnings, err
		}
		info, err := entryInfoFromPath(entry.absPath, entry.size)
		if err != nil {
			warnings = appendArchiveWarning(
				warnings, fmt.Sprintf("skipped %s: %v", entry.absPath, err),
			)
			continue
		}

		src, err := openExistingFileSharedRead(entry.absPath)
		if err != nil {
			warnings = appendArchiveWarning(
				warnings, fmt.Sprintf("skipped %s: %v", entry.absPath, err),
			)
			continue
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			_ = src.Close()
			return warnings, fmt.Errorf("failed to create zip header for %s: %w", entry.absPath, err)
		}
		header.Name = entry.zipName
		header.Method = zip.Deflate
		header.UncompressedSize64 = uint64(entry.size)

		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = src.Close()
			return warnings, fmt.Errorf("failed to add %s: %w", entry.zipName, err)
		}

		_, copyErr := io.Copy(writer, cancelAwareReader{ctx: ctx, r: src})
		_ = src.Close()
		if copyErr != nil {
			if archiveCanceledErr(copyErr) {
				return warnings, errArchiveCanceled
			}
			return warnings, fmt.Errorf("failed to write %s into archive: %w", entry.zipName, copyErr)
		}
		addedFiles++
	}

	if addedFiles == 0 && len(dirNames) == 0 {
		return warnings, fmt.Errorf("nothing to archive")
	}

	if err := zw.Close(); err != nil {
		return warnings, fmt.Errorf("failed to finalize archive: %w", err)
	}
	if err := out.Close(); err != nil {
		return warnings, fmt.Errorf("failed to close archive file: %w", err)
	}

	info, err := os.Stat(tempPath)
	if err != nil {
		return warnings, fmt.Errorf("failed to stat archive file: %w", err)
	}
	if info.Size() == 0 {
		return warnings, fmt.Errorf("archive file is empty")
	}

	success = true
	return warnings, nil
}

func entryInfoFromPath(path string, size int64) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if size >= 0 && !info.IsDir() {
		return &archiveFileInfo{FileInfo: info, size: size}, nil
	}
	return info, nil
}

type archiveFileInfo struct {
	os.FileInfo
	size int64
}

func (a *archiveFileInfo) Size() int64 {
	return a.size
}

func (a *Agent) PrepareFilesDownloadArchive(p *NatsMsg) (map[string]interface{}, error) {
	sessionID, err := parsePayloadString(p.Data, "session_id")
	if err != nil {
		return nil, err
	}

	paths, err := parsePayloadPathsJSON(p.Data)
	if err != nil {
		return nil, err
	}

	chunkSize, err := parsePayloadInt64(p.Data, "chunk_size")
	if err != nil {
		return nil, err
	}
	chunkSize, err = clampChunkSize(chunkSize)
	if err != nil {
		return nil, err
	}

	limits := parseArchiveLimits(p.Data)
	ctx := registerArchiveBuild(sessionID)
	go a.buildAndServeArchive(ctx, sessionID, paths, chunkSize, limits)

	return map[string]interface{}{"status": "building"}, nil
}

// buildAndServeArchive func builds the temp zip, reports readiness to the server and then
// streams it through the normal download chunk relay.
func (a *Agent) buildAndServeArchive(
	ctx context.Context, sessionID string, paths []string, chunkSize int64, limits archiveLimits,
) {
	defer a.recoverFilesRPC("files_download_archive build", nil)
	defer unregisterArchiveBuild(sessionID, ctx)
	ctx = archiveContext(ctx)

	// Builds are disk- and CPU-heavy; run a couple at a time and queue the
	// rest (still cancellable while they wait).
	select {
	case archiveBuildSlots <- struct{}{}:
		defer func() { <-archiveBuildSlots }()
	case <-ctx.Done():
	}

	if err := checkArchiveCanceled(ctx); err != nil {
		if a.Logger != nil {
			a.Logger.Debugf("file_transfer archive cancelled session=%s", sessionID)
		}
		return
	}

	tempPath, err := a.archiveTempPath(sessionID)
	if err != nil {
		a.Logger.Errorln("files_download_archive temp dir:", err)
		a.reportArchiveError(sessionID, err.Error())
		return
	}
	a.DownloadTransferSessionsMu.Lock()
	if existing, ok := a.DownloadTransferSessions[sessionID]; ok && existing != nil {
		if existing.StopStream != nil {
			close(existing.StopStream)
		}
		if existing.File != nil {
			_ = existing.File.Close()
		}
		if existing.RemoveOnClose && existing.SourcePath != "" {
			_ = os.Remove(existing.SourcePath)
		}
		delete(a.DownloadTransferSessions, sessionID)
	}
	a.DownloadTransferSessionsMu.Unlock()

	_ = os.Remove(tempPath)

	warnings, err := writeArchiveZip(ctx, tempPath, paths, limits)
	if err != nil {
		_ = os.Remove(tempPath)
		if archiveCanceledErr(err) {
			if a.Logger != nil {
				a.Logger.Debugf("file_transfer archive cancelled session=%s", sessionID)
			}
			return
		}
		a.Logger.Debugln("files_download_archive build:", err)
		a.reportArchiveError(sessionID, err.Error())
		return
	}

	if err := checkArchiveCanceled(ctx); err != nil {
		_ = os.Remove(tempPath)
		if a.Logger != nil {
			a.Logger.Debugf("file_transfer archive cancelled session=%s", sessionID)
		}
		return
	}

	info, err := os.Stat(tempPath)
	if err != nil || info.Size() <= 0 {
		_ = os.Remove(tempPath)
		a.reportArchiveError(sessionID, "archive file is empty")
		return
	}
	totalSize := info.Size()

	if !a.reportArchiveReady(ctx, sessionID, tempPath, totalSize, warnings) {
		_ = os.Remove(tempPath)
		if checkArchiveCanceled(ctx) != nil {
			if a.Logger != nil {
				a.Logger.Debugf("file_transfer archive cancelled session=%s", sessionID)
			}
			return
		}
		a.reportArchiveError(sessionID, "failed to report archive ready after retries")
		return
	}

	if err := checkArchiveCanceled(ctx); err != nil {
		_ = os.Remove(tempPath)
		if a.Logger != nil {
			a.Logger.Debugf("file_transfer archive cancelled session=%s", sessionID)
		}
		return
	}

	file, err := openExistingFileSharedRead(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		a.reportArchiveError(sessionID, fmt.Sprintf("failed to open archive: %v", err))
		return
	}

	stopStream := make(chan struct{})
	hasher := sha256.New()

	a.DownloadTransferSessionsMu.Lock()
	a.DownloadTransferSessions[sessionID] = &DownloadTransferSession{
		SessionID:     sessionID,
		SourcePath:    tempPath,
		TotalSize:     totalSize,
		ChunkSize:     chunkSize,
		File:          file,
		StopStream:    stopStream,
		LastActivity:  time.Now(),
		Hasher:        hasher,
		HashedOffset:  0,
		RemoveOnClose: true,
		AckCh:         make(chan struct{}, 1),
	}
	a.DownloadTransferSessionsMu.Unlock()

	go a.streamDownloadChunks(sessionID, stopStream, 0)
}

func (a *Agent) reportArchiveReady(
	ctx context.Context, sessionID, archivePath string, totalSize int64, warnings []string,
) bool {
	url := fmt.Sprintf("/api/v3/file-transfers/%s/archive-ready/", sessionID)
	payload := map[string]interface{}{
		"total_size":   totalSize,
		"archive_path": archivePath,
	}
	if len(warnings) > 0 {
		payload["warnings"] = warnings
	}

	const maxAttempts = 4
	backoff := 2 * time.Second
	for attempt := 1; ; attempt++ {
		if checkArchiveCanceled(ctx) != nil {
			return false
		}
		reqCtx, cancel := context.WithTimeout(archiveContext(ctx), fileTransferFailTimeout)
		resp, err := a.fileTransferHTTP().R().SetContext(reqCtx).SetBody(payload).Post(url)
		cancel()
		if checkArchiveCanceled(ctx) != nil {
			return false
		}
		if err == nil && resp.StatusCode() == 200 {
			return true
		}
		if err == nil && resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
			a.Logger.Debugf(
				"file_transfer archive-ready callback session=%s status=%d body=%s (not retrying)",
				sessionID, resp.StatusCode(), string(resp.Body()),
			)
			return false
		}
		if attempt >= maxAttempts {
			if err != nil {
				a.Logger.Errorf(
					"file_transfer archive-ready callback session=%s failed after %d attempts err=%v",
					sessionID, attempt, err,
				)
			} else {
				a.Logger.Errorf(
					"file_transfer archive-ready callback session=%s failed after %d attempts status=%d",
					sessionID, attempt, resp.StatusCode(),
				)
			}
			return false
		}
		timer := time.NewTimer(backoff)
		select {
		case <-archiveContext(ctx).Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
		backoff *= 2
	}
}

func (a *Agent) reportArchiveError(sessionID, message string) {
	url := fmt.Sprintf("/api/v3/file-transfers/%s/archive-ready/", sessionID)
	payload := map[string]interface{}{"error": message}
	ctx, cancel := context.WithTimeout(context.Background(), fileTransferFailTimeout)
	defer cancel()
	if _, err := a.fileTransferHTTP().R().SetContext(ctx).SetBody(payload).Post(url); err != nil {
		a.Logger.Errorf(
			"file_transfer archive-error callback session=%s err=%v", sessionID, err,
		)
	}
}

func sweepArchiveTempDir(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isArchiveTempName(name) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err == nil {
			removed++
		}
	}
	return removed
}

func (a *Agent) SweepOrphanedArchives() {
	a.recoverInterruptedReplaces()
	removed := sweepArchiveTempDir(a.fileTransferTempDir())
	osTmp := os.TempDir()
	if !sameFileTransferDir(osTmp, a.fileTransferTempDir()) {
		removed += sweepArchiveTempDir(osTmp)
	}
	if removed > 0 && a.Logger != nil {
		a.Logger.Debugf("file_transfer startup: removed %d orphaned archive temp file(s)", removed)
	}
	a.sweepOrphanedUploadPartials(time.Now())
	a.sweepOrphanedExtractPartials()
}
