/*
Copyright 2026 Amidaware Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/shirou/gopsutil/v3/disk"
)

const (
	downloadPutTimeout              = 120 * time.Second
	downloadReadyGetTimeout         = 15 * time.Second
	uploadChunkGetTimeout           = 120 * time.Second
	uploadChunkAckTimeout           = 60 * time.Second
	fileTransferFailTimeout         = 30 * time.Second
	downloadPushMaxAttempts         = 12
	downloadPushRetryMinBackoff     = 500 * time.Millisecond
	downloadPushRetryMaxBackoff     = 8 * time.Second
	downloadPushAckWaitLogEvery     = 5 * time.Second
	fileTransferSessionIdleTimeout  = 10 * time.Minute
	fileTransferReaperInterval      = 2 * time.Minute
	fileTransferPartialRetention    = 1 * time.Hour
	fileTransferDrainMinBackoff     = 50 * time.Millisecond
	fileTransferDrainMaxBackoff     = 500 * time.Millisecond
	fileTransferDrainIdleTimeout    = 15 * time.Second
	fileTransferChunkSizeMin        = 1 * 1024 * 1024
	fileTransferChunkSizeMax        = 16 * 1024 * 1024
	fileTransferIdleConnTimeout     = 3 * time.Minute
	fileTransferTLSHandshakeTimeout = 15 * time.Second
	fileTransferMaxIdleConns        = 32
	fileTransferMaxIdleConnsPerHost = 8
	fileTransferDiskSpaceMargin     = int64(64 * 1024 * 1024)
)

// Variables rather than constants so tests can shorten them.
var (
	downloadPushAckFallbackPoll = 5 * time.Second
	downloadPushAckWaitMax      = 120 * time.Second
	uploadDrainRetryWindow      = 10 * time.Minute
)

var downloadChunkExpectedOffsetRe = regexp.MustCompile(
	`does not match expected (\d+)`,
)

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampInt64(v, min, max int64) int64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

var diskUsage = func(path string) (*disk.UsageStat, error) {
	return disk.Usage(path)
}

func uploadRemainingBytes(totalSize, committedOffset int64) int64 {
	if committedOffset < 0 {
		committedOffset = 0
	}
	if committedOffset >= totalSize {
		return 0
	}
	return totalSize - committedOffset
}

func ensureDiskSpace(dir string, needed int64, action string) error {
	if needed <= 0 {
		return nil
	}
	usage, err := diskUsage(dir)
	if err != nil || usage == nil {
		return nil
	}
	required := needed + fileTransferDiskSpaceMargin
	if int64(usage.Free) < required {
		if action == "" {
			action = "for this transfer"
		}
		return fmt.Errorf(
			"insufficient disk space %s: need ~%d bytes, %d free in %s",
			action, required, usage.Free, dir,
		)
	}
	return nil
}

func isNoSpaceError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ENOSPC) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		err = pathErr.Err
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno == syscall.ENOSPC {
			return true
		}
		// Windows ERROR_DISK_FULL
		if runtime.GOOS == "windows" && errno == 0x70 {
			return true
		}
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "no space left") || strings.Contains(lower, "disk full")
}

func clampAtMostInt(v, max int) int {
	if v > max {
		return max
	}
	return v
}

func clampAtMostInt64(v, max int64) int64 {
	if v > max {
		return max
	}
	return v
}

func clampChunkSize(n int64) (int64, error) {
	if n <= 0 {
		return 0, fmt.Errorf("chunk_size must be greater than 0")
	}
	return clampInt64(n, fileTransferChunkSizeMin, fileTransferChunkSizeMax), nil
}

func newFileTransferResty(
	baseURL string,
	headers map[string]string,
	debug bool,
	insecure bool,
	proxy string,
	rootCert string,
) *resty.Client {
	c := resty.New()
	c.SetBaseURL(baseURL)
	if len(headers) > 0 {
		c.SetHeaders(headers)
	}
	c.SetTimeout(0)
	c.SetDebug(debug)
	if insecure {
		c.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}
	if proxy != "" {
		c.SetProxy(proxy)
	}
	if rootCert != "" {
		c.SetRootCertificate(rootCert)
	}
	enableFileTransferKeepAlive(c)
	return c
}

func enableFileTransferKeepAlive(c *resty.Client) {
	if c == nil {
		return
	}
	httpClient := c.GetClient()
	if httpClient == nil {
		return
	}
	tr, ok := httpClient.Transport.(*http.Transport)
	if !ok || tr == nil {
		tr = http.DefaultTransport.(*http.Transport).Clone()
		httpClient.Transport = tr
	}
	tr.DisableKeepAlives = false
	tr.MaxIdleConns = fileTransferMaxIdleConns
	tr.MaxIdleConnsPerHost = fileTransferMaxIdleConnsPerHost
	tr.IdleConnTimeout = fileTransferIdleConnTimeout
	tr.TLSHandshakeTimeout = fileTransferTLSHandshakeTimeout
}

func (a *Agent) fileTransferHTTP() *resty.Client {
	if a.fileTransferClient != nil {
		return a.fileTransferClient
	}
	return a.rClient
}

type UploadTransferSession struct {
	SessionID       string
	DestinationPath string
	PartialPath     string
	Filename        string
	TotalSize       int64
	ChunkSize       int64
	CommittedOffset int64
	ConflictPolicy  string
	File            *os.File
	LastActivity    time.Time
	Hasher          hash.Hash
	HashedOffset    int64
	hashLive        bool
	hashJob         *resumeHashJob
	DormantSince    time.Time
	Draining        bool
}

const (
	uploadConflictPolicyReplace = "replace"
	uploadConflictPolicySkip    = "skip"
	uploadConflictPolicySkipMsg = "destination already exists (conflict_policy=skip)"
)

func parseUploadConflictPolicy(data map[string]string) string {
	val := strings.ToLower(strings.TrimSpace(data["conflict_policy"]))
	if val == uploadConflictPolicySkip {
		return uploadConflictPolicySkip
	}
	return uploadConflictPolicyReplace
}

func uploadDestinationExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func enforceUploadConflictPolicySkip(destinationPath, conflictPolicy string) error {
	if conflictPolicy != uploadConflictPolicySkip {
		return nil
	}
	exists, err := uploadDestinationExists(destinationPath)
	if err != nil {
		return fmt.Errorf("failed to check destination file: %w", err)
	}
	if exists {
		return fmt.Errorf("%s", uploadConflictPolicySkipMsg)
	}
	return nil
}

func parsePayloadString(data map[string]string, key string) (string, error) {
	val, ok := data[key]
	if !ok {
		return "", fmt.Errorf("missing %s", key)
	}
	val = strings.TrimSpace(val)
	if val == "" {
		return "", fmt.Errorf("missing %s", key)
	}
	return val, nil
}

func parsePayloadInt64(data map[string]string, key string) (int64, error) {
	val, ok := data[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, fmt.Errorf("missing %s", key)
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n, nil
}

func parsePayloadResume(data map[string]string) (bool, int64) {
	resume := strings.EqualFold(strings.TrimSpace(data["resume"]), "true")
	offset, err := strconv.ParseInt(strings.TrimSpace(data["committed_offset"]), 10, 64)
	if err != nil || offset < 0 {
		offset = 0
	}
	return resume, offset
}

func requireRegularFileInfo(info os.FileInfo) error {
	if info == nil {
		return fmt.Errorf("path is not a regular file")
	}
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		return fmt.Errorf("path is a symlink")
	}
	if !mode.IsRegular() {
		return fmt.Errorf("path is not a regular file")
	}
	return nil
}

func verifyOpenedRegularFile(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	return requireRegularFileInfo(info)
}

func createExclusiveRegularFile(path string, perm os.FileMode) (*os.File, error) {
	f, err := openFileNoFollow(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return nil, err
	}
	if err := verifyOpenedRegularFile(f); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return f, nil
}

func createExclusiveReplaceRegularFile(path string, perm os.FileMode) (*os.File, error) {
	info, err := os.Lstat(path)
	if err == nil {
		if err := requireRegularFileInfo(info); err != nil {
			return nil, fmt.Errorf("refusing to use %s: %w", path, err)
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return createExclusiveRegularFile(path, perm)
}

func openExistingRegularFile(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := requireRegularFileInfo(info); err != nil {
		return nil, err
	}
	f, err := openFileNoFollow(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	if err := verifyOpenedRegularFile(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func prepareUploadPartialFile(
	partialPath string, resume bool, resumeOffset, totalSize int64,
) (*os.File, int64, error) {
	if resume {
		if resumeOffset < 0 || resumeOffset > totalSize {
			return nil, 0, fmt.Errorf("invalid resume offset")
		}
		info, err := os.Lstat(partialPath)
		if err != nil {
			return nil, 0, fmt.Errorf("cannot resume upload, partial file missing: %w", err)
		}
		if err := requireRegularFileInfo(info); err != nil {
			return nil, 0, fmt.Errorf("cannot resume upload, refusing partial path: %w", err)
		}
		if info.Size() < resumeOffset {
			return nil, 0, fmt.Errorf("partial file is shorter than resume offset")
		}

		file, err := openExistingRegularFile(partialPath)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to open partial file: %w", err)
		}
		if err := file.Truncate(resumeOffset); err != nil {
			_ = file.Close()
			return nil, 0, fmt.Errorf("failed to truncate partial file: %w", err)
		}
		return file, resumeOffset, nil
	}

	file, err := createExclusiveReplaceRegularFile(partialPath, 0o644)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create partial file: %w", err)
	}
	return file, 0, nil
}

func replaceUploadPartialWithDestination(partialPath, destinationPath, journalDir string) error {
	if err := rejectUploadDestinationIfDirectory(destinationPath); err != nil {
		return err
	}

	_, err := os.Lstat(destinationPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check destination file: %w", err)
	}

	if os.IsNotExist(err) {
		if err := os.Rename(partialPath, destinationPath); err != nil {
			return fmt.Errorf("failed to rename partial file: %w", err)
		}
		invalidateFileBrowserListingsFor(destinationPath)
		return nil
	}

	clearPathReadOnlyIfNeeded(destinationPath)
	backupPath, err := nextReplaceBackupPath(destinationPath)
	if err != nil {
		return err
	}
	// Journal the two renames so a crash between them is repaired at the
	// next start (see recoverInterruptedReplaces).
	journal := writeReplaceJournal(journalDir, destinationPath, backupPath)
	if err := os.Rename(destinationPath, backupPath); err != nil {
		removeReplaceJournal(journal)
		return fmt.Errorf("failed to move existing destination file: %w", err)
	}
	if err := os.Rename(partialPath, destinationPath); err != nil {
		if restoreErr := os.Rename(backupPath, destinationPath); restoreErr != nil {
			return fmt.Errorf(
				"failed to rename partial file: %v; original kept at %s: %v",
				err, backupPath, restoreErr,
			)
		}
		removeReplaceJournal(journal)
		return fmt.Errorf("failed to rename partial file: %w", err)
	}
	_ = os.Remove(backupPath)
	removeReplaceJournal(journal)
	invalidateFileBrowserListingsFor(destinationPath)
	return nil
}

const replaceJournalPrefix = "trmm-replace-"

type replaceJournal struct {
	Destination string `json:"destination"`
	Backup      string `json:"backup"`
}

// writeReplaceJournal records an in-progress replace; it returns "" (and the
// replace proceeds unjournaled) when the temp dir is not writable.
func writeReplaceJournal(dir, destination, backup string) string {
	if strings.TrimSpace(dir) == "" {
		return ""
	}
	data, err := json.Marshal(replaceJournal{Destination: destination, Backup: backup})
	if err != nil {
		return ""
	}
	f, err := os.CreateTemp(dir, replaceJournalPrefix+"*.json")
	if err != nil {
		return ""
	}
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil || serr != nil || cerr != nil {
		_ = os.Remove(f.Name())
		return ""
	}
	return f.Name()
}

func removeReplaceJournal(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

// recoverInterruptedReplaces finishes replaces cut short by a crash: if the
// destination is gone the original is moved back from its backup, otherwise
// the leftover backup is removed.
func (a *Agent) recoverInterruptedReplaces() {
	dir := a.fileTransferStateDir()
	matches, _ := filepath.Glob(filepath.Join(dir, replaceJournalPrefix+"*.json"))
	for _, journalPath := range matches {
		data, err := os.ReadFile(journalPath)
		var j replaceJournal
		if err != nil || json.Unmarshal(data, &j) != nil ||
			j.Destination == "" || !strings.Contains(filepath.Base(j.Backup), ".trmm-replace") {
			_ = os.Remove(journalPath)
			continue
		}
		if _, err := os.Lstat(j.Backup); err == nil {
			if _, derr := os.Lstat(j.Destination); os.IsNotExist(derr) {
				err = os.Rename(j.Backup, j.Destination)
			} else {
				err = os.Remove(j.Backup)
			}
			if err != nil && a.Logger != nil {
				a.Logger.Errorf("file_transfer startup: replace recovery for %s: %v", j.Destination, err)
				continue
			}
			if a.Logger != nil {
				a.Logger.Debugf("file_transfer startup: recovered interrupted replace of %s", j.Destination)
			}
		}
		_ = os.Remove(journalPath)
	}
}

const uploadPartialJournalPrefix = "trmm-upload-"

// uploadPartialJournal lets us find a .partial after an agent restart.
type uploadPartialJournal struct {
	SessionID string `json:"session_id"`
	Partial   string `json:"partial"`
}

func (a *Agent) uploadPartialJournalPath(sessionID string) string {
	dir := a.fileTransferStateDir()
	if strings.TrimSpace(dir) == "" || sessionID == "" || filepath.Base(sessionID) != sessionID {
		return ""
	}
	return filepath.Join(dir, uploadPartialJournalPrefix+sessionID+".json")
}

func (a *Agent) writeUploadPartialJournal(sessionID, partial string) {
	path := a.uploadPartialJournalPath(sessionID)
	if path == "" {
		return
	}
	data, err := json.Marshal(uploadPartialJournal{SessionID: sessionID, Partial: partial})
	if err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil && a.Logger != nil {
		a.Logger.Debugf("file_transfer upload session=%s: partial journal not written: %v", sessionID, err)
	}
}

func (a *Agent) removeUploadPartialJournal(sessionID string) {
	if path := a.uploadPartialJournalPath(sessionID); path != "" {
		_ = os.Remove(path)
	}
}

func readUploadPartialJournal(path string) (uploadPartialJournal, bool) {
	var j uploadPartialJournal
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &j) != nil ||
		j.SessionID == "" || !strings.HasSuffix(j.Partial, ".partial") {
		return j, false
	}
	return j, true
}

func (a *Agent) uploadPartialInUse(sessionID, partial string) bool {
	a.FileTransferSessionsMu.Lock()
	defer a.FileTransferSessionsMu.Unlock()
	for id, s := range a.FileTransferSessions {
		if s != nil && (id == sessionID || s.PartialPath == partial) {
			return true
		}
	}
	return false
}

func (a *Agent) removeJournaledUploadPartial(sessionID string) string {
	path := a.uploadPartialJournalPath(sessionID)
	if path == "" {
		return ""
	}
	j, ok := readUploadPartialJournal(path)
	_ = os.Remove(path)
	if !ok || j.SessionID != sessionID || a.uploadPartialInUse("", j.Partial) {
		return ""
	}
	if info, err := os.Lstat(j.Partial); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	if err := os.Remove(j.Partial); err != nil {
		return ""
	}
	return j.Partial
}

// sweepOrphanedUploadPartials removes partials left by a restart once they're
// past the retention window.
func (a *Agent) sweepOrphanedUploadPartials(now time.Time) int {
	dir := a.fileTransferStateDir()
	if strings.TrimSpace(dir) == "" {
		return 0
	}
	matches, _ := filepath.Glob(filepath.Join(dir, uploadPartialJournalPrefix+"*.json"))
	removed := 0
	for _, journalPath := range matches {
		j, ok := readUploadPartialJournal(journalPath)
		if !ok {
			_ = os.Remove(journalPath)
			continue
		}
		if a.uploadPartialInUse(j.SessionID, j.Partial) {
			continue
		}
		info, err := os.Lstat(j.Partial)
		if os.IsNotExist(err) || (err == nil && !info.Mode().IsRegular()) {
			_ = os.Remove(journalPath)
			continue
		}
		if err != nil || now.Sub(info.ModTime()) < fileTransferPartialRetention {
			continue
		}
		if err := os.Remove(j.Partial); err != nil {
			if a.Logger != nil {
				a.Logger.Errorf("file_transfer: failed to remove orphaned partial %s: %v", j.Partial, err)
			}
			continue
		}
		_ = os.Remove(journalPath)
		removed++
		if a.Logger != nil {
			a.Logger.Debugf(
				"file_transfer: removed orphaned partial %s (session=%s, no activity for %s)",
				j.Partial, j.SessionID, fileTransferPartialRetention,
			)
		}
	}
	return removed
}

func rejectUploadDestinationIfDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to check destination file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("destination is a directory")
	}
	return nil
}

const maxFileNameBytes = 255

// siblingWithSuffix falls back to a hashed name when path+suffix would be too
// long for the filesystem.
func siblingWithSuffix(path, suffix string) string {
	base := filepath.Base(path)
	if len(base)+len(suffix) <= maxFileNameBytes {
		return path + suffix
	}
	sum := sha256.Sum256([]byte(base))
	return filepath.Join(filepath.Dir(path), "."+hex.EncodeToString(sum[:8])+suffix)
}

func uploadPartialPath(destinationPath string) string {
	return siblingWithSuffix(destinationPath, ".partial")
}

func nextReplaceBackupPath(dest string) (string, error) {
	for i := 0; i < 100; i++ {
		candidate := siblingWithSuffix(dest, ".trmm-replace")
		if i > 0 {
			candidate = siblingWithSuffix(dest, fmt.Sprintf(".trmm-replace-%d", i))
		}
		_, err := os.Lstat(candidate)
		if os.IsNotExist(err) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("failed to allocate replace backup path: %w", err)
		}
	}
	return "", fmt.Errorf("failed to allocate replace backup path")
}

func hasTrailingSpaceOrPeriod(name, goos string) bool {
	return goos == "windows" &&
		(strings.HasSuffix(name, " ") || strings.HasSuffix(name, "."))
}

func validateUploadFilename(filename string) error {
	if hasTrailingSpaceOrPeriod(filename, runtime.GOOS) {
		return fmt.Errorf("filename cannot end with a space or a period")
	}
	if filename == "." || filename == ".." {
		return fmt.Errorf("invalid filename")
	}
	if len(filename) > 255 {
		return fmt.Errorf("filename is too long")
	}
	if strings.ContainsAny(filename, "<>:\"/\\|?*\x00") {
		return fmt.Errorf("invalid filename")
	}
	return nil
}

func pathHasTraversalComponent(path string) bool {
	vol := filepath.VolumeName(path)
	rest := strings.TrimPrefix(path, vol)
	rest = strings.TrimPrefix(rest, string(filepath.Separator))
	rest = strings.TrimPrefix(rest, "/")

	if rest == "" {
		return false
	}

	for _, part := range strings.FieldsFunc(rest, func(r rune) bool {
		return r == filepath.Separator || r == '/'
	}) {
		if part == ".." {
			return true
		}
	}
	return false
}

func validateUploadDestinationPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("missing destination_path")
	}

	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("destination_path must be an absolute path")
	}

	if pathHasTraversalComponent(path) {
		return "", fmt.Errorf("invalid destination_path")
	}

	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("destination_path must be an absolute path")
	}

	if pathHasTraversalComponent(cleaned) {
		return "", fmt.Errorf("invalid destination_path")
	}

	return cleaned, nil
}

func (a *Agent) HandleUploadChunkAvailable(p *NatsMsg) (map[string]interface{}, error) {
	sessionID, err := parsePayloadString(p.Data, "session_id")
	if err != nil {
		return nil, err
	}

	a.FileTransferSessionsMu.Lock()
	session, ok := a.FileTransferSessions[sessionID]
	if !ok || session == nil {
		a.FileTransferSessionsMu.Unlock()
		return nil, fmt.Errorf("upload session not found")
	}
	if session.Draining {
		a.FileTransferSessionsMu.Unlock()
		return map[string]interface{}{"status": "draining"}, nil
	}
	session.Draining = true
	a.FileTransferSessionsMu.Unlock()

	// only clear our own session's flag, a resume may have replaced it
	defer func() {
		a.FileTransferSessionsMu.Lock()
		session.Draining = false
		a.FileTransferSessionsMu.Unlock()
	}()

	committedOffset, err := a.drainUploadChunks(sessionID, session)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":           "acked",
		"committed_offset": committedOffset,
	}, nil
}

func (a *Agent) drainUploadChunks(sessionID string, owner *UploadTransferSession) (int64, error) {
	url := fmt.Sprintf("/api/v3/file-transfers/%s/next-chunk/", sessionID)

	readState := func() (int64, int64, bool) {
		a.FileTransferSessionsMu.Lock()
		defer a.FileTransferSessionsMu.Unlock()
		s, ok := a.FileTransferSessions[sessionID]
		if !ok || s == nil || s != owner {
			return 0, 0, false
		}
		return s.CommittedOffset, s.TotalSize, true
	}

	committedOffset, totalSize, ok := readState()
	if !ok {
		return 0, fmt.Errorf("upload session not found")
	}

	backoff := fileTransferDrainMinBackoff
	idleDeadline := time.Now().Add(fileTransferDrainIdleTimeout)
	retryBackoff := downloadPushRetryMinBackoff
	var retryStarted time.Time

	// Don't give up on a network error, the server won't notify us again while
	// the browser's pipeline is full.
	retry := func(err error) bool {
		if !isRetryableUploadDrainError(err) {
			return false
		}
		if retryStarted.IsZero() {
			retryStarted = time.Now()
		}
		if time.Since(retryStarted) > uploadDrainRetryWindow {
			return false
		}
		a.Logger.Debugf(
			"file_transfer upload drain session=%s committed_offset=%d retry after %v: %v",
			sessionID, committedOffset, retryBackoff, err,
		)
		time.Sleep(retryBackoff)
		retryBackoff *= 2
		if retryBackoff > downloadPushRetryMaxBackoff {
			retryBackoff = downloadPushRetryMaxBackoff
		}
		// restart the idle window after an outage
		idleDeadline = time.Now().Add(fileTransferDrainIdleTimeout)
		return true
	}

	for committedOffset < totalSize {
		fetchStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), uploadChunkGetTimeout)
		resp, err := a.fileTransferHTTP().R().
			SetDebug(false).
			SetContext(ctx).
			Get(url)
		cancel()
		if err != nil {
			err = fmt.Errorf("failed to pull upload chunk: %w", err)
			if _, _, ok := readState(); ok && retry(err) {
				continue
			}
			return committedOffset, err
		}

		switch resp.StatusCode() {
		case 200:
			if err := a.applyUploadChunk(sessionID, owner, resp); err != nil {
				if errors.Is(err, errUploadSessionReplaced) {
					return committedOffset, nil
				}
				if isUploadWriteError(err) {
					a.reportFileTransferFailure(sessionID, err.Error())
					return committedOffset, err
				}
				if _, _, ok := readState(); ok && retry(err) {
					continue
				}
				return committedOffset, err
			}
			retryStarted = time.Time{}
			retryBackoff = downloadPushRetryMinBackoff
			committedOffset, totalSize, ok = readState()
			if !ok {
				return committedOffset, fmt.Errorf("upload session not found")
			}
			a.Logger.Debugf(
				"file_transfer chunk session=%s chunk_fetch_ms=%d committed_offset=%d",
				sessionID,
				time.Since(fetchStart).Milliseconds(),
				committedOffset,
			)
			backoff = fileTransferDrainMinBackoff
			idleDeadline = time.Now().Add(fileTransferDrainIdleTimeout)
		case 404:
			if time.Now().After(idleDeadline) {
				a.Logger.Debugf(
					"file_transfer chunk session=%s: drain idle committed_offset=%d/%d",
					sessionID, committedOffset, totalSize,
				)
				return committedOffset, nil
			}
			time.Sleep(backoff)
			backoff *= 2
			if backoff > fileTransferDrainMaxBackoff {
				backoff = fileTransferDrainMaxBackoff
			}
		default:
			err := fmt.Errorf(
				"pull upload chunk failed: %d %s",
				resp.StatusCode(),
				string(resp.Body()),
			)
			if _, _, ok := readState(); ok && retry(err) {
				continue
			}
			return committedOffset, err
		}
	}

	return committedOffset, nil
}

var errUploadSessionReplaced = errors.New("upload session was re-prepared by a resume")

func isRetryableUploadDrainError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "failed to pull upload chunk:") ||
		strings.Contains(msg, "failed to ack upload chunk:") ||
		strings.Contains(msg, "chunk body size does not match Content-Range") {
		return true
	}
	for _, prefix := range []string{"pull upload chunk failed: ", "upload chunk ack failed: "} {
		i := strings.Index(msg, prefix)
		if i < 0 {
			continue
		}
		code := msg[i+len(prefix):]
		return strings.HasPrefix(code, "5") ||
			strings.HasPrefix(code, "408") ||
			strings.HasPrefix(code, "425") ||
			strings.HasPrefix(code, "429")
	}
	return false
}

func isUploadWriteError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "insufficient disk space to upload file") ||
		strings.Contains(msg, "failed to write chunk") ||
		strings.Contains(msg, "short write for upload chunk")
}

func (a *Agent) applyUploadChunk(sessionID string, owner *UploadTransferSession, resp *resty.Response) error {
	start, end, err := parseUploadChunkHeaders(resp)
	if err != nil {
		return err
	}

	data := resp.Body()
	expectedLen := end - start + 1
	if int64(len(data)) != expectedLen {
		return fmt.Errorf("chunk body size does not match Content-Range")
	}

	a.FileTransferSessionsMu.Lock()
	session, ok := a.FileTransferSessions[sessionID]
	if !ok || session == nil {
		a.FileTransferSessionsMu.Unlock()
		return fmt.Errorf("upload session not found")
	}
	if session != owner {
		a.FileTransferSessionsMu.Unlock()
		return errUploadSessionReplaced
	}
	if session.File == nil {
		a.FileTransferSessionsMu.Unlock()
		return fmt.Errorf("upload session file handle is missing")
	}
	session.LastActivity = time.Now()
	if end >= session.TotalSize {
		a.FileTransferSessionsMu.Unlock()
		return fmt.Errorf("chunk end exceeds total_size")
	}

	committedOffset := end + 1
	if committedOffset <= session.CommittedOffset {
		a.FileTransferSessionsMu.Unlock()
		ackStart := time.Now()
		if err := a.ackUploadChunk(sessionID, committedOffset); err != nil {
			return err
		}
		a.Logger.Debugf(
			"file_transfer chunk session=%s chunk_reack_ms=%d offset=%d",
			sessionID,
			time.Since(ackStart).Milliseconds(),
			committedOffset,
		)
		return nil
	}

	if start != session.CommittedOffset {
		a.FileTransferSessionsMu.Unlock()
		return fmt.Errorf("chunk start offset does not match committed_offset")
	}
	file := session.File
	a.FileTransferSessionsMu.Unlock()

	writeStart := time.Now()
	n, err := file.WriteAt(data, start)
	if err != nil {
		if isNoSpaceError(err) {
			return fmt.Errorf("insufficient disk space to upload file")
		}
		return fmt.Errorf("failed to write chunk: %w", err)
	}
	if int64(n) != expectedLen {
		return fmt.Errorf("short write for upload chunk")
	}
	writeMs := time.Since(writeStart).Milliseconds()

	a.FileTransferSessionsMu.Lock()
	session, ok = a.FileTransferSessions[sessionID]
	if !ok || session == nil {
		a.FileTransferSessionsMu.Unlock()
		return fmt.Errorf("upload session not found after write")
	}
	if session != owner {
		a.FileTransferSessionsMu.Unlock()
		return errUploadSessionReplaced
	}
	if start != session.CommittedOffset {
		a.FileTransferSessionsMu.Unlock()
		return fmt.Errorf("chunk start offset changed during write")
	}
	session.CommittedOffset = committedOffset
	if session.hashLive && session.Hasher != nil && start == session.HashedOffset {
		session.Hasher.Write(data)
		session.HashedOffset = committedOffset
	}
	a.FileTransferSessionsMu.Unlock()

	ackStart := time.Now()
	if err := a.ackUploadChunk(sessionID, committedOffset); err != nil {
		return err
	}
	ackMs := time.Since(ackStart).Milliseconds()

	a.Logger.Debugf(
		"file_transfer chunk session=%s chunk_write_ms=%d chunk_ack_ms=%d bytes=%d offset=%d",
		sessionID,
		writeMs,
		ackMs,
		expectedLen,
		committedOffset,
	)
	return nil
}

func (a *Agent) ackUploadChunk(sessionID string, committedOffset int64) error {
	url := fmt.Sprintf("/api/v3/file-transfers/%s/ack/", sessionID)
	payload := map[string]int64{"committed_offset": committedOffset}

	ctx, cancel := context.WithTimeout(context.Background(), uploadChunkAckTimeout)
	defer cancel()
	resp, err := a.fileTransferHTTP().R().
		SetContext(ctx).
		SetBody(payload).
		Post(url)
	if err != nil {
		return fmt.Errorf("failed to ack upload chunk: %w", err)
	}

	switch resp.StatusCode() {
	case 200:
		return nil
	case 401, 403, 410:
		return fmt.Errorf(
			"upload chunk ack auth/session error: %d %s",
			resp.StatusCode(),
			string(resp.Body()),
		)
	default:
		return fmt.Errorf(
			"upload chunk ack failed: %d %s",
			resp.StatusCode(),
			string(resp.Body()),
		)
	}
}

func parseUploadChunkHeaders(resp *resty.Response) (int64, int64, error) {
	startStr := strings.TrimSpace(resp.Header().Get("X-Chunk-Start"))
	endStr := strings.TrimSpace(resp.Header().Get("X-Chunk-End"))
	if startStr == "" || endStr == "" {
		return 0, 0, fmt.Errorf("missing chunk range headers")
	}

	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid X-Chunk-Start")
	}
	end, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid X-Chunk-End")
	}
	if start < 0 || end < start {
		return 0, 0, fmt.Errorf("invalid chunk byte range")
	}

	return start, end, nil
}

func (a *Agent) markUploadDormant(sessionID string) {
	now := time.Now()
	a.FileTransferSessionsMu.Lock()
	defer a.FileTransferSessionsMu.Unlock()
	session, ok := a.FileTransferSessions[sessionID]
	if !ok || session == nil {
		return
	}
	session.File = nil
	if session.DormantSince.IsZero() {
		session.DormantSince = now
	}
}

func (a *Agent) discardUploadSession(sessionID, partialPath string) {
	a.removeUploadPartialJournal(sessionID)
	if partialPath != "" {
		if err := os.Remove(partialPath); err != nil && !os.IsNotExist(err) {
			if a.Logger != nil {
				a.Logger.Errorf(
					"file_transfer upload session=%s: failed to remove partial %s: %v",
					sessionID, partialPath, err,
				)
			}
		}
	}
	a.FileTransferSessionsMu.Lock()
	var job *resumeHashJob
	if session, ok := a.FileTransferSessions[sessionID]; ok && session != nil {
		job = session.hashJob
		session.hashJob = nil
	}
	delete(a.FileTransferSessions, sessionID)
	a.FileTransferSessionsMu.Unlock()
	job.stop()
}

type uploadFinalizeCall struct {
	done       chan struct{}
	result     map[string]interface{}
	err        error
	finishedAt time.Time
}

var (
	uploadFinalizeMu    sync.Mutex
	uploadFinalizeCalls = make(map[string]*uploadFinalizeCall)
)

func (a *Agent) FinalizeFilesUpload(p *NatsMsg) (map[string]interface{}, error) {
	sessionID, err := parsePayloadString(p.Data, "session_id")
	if err != nil {
		return nil, err
	}

	uploadFinalizeMu.Lock()
	call, running := uploadFinalizeCalls[sessionID]
	if !running {
		call = &uploadFinalizeCall{done: make(chan struct{})}
		uploadFinalizeCalls[sessionID] = call
	}
	uploadFinalizeMu.Unlock()
	if running {
		<-call.done
		return call.result, call.err
	}

	call.result, call.err = a.finalizeFilesUpload(sessionID, p)
	uploadFinalizeMu.Lock()
	call.finishedAt = time.Now()
	if call.err != nil {
		// Only successes are remembered; a failed finalize may be retried.
		delete(uploadFinalizeCalls, sessionID)
	}
	uploadFinalizeMu.Unlock()
	close(call.done)
	return call.result, call.err
}

// pruneUploadFinalizeCalls forgets finished finalize results after the idle
// window; a retry that late would find the transfer long completed.
func pruneUploadFinalizeCalls(now time.Time) {
	uploadFinalizeMu.Lock()
	defer uploadFinalizeMu.Unlock()
	for id, call := range uploadFinalizeCalls {
		if !call.finishedAt.IsZero() && now.Sub(call.finishedAt) > fileTransferSessionIdleTimeout {
			delete(uploadFinalizeCalls, id)
		}
	}
}

func (a *Agent) finalizeFilesUpload(
	sessionID string, p *NatsMsg,
) (map[string]interface{}, error) {
	rawDestinationPath, err := parsePayloadString(p.Data, "destination_path")
	if err != nil {
		return nil, err
	}

	destinationPath, err := validateUploadDestinationPath(rawDestinationPath)
	if err != nil {
		return nil, err
	}

	totalSize, err := parsePayloadInt64(p.Data, "total_size")
	if err != nil {
		return nil, err
	}
	if totalSize <= 0 {
		return nil, fmt.Errorf("total_size must be greater than 0")
	}

	a.FileTransferSessionsMu.Lock()
	session, ok := a.FileTransferSessions[sessionID]
	if !ok || session == nil {
		a.FileTransferSessionsMu.Unlock()
		return nil, fmt.Errorf("upload session not found")
	}
	if destinationPath != session.DestinationPath {
		a.FileTransferSessionsMu.Unlock()
		return nil, fmt.Errorf("destination_path does not match session")
	}
	hashJob := session.hashJob
	a.FileTransferSessionsMu.Unlock()

	if err := waitResumeHashJob(hashJob); err != nil && a.Logger != nil {
		a.Logger.Debugf(
			"file_transfer upload finalize session=%s: resume prefix hash: %v",
			sessionID, err,
		)
	}

	a.FileTransferSessionsMu.Lock()
	session, ok = a.FileTransferSessions[sessionID]
	if !ok || session == nil {
		a.FileTransferSessionsMu.Unlock()
		return nil, fmt.Errorf("upload session not found")
	}

	partialPath := session.PartialPath
	if partialPath == "" {
		partialPath = uploadPartialPath(destinationPath)
	}
	file := session.File
	hasher := session.Hasher
	hashedOffset := session.HashedOffset
	conflictPolicy := session.ConflictPolicy
	if conflictPolicy == "" {
		conflictPolicy = parseUploadConflictPolicy(p.Data)
	}
	a.FileTransferSessionsMu.Unlock()

	if file != nil {
		if err := file.Sync(); err != nil {
			return nil, fmt.Errorf("failed to sync partial file: %w", err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("failed to close partial file: %w", err)
		}
	}

	a.markUploadDormant(sessionID)

	info, err := os.Stat(partialPath)
	if err != nil {
		if os.IsNotExist(err) {
			a.discardUploadSession(sessionID, "")
		}
		return nil, fmt.Errorf("failed to stat partial file: %w", err)
	}
	if info.Size() != totalSize {
		a.discardUploadSession(sessionID, partialPath)
		return nil, fmt.Errorf("partial file size does not match total_size")
	}

	expectedSHA := strings.ToLower(strings.TrimSpace(p.Data["sha256"]))
	computedSHA := ""
	if hasher != nil && hashedOffset == totalSize {
		computedSHA = hex.EncodeToString(hasher.Sum(nil))
	} else if hasher != nil && hashedOffset >= 0 && hashedOffset < totalSize {
		if err := hashFileRange(context.Background(), hasher, partialPath, hashedOffset, totalSize); err != nil {
			return nil, fmt.Errorf("failed to hash partial file: %w", err)
		}
		computedSHA = hex.EncodeToString(hasher.Sum(nil))
	} else if expectedSHA != "" {
		computedSHA, err = hashFileSHA256(partialPath)
		if err != nil {
			return nil, fmt.Errorf("failed to hash partial file: %w", err)
		}
	}
	if expectedSHA != "" && computedSHA != "" && expectedSHA != computedSHA {
		a.discardUploadSession(sessionID, partialPath)
		return nil, fmt.Errorf(
			"integrity check failed: expected sha256 %s but received %s",
			expectedSHA, computedSHA,
		)
	}

	if err := enforceUploadConflictPolicySkip(destinationPath, conflictPolicy); err != nil {
		a.discardUploadSession(sessionID, partialPath)
		return nil, err
	}

	if err := replaceUploadPartialWithDestination(
		partialPath, destinationPath, a.fileTransferStateDir(),
	); err != nil {
		if a.Logger != nil {
			a.Logger.Debugf(
				"file_transfer upload finalize session=%s: destination replace failed, "+
					".partial retained at %s: %v",
				sessionID, partialPath, err,
			)
		}
		return nil, err
	}

	a.FileTransferSessionsMu.Lock()
	delete(a.FileTransferSessions, sessionID)
	a.FileTransferSessionsMu.Unlock()
	a.removeUploadPartialJournal(sessionID)

	return map[string]interface{}{
		"status":           "completed",
		"destination_path": destinationPath,
		"sha256":           computedSHA,
	}, nil
}

func (a *Agent) AbortFilesUpload(p *NatsMsg) (map[string]interface{}, error) {
	sessionID, err := parsePayloadString(p.Data, "session_id")
	if err != nil {
		return nil, err
	}

	a.FileTransferSessionsMu.Lock()
	session, ok := a.FileTransferSessions[sessionID]
	if !ok || session == nil {
		a.FileTransferSessionsMu.Unlock()
		// agent restarted since the upload started
		if removed := a.removeJournaledUploadPartial(sessionID); removed != "" {
			a.Logger.Debugf(
				"file_transfer upload aborted session=%s: removed partial %s left from before a restart",
				sessionID, removed,
			)
		}
		return map[string]interface{}{"status": "aborted"}, nil
	}
	file := session.File
	partialPath := session.PartialPath
	hashJob := session.hashJob
	session.hashJob = nil
	session.File = nil
	delete(a.FileTransferSessions, sessionID)
	a.FileTransferSessionsMu.Unlock()

	hashJob.stop()

	if file != nil {
		_ = file.Close()
	}
	if partialPath != "" {
		if err := os.Remove(partialPath); err != nil && !os.IsNotExist(err) {
			a.Logger.Errorf(
				"file_transfer upload abort session=%s: failed to remove partial %s: %v",
				sessionID, partialPath, err,
			)
		}
	}

	a.removeUploadPartialJournal(sessionID)
	a.Logger.Debugf("file_transfer upload aborted session=%s", sessionID)
	return map[string]interface{}{"status": "aborted"}, nil
}

type DownloadTransferSession struct {
	SessionID     string
	SourcePath    string
	TotalSize     int64
	ChunkSize     int64
	File          *os.File
	StopStream    chan struct{}
	AckCh         chan struct{}
	LastActivity  time.Time
	Hasher        hash.Hash
	HashedOffset  int64
	PushedOffset  int64
	hashLive      bool
	hashJob       *resumeHashJob
	RemoveOnClose bool
	// DormantSince is set when the stream was parked without a client ACK;
	// the handle is closed but the session stays resumable.
	DormantSince time.Time
}

// downloadStreamIsCurrentLocked reports whether stop belongs to the session's
// live stream. Caller holds DownloadTransferSessionsMu.
func downloadStreamIsCurrentLocked(session *DownloadTransferSession, stop <-chan struct{}) bool {
	return session != nil && session.StopStream != nil &&
		(<-chan struct{})(session.StopStream) == stop
}

func (a *Agent) downloadStreamIsCurrent(sessionID string, stop <-chan struct{}) bool {
	a.DownloadTransferSessionsMu.Lock()
	defer a.DownloadTransferSessionsMu.Unlock()
	return downloadStreamIsCurrentLocked(a.DownloadTransferSessions[sessionID], stop)
}

func (a *Agent) HandleDownloadAck(p *NatsMsg) {
	sessionID, err := parsePayloadString(p.Data, "session_id")
	if err != nil {
		return
	}

	a.DownloadTransferSessionsMu.Lock()
	session, ok := a.DownloadTransferSessions[sessionID]
	var ackCh chan struct{}
	if ok && session != nil {
		ackCh = session.AckCh
		session.LastActivity = time.Now()
	}
	a.DownloadTransferSessionsMu.Unlock()
	if ackCh == nil {
		return
	}

	select {
	case ackCh <- struct{}{}:
	default:
	}
}

func (a *Agent) waitForDownloadAckSlot(sessionID string, stop <-chan struct{}) bool {
	var ackCh chan struct{}
	a.DownloadTransferSessionsMu.Lock()
	if s, ok := a.DownloadTransferSessions[sessionID]; ok && s != nil {
		ackCh = s.AckCh
	}
	a.DownloadTransferSessionsMu.Unlock()

	timer := time.NewTimer(downloadPushAckFallbackPoll)
	defer timer.Stop()

	if ackCh == nil {
		select {
		case <-stop:
			return false
		case <-timer.C:
			return true
		}
	}

	select {
	case <-stop:
		return false
	case <-ackCh:
		return true
	case <-timer.C:
		return true
	}
}

func (a *Agent) PrepareFilesDownload(p *NatsMsg) (map[string]interface{}, error) {
	sessionID, err := parsePayloadString(p.Data, "session_id")
	if err != nil {
		return nil, err
	}

	rawSourcePath, err := parsePayloadString(p.Data, "source_path")
	if err != nil {
		return nil, err
	}

	sourcePath, err := validateUploadDestinationPath(rawSourcePath)
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

	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source file not found: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("source path is a directory")
	}
	totalSize := info.Size()
	if totalSize == 0 {
		return nil, fmt.Errorf("source file is empty")
	}

	resume, resumeOffset := parsePayloadResume(p.Data)
	startOffset := int64(0)
	if resume {
		if resumeOffset < 0 || resumeOffset > totalSize {
			return nil, fmt.Errorf("invalid resume offset")
		}
		startOffset = resumeOffset
	}

	removeOnClose := strings.EqualFold(strings.TrimSpace(p.Data["remove_on_close"]), "true") &&
		a.isArchiveTempPath(sourcePath)

	file, err := openExistingFileSharedRead(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open source file: %w", err)
	}

	var hasher hash.Hash
	hashLive := startOffset == 0
	if hashLive {
		hasher = sha256.New()
	}

	stopStream := make(chan struct{})

	a.DownloadTransferSessionsMu.Lock()
	var oldHashJob *resumeHashJob
	if existing, ok := a.DownloadTransferSessions[sessionID]; ok && existing != nil {
		if existing.StopStream != nil {
			close(existing.StopStream)
			existing.StopStream = nil
		}
		if existing.File != nil {
			_ = existing.File.Close()
			existing.File = nil
		}
		oldHashJob = existing.hashJob
		existing.hashJob = nil
	}
	session := &DownloadTransferSession{
		SessionID:     sessionID,
		SourcePath:    sourcePath,
		TotalSize:     totalSize,
		ChunkSize:     chunkSize,
		File:          file,
		StopStream:    stopStream,
		AckCh:         make(chan struct{}, 1),
		LastActivity:  time.Now(),
		Hasher:        hasher,
		HashedOffset:  0,
		PushedOffset:  startOffset,
		hashLive:      hashLive,
		RemoveOnClose: removeOnClose,
	}
	var resumeJob *resumeHashJob
	var resumeCtx context.Context
	if startOffset > 0 {
		resumeJob, resumeCtx = newResumeHashJob()
		session.hashJob = resumeJob
		session.hashLive = false
		session.Hasher = nil
		session.HashedOffset = 0
	}
	a.DownloadTransferSessions[sessionID] = session
	a.DownloadTransferSessionsMu.Unlock()
	oldHashJob.stop()
	if resumeJob != nil {
		go a.runDownloadResumeHash(resumeCtx, resumeJob, sessionID, sourcePath, startOffset)
	}

	go a.streamDownloadChunks(sessionID, stopStream, startOffset)

	return map[string]interface{}{
		"status":     "ready",
		"total_size": totalSize,
		"chunk_size": chunkSize,
	}, nil
}

func (a *Agent) streamDownloadChunks(sessionID string, stop <-chan struct{}, startOffset int64) {
	defer a.recoverFilesRPC("files_download stream", nil)
	offset := startOffset
	backoff := downloadPushRetryMinBackoff
	attempt := 0
	var ackWaitStarted time.Time
	var lastAckWaitLog time.Time

	for {
		select {
		case <-stop:
			a.Logger.Debugf("file_transfer download stream session=%s stopped", sessionID)
			return
		default:
		}

		a.DownloadTransferSessionsMu.Lock()
		session, ok := a.DownloadTransferSessions[sessionID]
		if !ok || session == nil {
			a.DownloadTransferSessionsMu.Unlock()
			return
		}
		totalSize := session.TotalSize
		a.DownloadTransferSessionsMu.Unlock()

		if offset >= totalSize {
			a.Logger.Debugf(
				"file_transfer download stream session=%s complete offset=%d",
				sessionID, offset,
			)
			return
		}

		ready, err := a.getDownloadChunkReady(sessionID)
		if err != nil {
			if !isRetryableDownloadPushError(err) {
				a.Logger.Errorf(
					"file_transfer download stream session=%s offset=%d fatal ready err=%v",
					sessionID, offset, err,
				)
				a.failDownloadStream(sessionID, stop, err)
				return
			}
			ackWaitStarted = time.Time{}
			attempt++
			if attempt >= downloadPushMaxAttempts {
				a.Logger.Errorf(
					"file_transfer download stream session=%s offset=%d exhausted ready retries err=%v",
					sessionID, offset, err,
				)
				a.failDownloadStream(sessionID, stop, err)
				return
			}
			a.Logger.Debugf(
				"file_transfer download stream session=%s offset=%d ready retry %d/%d backoff=%s err=%v",
				sessionID, offset, attempt, downloadPushMaxAttempts, backoff, err,
			)
			timer := time.NewTimer(backoff)
			select {
			case <-stop:
				timer.Stop()
				return
			case <-timer.C:
			}
			backoff *= 2
			if backoff > downloadPushRetryMaxBackoff {
				backoff = downloadPushRetryMaxBackoff
			}
			continue
		}

		attempt = 0
		backoff = downloadPushRetryMinBackoff

		switch ready.Status {
		case "cancelled", "completed", "failed", "expired":
			a.Logger.Debugf(
				"file_transfer download stream session=%s stopped status=%s",
				sessionID, ready.Status,
			)
			return
		}

		if ready.OfferedOffset > offset {
			a.Logger.Debugf(
				"file_transfer download stream session=%s re-sync offset %d → %d",
				sessionID, offset, ready.OfferedOffset,
			)
			offset = ready.OfferedOffset
			attempt = 0
			backoff = downloadPushRetryMinBackoff
			ackWaitStarted = time.Time{}
			continue
		}

		if !ready.CanPut {
			if ackWaitStarted.IsZero() {
				ackWaitStarted = time.Now()
			}
			if time.Since(ackWaitStarted) >= downloadPushAckWaitMax {
				a.parkDownloadStream(sessionID, stop, offset)
				return
			}
			if lastAckWaitLog.IsZero() || time.Since(lastAckWaitLog) >= downloadPushAckWaitLogEvery {
				a.Logger.Debugf(
					"file_transfer download stream session=%s offset=%d waiting for client ACK (%s)",
					sessionID, offset, time.Since(ackWaitStarted).Round(time.Millisecond),
				)
				lastAckWaitLog = time.Now()
			}
			if !a.waitForDownloadAckSlot(sessionID, stop) {
				return
			}
			continue
		}

		offset = ready.OfferedOffset
		offeredOffset, err := a.pushDownloadChunk(sessionID, offset)
		if err == nil {
			offset = offeredOffset
			attempt = 0
			backoff = downloadPushRetryMinBackoff
			ackWaitStarted = time.Time{}
			continue
		}

		if synced, ok := parseExpectedDownloadOffset(err); ok && synced > offset {
			a.Logger.Debugf(
				"file_transfer download stream session=%s re-sync offset %d → %d (%v)",
				sessionID, offset, synced, err,
			)
			offset = synced
			attempt = 0
			backoff = downloadPushRetryMinBackoff
			ackWaitStarted = time.Time{}
			continue
		}

		if !isRetryableDownloadPushError(err) {
			if !a.downloadStreamIsCurrent(sessionID, stop) {
				// cancelled or resumed while this push was in flight
				a.Logger.Debugf(
					"file_transfer download stream session=%s offset=%d stale stream stopped: %v",
					sessionID, offset, err,
				)
				return
			}
			a.Logger.Errorf(
				"file_transfer download stream session=%s offset=%d fatal err=%v",
				sessionID, offset, err,
			)
			a.failDownloadStream(sessionID, stop, err)
			return
		}

		if isDownloadPushAckWaitError(err) {
			if ackWaitStarted.IsZero() {
				ackWaitStarted = time.Now()
			}
			if time.Since(ackWaitStarted) >= downloadPushAckWaitMax {
				a.parkDownloadStream(sessionID, stop, offset)
				return
			}
			if lastAckWaitLog.IsZero() || time.Since(lastAckWaitLog) >= downloadPushAckWaitLogEvery {
				a.Logger.Debugf(
					"file_transfer download stream session=%s offset=%d waiting for client ACK (%s)",
					sessionID, offset, time.Since(ackWaitStarted).Round(time.Millisecond),
				)
				lastAckWaitLog = time.Now()
			}
			if !a.waitForDownloadAckSlot(sessionID, stop) {
				return
			}
			continue
		}

		ackWaitStarted = time.Time{}
		attempt++
		if attempt >= downloadPushMaxAttempts {
			a.Logger.Errorf(
				"file_transfer download stream session=%s offset=%d exhausted retries err=%v",
				sessionID, offset, err,
			)
			a.failDownloadStream(sessionID, stop, err)
			return
		}

		a.Logger.Debugf(
			"file_transfer download stream session=%s offset=%d retry %d/%d after %v: %v",
			sessionID, offset, attempt, downloadPushMaxAttempts, backoff, err,
		)

		timer := time.NewTimer(backoff)
		select {
		case <-stop:
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff *= 2
		if backoff > downloadPushRetryMaxBackoff {
			backoff = downloadPushRetryMaxBackoff
		}
	}
}

func parseExpectedDownloadOffset(err error) (int64, bool) {
	if err == nil {
		return 0, false
	}
	m := downloadChunkExpectedOffsetRe.FindStringSubmatch(err.Error())
	if len(m) != 2 {
		return 0, false
	}
	n, perr := strconv.ParseInt(m[1], 10, 64)
	if perr != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func isRetryableDownloadPushError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "download session not found") ||
		strings.Contains(msg, "file handle is missing") ||
		strings.Contains(msg, "failed to read chunk") ||
		strings.Contains(msg, "short read") {
		return false
	}

	if strings.Contains(msg, "push download chunk failed: 4") ||
		strings.Contains(msg, "download chunk ready failed: 4") {
		if strings.Contains(msg, "failed: 408") ||
			strings.Contains(msg, "failed: 425") ||
			strings.Contains(msg, "failed: 429") {
			return true
		}
		if strings.Contains(msg, "not ready for chunk transfer") {
			return true
		}
		return false
	}
	// Network / timeout / 5xx / context deadline.
	return true
}

func isDownloadPushAckWaitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "failed: 408") &&
		strings.Contains(msg, "Timed out waiting for client to ACK")
}

// parkDownloadStream stops a stream whose client stopped ACKing (paused tab,
// slow link) and releases the source handle without failing the session, so
// a later resume re-prepares it. The reaper removes it after the partial
// retention window if it is never resumed.
func (a *Agent) parkDownloadStream(sessionID string, stop <-chan struct{}, offset int64) {
	a.DownloadTransferSessionsMu.Lock()
	session, ok := a.DownloadTransferSessions[sessionID]
	if !ok || !downloadStreamIsCurrentLocked(session, stop) {
		a.DownloadTransferSessionsMu.Unlock()
		return
	}
	close(session.StopStream)
	session.StopStream = nil
	file := session.File
	session.File = nil
	hashJob := session.hashJob
	session.hashJob = nil
	session.Hasher = nil
	session.hashLive = false
	session.DormantSince = time.Now()
	a.DownloadTransferSessionsMu.Unlock()

	hashJob.stop()
	if file != nil {
		_ = file.Close()
	}
	a.Logger.Debugf(
		"file_transfer download stream session=%s offset=%d parked: no client ACK for %s; handle released, resumable",
		sessionID, offset, downloadPushAckWaitMax,
	)
}

func (a *Agent) failDownloadStream(sessionID string, stop <-chan struct{}, cause error) {
	a.DownloadTransferSessionsMu.Lock()
	session, ok := a.DownloadTransferSessions[sessionID]
	if !ok || !downloadStreamIsCurrentLocked(session, stop) {
		// A resume or finalize replaced this stream; the failure belongs to a
		// stale goroutine and must not touch the new session.
		a.DownloadTransferSessionsMu.Unlock()
		a.Logger.Debugf(
			"file_transfer download stream session=%s stale stream failure ignored: %v",
			sessionID, cause,
		)
		return
	}
	delete(a.DownloadTransferSessions, sessionID)
	close(session.StopStream)
	session.StopStream = nil
	a.DownloadTransferSessionsMu.Unlock()

	if session != nil {
		session.hashJob.stop()
		if session.File != nil {
			_ = session.File.Close()
			session.File = nil
		}
		if session.RemoveOnClose && session.SourcePath != "" {
			if err := os.Remove(session.SourcePath); err != nil && !os.IsNotExist(err) {
				a.Logger.Errorf(
					"file_transfer download fail cleanup archive=%s session=%s err=%v",
					session.SourcePath, sessionID, err,
				)
			}
		}
	}

	msg := "download stream failed"
	if cause != nil {
		msg = cause.Error()
	}
	a.reportFileTransferFailure(sessionID, msg)
}

func (a *Agent) reportFileTransferFailure(sessionID, message string) {
	url := fmt.Sprintf("/api/v3/file-transfers/%s/fail/", sessionID)
	payload := map[string]string{"error": message}
	ctx, cancel := context.WithTimeout(context.Background(), fileTransferFailTimeout)
	defer cancel()
	resp, err := a.fileTransferHTTP().R().
		SetContext(ctx).
		SetBody(payload).
		Post(url)
	if err != nil {
		a.Logger.Errorf(
			"file_transfer fail callback session=%s err=%v", sessionID, err,
		)
		return
	}
	if resp.StatusCode() != 200 && resp.StatusCode() != 409 {
		a.Logger.Errorf(
			"file_transfer fail callback session=%s status=%d body=%s",
			sessionID, resp.StatusCode(), string(resp.Body()),
		)
	}
}

type downloadChunkReady struct {
	Status          string `json:"status"`
	CommittedOffset int64  `json:"committed_offset"`
	OfferedOffset   int64  `json:"offered_offset"`
	ChunkSize       int64  `json:"chunk_size"`
	TotalSize       int64  `json:"total_size"`
	CanPut          bool   `json:"can_put"`
}

func (a *Agent) getDownloadChunkReady(sessionID string) (downloadChunkReady, error) {
	var ready downloadChunkReady
	url := fmt.Sprintf("/api/v3/file-transfers/%s/download-chunk/", sessionID)
	ctx, cancel := context.WithTimeout(context.Background(), downloadReadyGetTimeout)
	defer cancel()

	resp, err := a.fileTransferHTTP().R().
		SetDebug(false).
		SetContext(ctx).
		Get(url)
	if err != nil {
		return ready, fmt.Errorf("failed to get download chunk ready: %w", err)
	}
	if resp.StatusCode() != 200 {
		return ready, fmt.Errorf(
			"download chunk ready failed: %d %s",
			resp.StatusCode(),
			string(resp.Body()),
		)
	}
	if err := json.Unmarshal(resp.Body(), &ready); err != nil {
		return ready, fmt.Errorf("download chunk ready: invalid response: %w", err)
	}
	return ready, nil
}

func (a *Agent) pushDownloadChunk(sessionID string, offset int64) (int64, error) {
	a.DownloadTransferSessionsMu.Lock()
	session, ok := a.DownloadTransferSessions[sessionID]
	if !ok || session == nil {
		a.DownloadTransferSessionsMu.Unlock()
		return 0, fmt.Errorf("download session not found")
	}
	if session.File == nil {
		a.DownloadTransferSessionsMu.Unlock()
		return 0, fmt.Errorf("download session file handle is missing")
	}
	session.LastActivity = time.Now()
	totalSize := session.TotalSize
	chunkSize := session.ChunkSize
	file := session.File
	a.DownloadTransferSessionsMu.Unlock()

	if offset >= totalSize {
		return offset, nil
	}

	remaining := totalSize - offset
	readSize := chunkSize
	if remaining < readSize {
		readSize = remaining
	}

	buf := make([]byte, readSize)
	readStart := time.Now()
	n, err := file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("failed to read chunk at offset %d: %w", offset, err)
	}
	if int64(n) != readSize {
		return 0, fmt.Errorf("short read at offset %d: got %d want %d", offset, n, readSize)
	}
	readMs := time.Since(readStart).Milliseconds()

	end := offset + int64(n) - 1
	contentRange := fmt.Sprintf("bytes %d-%d/%d", offset, end, totalSize)

	url := fmt.Sprintf("/api/v3/file-transfers/%s/download-chunk/", sessionID)
	ctx, cancel := context.WithTimeout(context.Background(), downloadPutTimeout)
	defer cancel()

	putStart := time.Now()
	resp, err := a.fileTransferHTTP().R().
		SetDebug(false).
		SetContext(ctx).
		SetHeader("Content-Range", contentRange).
		SetBody(buf[:n]).
		Put(url)
	if err != nil {
		return 0, fmt.Errorf("failed to push download chunk: %w", err)
	}
	putMs := time.Since(putStart).Milliseconds()

	if resp.StatusCode() != 200 {
		return 0, fmt.Errorf(
			"push download chunk failed: %d %s",
			resp.StatusCode(),
			string(resp.Body()),
		)
	}

	offeredOffset := end + 1
	var result struct {
		OfferedOffset int64 `json:"offered_offset"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err == nil && result.OfferedOffset > 0 {
		offeredOffset = result.OfferedOffset
	}

	a.DownloadTransferSessionsMu.Lock()
	if s, ok := a.DownloadTransferSessions[sessionID]; ok && s != nil {
		s.PushedOffset = offeredOffset
		if s.hashLive && s.Hasher != nil && offset == s.HashedOffset {
			s.Hasher.Write(buf[:n])
			s.HashedOffset = offeredOffset
		}
	}
	a.DownloadTransferSessionsMu.Unlock()

	a.Logger.Debugf(
		"file_transfer download chunk pushed session=%s offset=%d end=%d read_ms=%d put_ms=%d",
		sessionID, offset, end, readMs, putMs,
	)
	return offeredOffset, nil
}

func (a *Agent) FinalizeFilesDownload(p *NatsMsg) (map[string]interface{}, error) {
	sessionID, err := parsePayloadString(p.Data, "session_id")
	if err != nil {
		return nil, err
	}

	cancelledBuild := cancelArchiveBuild(sessionID)

	a.DownloadTransferSessionsMu.Lock()
	session, ok := a.DownloadTransferSessions[sessionID]
	if !ok || session == nil {
		a.DownloadTransferSessionsMu.Unlock()
		if cancelledBuild && a.Logger != nil {
			a.Logger.Debugf("file_transfer archive cancelled session=%s", sessionID)
		}
		return map[string]interface{}{"status": "completed"}, nil
	}
	hashJob := session.hashJob
	if strings.EqualFold(strings.TrimSpace(p.Data["release"]), "true") {
		// Cancel, fail or expiry: nobody needs the hash, so do not wait for
		// (or start) a full-file read inside the server's short release call.
		delete(a.DownloadTransferSessions, sessionID)
		session.hashJob = nil
		if session.StopStream != nil {
			close(session.StopStream)
			session.StopStream = nil
		}
		file := session.File
		session.File = nil
		a.DownloadTransferSessionsMu.Unlock()
		hashJob.stop()
		if file != nil {
			_ = file.Close()
		}
		if session.RemoveOnClose && session.SourcePath != "" {
			if err := os.Remove(session.SourcePath); err != nil && !os.IsNotExist(err) {
				a.Logger.Errorf(
					"file_transfer download release session=%s: failed to remove archive %s: %v",
					sessionID, session.SourcePath, err,
				)
			}
		}
		a.Logger.Debugf("file_transfer download released session=%s", sessionID)
		return map[string]interface{}{"status": "completed"}, nil
	}
	a.DownloadTransferSessionsMu.Unlock()

	if err := waitResumeHashJob(hashJob); err != nil && a.Logger != nil {
		a.Logger.Debugf(
			"file_transfer download finalize session=%s: resume prefix hash: %v",
			sessionID, err,
		)
	}

	a.DownloadTransferSessionsMu.Lock()
	session, ok = a.DownloadTransferSessions[sessionID]
	if !ok || session == nil {
		a.DownloadTransferSessionsMu.Unlock()
		if cancelledBuild && a.Logger != nil {
			a.Logger.Debugf("file_transfer archive cancelled session=%s", sessionID)
		}
		return map[string]interface{}{"status": "completed"}, nil
	}
	stopStream := session.StopStream
	file := session.File
	sourcePath := session.SourcePath
	totalSize := session.TotalSize
	hasher := session.Hasher
	hashedOffset := session.HashedOffset
	removeOnClose := session.RemoveOnClose
	session.hashJob = nil
	delete(a.DownloadTransferSessions, sessionID)
	a.DownloadTransferSessionsMu.Unlock()

	computedSHA := ""
	if hasher != nil && hashedOffset == totalSize {
		computedSHA = hex.EncodeToString(hasher.Sum(nil))
	} else if hasher != nil && hashedOffset >= 0 && hashedOffset < totalSize && sourcePath != "" {
		if herr := hashFileRange(context.Background(), hasher, sourcePath, hashedOffset, totalSize); herr == nil {
			computedSHA = hex.EncodeToString(hasher.Sum(nil))
		} else if a.Logger != nil {
			a.Logger.Errorf(
				"file_transfer download finalize session=%s: failed to hash remainder: %v",
				sessionID, herr,
			)
		}
	} else if sourcePath != "" {
		if sha, herr := hashFileSHA256(sourcePath); herr == nil {
			computedSHA = sha
		} else {
			a.Logger.Errorf(
				"file_transfer download finalize session=%s: failed to hash source: %v",
				sessionID, herr,
			)
		}
	}

	if stopStream != nil {
		close(stopStream)
	}
	if file != nil {
		_ = file.Close()
	}
	if removeOnClose && sourcePath != "" {
		if err := os.Remove(sourcePath); err != nil && !os.IsNotExist(err) {
			a.Logger.Errorf(
				"file_transfer download finalize session=%s: failed to remove archive %s: %v",
				sessionID, sourcePath, err,
			)
		}
	}

	return map[string]interface{}{
		"status": "completed",
		"sha256": computedSHA,
	}, nil
}

func hashFileSHA256(path string) (string, error) {
	f, err := openExistingFileSharedRead(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type resumeHashJob struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func newResumeHashJob() (*resumeHashJob, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	return &resumeHashJob{cancel: cancel, done: make(chan struct{})}, ctx
}

func (j *resumeHashJob) stop() {
	if j == nil {
		return
	}
	j.cancel()
	<-j.done
}

func waitResumeHashJob(j *resumeHashJob) error {
	if j == nil {
		return nil
	}
	<-j.done
	return j.err
}

func hashFileRange(ctx context.Context, h hash.Hash, path string, start, end int64) error {
	if h == nil {
		return fmt.Errorf("hasher is nil")
	}
	if end < start {
		return fmt.Errorf("invalid hash range")
	}
	if end == start {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	f, err := openExistingFileSharedRead(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return err
	}
	buf := make([]byte, 1024*1024)
	reader := io.LimitReader(f, end-start)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := reader.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func (a *Agent) runUploadResumeHash(
	ctx context.Context, job *resumeHashJob, sessionID, path string, prefix int64,
) {
	defer a.recoverFilesRPC("files_upload resume hash", nil)
	defer close(job.done)
	h := sha256.New()
	if err := hashFileRange(ctx, h, path, 0, prefix); err != nil {
		job.err = err
		return
	}
	a.FileTransferSessionsMu.Lock()
	session, ok := a.FileTransferSessions[sessionID]
	if !ok || session == nil || session.hashJob != job {
		a.FileTransferSessionsMu.Unlock()
		return
	}
	session.Hasher = h
	session.HashedOffset = prefix
	a.FileTransferSessionsMu.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			job.err = err
			return
		}
		a.FileTransferSessionsMu.Lock()
		session, ok = a.FileTransferSessions[sessionID]
		if !ok || session == nil || session.hashJob != job {
			a.FileTransferSessionsMu.Unlock()
			return
		}
		from := session.HashedOffset
		to := session.CommittedOffset
		if from >= to {
			session.hashLive = true
			a.FileTransferSessionsMu.Unlock()
			return
		}
		a.FileTransferSessionsMu.Unlock()
		if err := hashFileRange(ctx, h, path, from, to); err != nil {
			job.err = err
			return
		}
		a.FileTransferSessionsMu.Lock()
		session, ok = a.FileTransferSessions[sessionID]
		if !ok || session == nil || session.hashJob != job {
			a.FileTransferSessionsMu.Unlock()
			return
		}
		if session.HashedOffset == from {
			session.HashedOffset = to
		}
		a.FileTransferSessionsMu.Unlock()
	}
}

func (a *Agent) runDownloadResumeHash(
	ctx context.Context, job *resumeHashJob, sessionID, path string, prefix int64,
) {
	defer a.recoverFilesRPC("files_download resume hash", nil)
	defer close(job.done)
	h := sha256.New()
	if err := hashFileRange(ctx, h, path, 0, prefix); err != nil {
		job.err = err
		return
	}
	a.DownloadTransferSessionsMu.Lock()
	session, ok := a.DownloadTransferSessions[sessionID]
	if !ok || session == nil || session.hashJob != job {
		a.DownloadTransferSessionsMu.Unlock()
		return
	}
	session.Hasher = h
	session.HashedOffset = prefix
	a.DownloadTransferSessionsMu.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			job.err = err
			return
		}
		a.DownloadTransferSessionsMu.Lock()
		session, ok = a.DownloadTransferSessions[sessionID]
		if !ok || session == nil || session.hashJob != job {
			a.DownloadTransferSessionsMu.Unlock()
			return
		}
		from := session.HashedOffset
		to := session.PushedOffset
		if from >= to {
			session.hashLive = true
			a.DownloadTransferSessionsMu.Unlock()
			return
		}
		a.DownloadTransferSessionsMu.Unlock()
		if err := hashFileRange(ctx, h, path, from, to); err != nil {
			job.err = err
			return
		}
		a.DownloadTransferSessionsMu.Lock()
		session, ok = a.DownloadTransferSessions[sessionID]
		if !ok || session == nil || session.hashJob != job {
			a.DownloadTransferSessionsMu.Unlock()
			return
		}
		if session.HashedOffset == from {
			session.HashedOffset = to
		}
		a.DownloadTransferSessionsMu.Unlock()
	}
}

func (a *Agent) ReapStaleFileTransferSessions() {
	now := time.Now()
	pruneUploadFinalizeCalls(now)

	type uploadCloseJob struct {
		sessionID string
		file      *os.File
	}
	var toRelease []uploadCloseJob
	var toRemove []*UploadTransferSession

	a.FileTransferSessionsMu.Lock()
	for id, session := range a.FileTransferSessions {
		if session == nil {
			delete(a.FileTransferSessions, id)
			continue
		}
		if session.File != nil {
			if now.Sub(session.LastActivity) > fileTransferSessionIdleTimeout {
				toRelease = append(toRelease, uploadCloseJob{id, session.File})
				session.File = nil
				session.DormantSince = now
			}
			continue
		}
		if !session.DormantSince.IsZero() &&
			now.Sub(session.DormantSince) > fileTransferPartialRetention {
			toRemove = append(toRemove, session)
			delete(a.FileTransferSessions, id)
		}
	}
	a.FileTransferSessionsMu.Unlock()

	for _, job := range toRelease {
		_ = job.file.Close()
		a.Logger.Debugf(
			"file_transfer reaper: upload session=%s idle, handle released; "+
				".partial kept for resume",
			job.sessionID,
		)
	}
	for _, session := range toRemove {
		session.hashJob.stop()
		a.removeUploadPartialJournal(session.SessionID)
		if session.PartialPath != "" {
			if err := os.Remove(session.PartialPath); err != nil && !os.IsNotExist(err) {
				a.Logger.Errorf(
					"file_transfer reaper: failed to remove partial file %s for session=%s: %v",
					session.PartialPath, session.SessionID, err,
				)
			}
		}
		a.Logger.Debugf(
			"file_transfer reaper: removed dormant upload session=%s "+
				"(.partial retention elapsed)",
			session.SessionID,
		)
	}

	var staleDownloads []*DownloadTransferSession
	a.DownloadTransferSessionsMu.Lock()
	for id, session := range a.DownloadTransferSessions {
		if session == nil {
			delete(a.DownloadTransferSessions, id)
			continue
		}
		if !session.DormantSince.IsZero() {
			// Parked for a paused client: keep it (and any archive) resumable
			// for the same window as an upload .partial.
			if now.Sub(session.DormantSince) > fileTransferPartialRetention {
				staleDownloads = append(staleDownloads, session)
				delete(a.DownloadTransferSessions, id)
			}
			continue
		}
		if now.Sub(session.LastActivity) > fileTransferSessionIdleTimeout {
			staleDownloads = append(staleDownloads, session)
			delete(a.DownloadTransferSessions, id)
		}
	}
	a.DownloadTransferSessionsMu.Unlock()

	for _, session := range staleDownloads {
		session.hashJob.stop()
		if session.StopStream != nil {
			close(session.StopStream)
		}
		if session.File != nil {
			_ = session.File.Close()
		}
		if session.RemoveOnClose && session.SourcePath != "" {
			if err := os.Remove(session.SourcePath); err != nil && !os.IsNotExist(err) {
				a.Logger.Errorf(
					"file_transfer reaper: failed to remove archive %s session=%s: %v",
					session.SourcePath, session.SessionID, err,
				)
			}
		}
		a.Logger.Debugf(
			"file_transfer reaper: reaped idle download session=%s idle=%s",
			session.SessionID, now.Sub(session.LastActivity).Round(time.Second),
		)
	}

	a.sweepOrphanedUploadPartials(now)
}
