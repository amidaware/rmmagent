/*
Copyright 2026 Amidaware Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	fileBrowserDefaultPageSize      = 500
	fileBrowserMaxPageSize          = 1000
	fileBrowserMaxPage              = 10_000
	fileBrowserFilterSnapshotTTL    = 60 * time.Second
	fileBrowserFilterSnapshotMax    = 32
	fileBrowserFilterMaxLen         = 255
	fileBrowserMaxExistsNames       = 500
	defaultFolderSummaryMaxFiles    = 100_000
	defaultFolderSummaryMaxDepth    = 32
	defaultFolderSummaryMaxDuration = 20 * time.Second
	maxFolderSummaryDuration        = 40 * time.Second
	maxConcurrentFolderSummaries    = 2
)

var folderSummarySlots = make(chan struct{}, maxConcurrentFolderSummaries)

type fileBrowserItem struct {
	ID        string
	Name      string
	Path      string
	Type      string
	Extension string
	Size      string
	Modified  string
	Created   string
	Accessed  string
	Hidden    bool
	System    bool
	ReadOnly  bool
}

type folderSummaryLimits struct {
	maxFiles    int
	maxDepth    int
	maxDuration time.Duration
}

type folderSummary struct {
	totalBytes  int64
	fileCount   int
	folderCount int
	truncated   bool
}

func parseFolderSummaryLimits(data map[string]string) folderSummaryLimits {
	limits := folderSummaryLimits{
		maxFiles:    defaultFolderSummaryMaxFiles,
		maxDepth:    defaultFolderSummaryMaxDepth,
		maxDuration: defaultFolderSummaryMaxDuration,
	}
	if data == nil {
		return limits
	}
	if v, err := strconv.Atoi(strings.TrimSpace(data["max_files"])); err == nil && v > 0 {
		limits.maxFiles = clampAtMostInt(v, defaultFolderSummaryMaxFiles)
	}
	if v, err := strconv.Atoi(strings.TrimSpace(data["max_depth"])); err == nil && v > 0 {
		limits.maxDepth = clampAtMostInt(v, defaultFolderSummaryMaxDepth)
	}
	if v, err := strconv.Atoi(strings.TrimSpace(data["max_duration_seconds"])); err == nil && v > 0 {
		limits.maxDuration = time.Duration(clampInt(v, 1, int(maxFolderSummaryDuration/time.Second))) * time.Second
	}
	return limits
}

func fsIdentityKey(s string) string {
	return fsIdentityKeyFor(s, runtime.GOOS)
}

func fsIdentityKeyFor(s, goos string) string {
	if goos == "windows" {
		return strings.ToLower(s)
	}
	return s
}

func parsePayloadNamesJSON(data map[string]string) ([]string, error) {
	raw, err := parsePayloadString(data, "names_json")
	if err != nil {
		return nil, err
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return nil, fmt.Errorf("invalid names_json")
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("names_json must contain at least one name")
	}
	cleaned := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if err := validateFileBrowserName(name); err != nil {
			return nil, fmt.Errorf("invalid name %q", name)
		}
		key := fsIdentityKey(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, name)
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("names_json must contain at least one valid name")
	}
	if len(cleaned) > fileBrowserMaxExistsNames {
		return nil, fmt.Errorf("too many names (max %d)", fileBrowserMaxExistsNames)
	}
	return cleaned, nil
}

func fileExistsNames(rawDir string, names []string) (map[string]interface{}, error) {
	dir, err := validateUploadDestinationPath(rawDir)
	if err != nil {
		return nil, fmt.Errorf("invalid path")
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path not found")
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		return nil, fmt.Errorf("unable to access path")
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	existing := make([]string, 0)
	for _, name := range names {
		candidate := filepath.Join(dir, name)
		if _, statErr := os.Lstat(candidate); statErr == nil {
			existing = append(existing, name)
			continue
		} else if !os.IsNotExist(statErr) {
			if os.IsPermission(statErr) {
				return nil, fmt.Errorf("permission denied")
			}
			return nil, fmt.Errorf("unable to access path")
		}
	}
	return map[string]interface{}{"existing": existing}, nil
}

func summarizeFolder(root string, limits folderSummaryLimits) folderSummary {
	summary := folderSummary{}
	deadline := time.Now().Add(limits.maxDuration)
	entriesSeen := 0

	wait := time.NewTimer(limits.maxDuration)
	select {
	case folderSummarySlots <- struct{}{}:
		wait.Stop()
		defer func() { <-folderSummarySlots }()
	case <-wait.C:
		summary.truncated = true
		return summary
	}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip unreadable nodes
			if d != nil && d.IsDir() && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		if time.Now().After(deadline) {
			summary.truncated = true
			return filepath.SkipAll
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		depth := len(strings.Split(filepath.ToSlash(rel), "/"))
		if depth > limits.maxDepth {
			summary.truncated = true
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entriesSeen++
		if entriesSeen > limits.maxFiles {
			summary.truncated = true
			return filepath.SkipAll
		}

		if d.IsDir() {
			summary.folderCount++
			return nil
		}

		summary.fileCount++
		if size := info.Size(); size > 0 {
			summary.totalBytes += size
		}
		return nil
	})

	return summary
}

func parseFileBrowserPageParams(data map[string]string) (int, int) {
	page := 1
	pageSize := fileBrowserDefaultPageSize

	if v, err := strconv.Atoi(strings.TrimSpace(data["page"])); err == nil && v >= 1 {
		page = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(data["page_size"])); err == nil && v >= 1 {
		pageSize = v
	}
	return normalizeFileBrowserPage(page, pageSize)
}

func parseFileBrowserNameFilter(data map[string]string) string {
	if data == nil {
		return ""
	}
	raw := strings.TrimSpace(data["filter"])
	if raw == "" {
		return ""
	}
	raw = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == 0 {
			return -1
		}
		return r
	}, raw)
	raw = strings.TrimSpace(raw)
	if len(raw) > fileBrowserFilterMaxLen {
		raw = raw[:fileBrowserFilterMaxLen]
	}
	return raw
}

type fileBrowserFilterSnapshot struct {
	path    string
	filter  string
	items   []fileBrowserItem
	expires time.Time
}

var (
	fileBrowserFilterSnapshotsMu sync.Mutex
	fileBrowserFilterSnapshots   = map[string]*fileBrowserFilterSnapshot{}
)

func fileBrowserFilterSnapshotKey(path, filter string) string {
	return path + "\x00" + strings.ToLower(filter)
}

func getFileBrowserFilterSnapshot(path, filter string) ([]fileBrowserItem, bool) {
	key := fileBrowserFilterSnapshotKey(path, filter)
	now := time.Now()

	fileBrowserFilterSnapshotsMu.Lock()
	defer fileBrowserFilterSnapshotsMu.Unlock()

	snap, ok := fileBrowserFilterSnapshots[key]
	if !ok {
		return nil, false
	}
	if now.After(snap.expires) {
		delete(fileBrowserFilterSnapshots, key)
		return nil, false
	}
	return snap.items, true
}

func storeFileBrowserFilterSnapshot(path, filter string, items []fileBrowserItem) {
	key := fileBrowserFilterSnapshotKey(path, filter)
	now := time.Now()

	fileBrowserFilterSnapshotsMu.Lock()
	defer fileBrowserFilterSnapshotsMu.Unlock()

	if len(fileBrowserFilterSnapshots) >= fileBrowserFilterSnapshotMax {
		var oldestKey string
		var oldestExp time.Time
		for k, snap := range fileBrowserFilterSnapshots {
			if now.After(snap.expires) {
				delete(fileBrowserFilterSnapshots, k)
				continue
			}
			if oldestKey == "" || snap.expires.Before(oldestExp) {
				oldestKey = k
				oldestExp = snap.expires
			}
		}
		if len(fileBrowserFilterSnapshots) >= fileBrowserFilterSnapshotMax && oldestKey != "" {
			delete(fileBrowserFilterSnapshots, oldestKey)
		}
	}

	copied := make([]fileBrowserItem, len(items))
	copy(copied, items)
	fileBrowserFilterSnapshots[key] = &fileBrowserFilterSnapshot{
		path:    path,
		filter:  filter,
		items:   copied,
		expires: now.Add(fileBrowserFilterSnapshotTTL),
	}
}

func clearFileBrowserFilterSnapshotsForTest() {
	fileBrowserFilterSnapshotsMu.Lock()
	defer fileBrowserFilterSnapshotsMu.Unlock()
	fileBrowserFilterSnapshots = map[string]*fileBrowserFilterSnapshot{}
}

func fileBrowserSnapshotPathAffected(snapPath, root string) bool {
	snapPath = filepath.Clean(snapPath)
	root = filepath.Clean(root)
	if snapPath == root {
		return true
	}
	sep := string(os.PathSeparator)
	prefix := root
	if !strings.HasSuffix(prefix, sep) {
		prefix += sep
	}
	return strings.HasPrefix(snapPath, prefix)
}

func invalidateFileBrowserListingsFor(paths ...string) {
	seen := make(map[string]struct{}, len(paths)*2)
	roots := make([]string, 0, len(paths)*2)
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		cleaned, err := validateUploadDestinationPath(p)
		if err != nil {
			cleaned = filepath.Clean(p)
		}
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		roots = append(roots, cleaned)
	}
	for _, p := range paths {
		add(p)
		cleaned, err := validateUploadDestinationPath(p)
		if err != nil {
			cleaned = filepath.Clean(strings.TrimSpace(p))
		}
		parent := filepath.Dir(cleaned)
		if parent != "" && parent != "." && parent != cleaned {
			add(parent)
		}
	}
	if len(roots) == 0 {
		return
	}

	fileBrowserFilterSnapshotsMu.Lock()
	defer fileBrowserFilterSnapshotsMu.Unlock()
	for key, snap := range fileBrowserFilterSnapshots {
		if snap == nil {
			delete(fileBrowserFilterSnapshots, key)
			continue
		}
		for _, root := range roots {
			if fileBrowserSnapshotPathAffected(snap.path, root) {
				delete(fileBrowserFilterSnapshots, key)
				break
			}
		}
	}
}

func nameMatchesFileBrowserFilter(name, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(filter))
}

func mulNonNeg(a, b int) (int, bool) {
	if a < 0 || b < 0 {
		return 0, false
	}
	if a == 0 || b == 0 {
		return 0, true
	}
	if a > math.MaxInt/b {
		return 0, false
	}
	product := a * b
	if product < 0 {
		return 0, false
	}
	return product, true
}

func paginateFileBrowserItems(items []fileBrowserItem, page, pageSize int) (encoded []map[string]interface{}, total int, hasMore bool) {
	total = len(items)
	encoded = make([]map[string]interface{}, 0)
	page, pageSize = normalizeFileBrowserPage(page, pageSize)

	start, ok := mulNonNeg(page-1, pageSize)
	if !ok || start >= total {
		return encoded, total, false
	}
	end := start + pageSize
	if end < start || end > total {
		end = total
	}
	hasMore = end < total

	pageItems := items[start:end]
	encoded = make([]map[string]interface{}, 0, len(pageItems))
	for _, item := range pageItems {
		encoded = append(encoded, item.toMap())
	}
	return encoded, total, hasMore
}

func uploadPartialNameKeysIn(a *Agent, dir string) map[string]struct{} {
	if a == nil {
		return nil
	}
	dirKey := fsIdentityKey(filepath.Clean(dir))
	a.FileTransferSessionsMu.Lock()
	defer a.FileTransferSessionsMu.Unlock()
	var hidden map[string]struct{}
	for _, session := range a.FileTransferSessions {
		if session == nil || strings.TrimSpace(session.PartialPath) == "" {
			continue
		}
		partial := filepath.Clean(session.PartialPath)
		if fsIdentityKey(filepath.Dir(partial)) != dirKey {
			continue
		}
		if hidden == nil {
			hidden = make(map[string]struct{})
		}
		hidden[fsIdentityKey(filepath.Base(partial))] = struct{}{}
	}
	return hidden
}

func excludeUploadPartialItems(a *Agent, dir string, items []fileBrowserItem) []fileBrowserItem {
	hidden := uploadPartialNameKeysIn(a, dir)
	if len(hidden) == 0 {
		return items
	}
	out := make([]fileBrowserItem, 0, len(items))
	for _, item := range items {
		if _, ok := hidden[fsIdentityKey(item.Name)]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func buildDirectoryItems(cleaned string) ([]fileBrowserItem, error) {
	entries, err := os.ReadDir(cleaned)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		return nil, fmt.Errorf("unable to read directory")
	}

	items := make([]fileBrowserItem, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		fullPath := filepath.Join(cleaned, name)
		entryInfo, err := entry.Info()
		if err != nil {
			entryInfo, err = os.Lstat(fullPath)
			if err != nil {
				continue
			}
		}

		items = append(items, buildFileBrowserItem(cleaned, name, entryInfo))
	}
	return items, nil
}

func extensionFromName(name string) string {
	ext := filepath.Ext(name)
	if ext == "" || ext == "." || ext == ".." {
		return ""
	}
	return strings.ToUpper(strings.TrimPrefix(ext, "."))
}

func formatFileTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func classifyEntryType(fullPath string, info os.FileInfo) string {
	if info.IsDir() {
		return "folder"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Stat(fullPath); err == nil && target.IsDir() {
			return "folder"
		}
	}
	return "file"
}

func buildFileBrowserItem(dirPath, name string, info os.FileInfo) fileBrowserItem {
	fullPath := filepath.Join(dirPath, name)
	itemType := classifyEntryType(fullPath, info)
	modified, created, accessed := fileTimes(info)
	hidden, system, readOnly := fileAttributeFlags(fullPath, info, name)

	size := "0"
	extension := ""
	if itemType == "file" {
		size = strconv.FormatInt(info.Size(), 10)
		extension = extensionFromName(name)
	}

	return fileBrowserItem{
		ID:        fullPath,
		Name:      name,
		Path:      fullPath,
		Type:      itemType,
		Extension: extension,
		Size:      size,
		Modified:  modified,
		Created:   created,
		Accessed:  accessed,
		Hidden:    hidden,
		System:    system,
		ReadOnly:  readOnly,
	}
}

func (item fileBrowserItem) toMap() map[string]interface{} {
	out := map[string]interface{}{
		"id":       item.ID,
		"name":     item.Name,
		"path":     item.Path,
		"type":     item.Type,
		"size":     item.Size,
		"modified": item.Modified,
		"created":  item.Created,
		"accessed": item.Accessed,
		"hidden":   item.Hidden,
		"system":   item.System,
		"readonly": item.ReadOnly,
	}
	if item.Extension != "" {
		out["extension"] = item.Extension
	}
	return out
}

func compareFileBrowserItems(a, b fileBrowserItem) int {
	aFolder := a.Type == "folder"
	bFolder := b.Type == "folder"
	if aFolder && !bFolder {
		return -1
	}
	if !aFolder && bFolder {
		return 1
	}
	return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

func normalizeFileBrowserPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if page > fileBrowserMaxPage {
		page = fileBrowserMaxPage
	}
	if pageSize < 1 {
		pageSize = fileBrowserDefaultPageSize
	}
	if pageSize > fileBrowserMaxPageSize {
		pageSize = fileBrowserMaxPageSize
	}
	return page, pageSize
}

func listDirectory(a *Agent, rawPath string, page, pageSize int, nameFilter string) (map[string]interface{}, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		resolved, resolveErr := resolveDefaultFileBrowserPath()
		if resolveErr != nil {
			return nil, resolveErr
		}
		path = resolved
	}

	cleaned, err := validateUploadDestinationPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path")
	}

	info, err := os.Stat(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path not found")
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		return nil, fmt.Errorf("unable to access path")
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	page, pageSize = normalizeFileBrowserPage(page, pageSize)
	nameFilter = strings.TrimSpace(nameFilter)

	var items []fileBrowserItem

	if nameFilter != "" {
		if cached, ok := getFileBrowserFilterSnapshot(cleaned, nameFilter); ok {
			items = cached
		} else {
			built, buildErr := buildDirectoryItems(cleaned)
			if buildErr != nil {
				return nil, buildErr
			}
			filtered := make([]fileBrowserItem, 0, len(built))
			for _, item := range built {
				if nameMatchesFileBrowserFilter(item.Name, nameFilter) {
					filtered = append(filtered, item)
				}
			}
			sort.Slice(filtered, func(i, j int) bool {
				return compareFileBrowserItems(filtered[i], filtered[j]) < 0
			})
			storeFileBrowserFilterSnapshot(cleaned, nameFilter, filtered)
			items = filtered
		}
	} else {
		if page > 1 {
			if cached, ok := getFileBrowserFilterSnapshot(cleaned, ""); ok {
				items = cached
			}
		}
		if items == nil {
			built, buildErr := buildDirectoryItems(cleaned)
			if buildErr != nil {
				return nil, buildErr
			}
			sort.Slice(built, func(i, j int) bool {
				return compareFileBrowserItems(built[i], built[j]) < 0
			})
			storeFileBrowserFilterSnapshot(cleaned, "", built)
			items = built
		}
	}

	items = excludeUploadPartialItems(a, cleaned, items)
	encoded, total, hasMore := paginateFileBrowserItems(items, page, pageSize)

	return map[string]interface{}{
		"path":      cleaned,
		"items":     encoded,
		"has_more":  hasMore,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	}, nil
}

func resolveDefaultFileBrowserPath() (string, error) {
	candidates := defaultFileBrowserPathCandidates()
	if path := firstReadableDirectory(candidates); path != "" {
		return path, nil
	}
	return "", fmt.Errorf("no readable default directory found")
}

func defaultFileBrowserPathCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		return defaultWindowsFileBrowserPathCandidates()
	case "darwin":
		return []string{"/Users", "/"}
	default:
		// linux and other unix-like agents
		return []string{"/home", "/"}
	}
}

func firstReadableDirectory(candidates []string) string {
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		cleaned := normalizeFileBrowserCandidate(candidate)
		if cleaned == "" {
			continue
		}
		key := fsIdentityKey(cleaned)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		info, err := os.Stat(cleaned)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.ReadDir(cleaned); err != nil {
			continue
		}
		return cleaned
	}
	return ""
}

func normalizeFileBrowserCandidate(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	if runtime.GOOS == "windows" && len(candidate) == 2 && candidate[1] == ':' {
		candidate += `\`
	}
	cleaned, err := validateUploadDestinationPath(candidate)
	if err != nil {
		return ""
	}
	return cleaned
}

func entryNameFromPath(cleaned string) string {
	name := filepath.Base(cleaned)
	if name == "." || name == "" {
		return cleaned
	}
	if name == string(os.PathSeparator) {
		trimmed := strings.TrimRight(cleaned, string(os.PathSeparator))
		if trimmed != "" {
			return filepath.Base(trimmed)
		}
		return cleaned
	}
	return name
}

func fileProperties(rawPath string, limits *folderSummaryLimits) (map[string]interface{}, error) {
	cleaned, err := validateUploadDestinationPath(rawPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path")
	}

	info, err := os.Lstat(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path not found")
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		return nil, fmt.Errorf("unable to access path")
	}

	parent := filepath.Dir(cleaned)
	name := entryNameFromPath(cleaned)
	item := buildFileBrowserItem(parent, name, info)
	item.Path = cleaned
	item.ID = cleaned
	item.Name = name

	out := item.toMap()
	out["location"] = parent

	if item.Type == "folder" && limits != nil {
		summary := summarizeFolder(cleaned, *limits)
		out["size"] = strconv.FormatInt(summary.totalBytes, 10)
		out["file_count"] = summary.fileCount
		out["folder_count"] = summary.folderCount
		out["summary_truncated"] = summary.truncated
	}

	return out, nil
}

func validateFileBrowserName(name string) error {
	if hasTrailingSpaceOrPeriod(name, runtime.GOOS) {
		return fmt.Errorf("invalid name")
	}
	trimmed := strings.TrimSpace(name)
	if err := validateUploadFilename(trimmed); err != nil {
		return err
	}
	return nil
}

func deletePathKey(s, goos string) string {
	s = strings.TrimSpace(s)
	if goos == "windows" {
		s = strings.ReplaceAll(s, "/", `\`)
		s = strings.ToLower(s)
		return strings.TrimRight(s, `\`)
	}
	if s != "/" {
		s = strings.TrimRight(s, "/")
	}
	return s
}

func sameDeletePath(a, b, goos string) bool {
	if a == "" || b == "" {
		return false
	}
	return deletePathKey(a, goos) == deletePathKey(b, goos)
}

func isVolumeRootDeletePath(cleaned, goos string) bool {
	key := deletePathKey(cleaned, goos)
	if key == "" {
		return false
	}
	if goos == "windows" {
		return isWindowsVolumeRootKey(key)
	}
	return key == "/"
}

func isWindowsVolumeRootKey(key string) bool {
	isDrive := func(k string) bool {
		return len(k) == 2 && k[0] >= 'a' && k[0] <= 'z' && k[1] == ':'
	}
	if isDrive(key) {
		return true
	}
	if !strings.HasPrefix(key, `\\`) {
		return false
	}
	rest := key[2:]
	switch {
	case strings.HasPrefix(rest, `?\unc\`):
		rest = rest[len(`?\unc\`):]
	case strings.HasPrefix(rest, `?\`), strings.HasPrefix(rest, `.\`):
		return isDrive(strings.TrimRight(rest[2:], `\`))
	}
	segments := 0
	for _, part := range strings.Split(rest, `\`) {
		if part != "" {
			segments++
		}
	}
	return segments >= 1 && segments <= 2
}

func isStrictDeleteAncestor(parent, child, goos string) bool {
	p := deletePathKey(parent, goos)
	c := deletePathKey(child, goos)
	if p == "" || c == "" || p == c {
		return false
	}
	if goos == "windows" {
		if isVolumeRootDeletePath(parent, goos) {
			return strings.HasPrefix(c, p+`\`)
		}
		return strings.HasPrefix(c, p+`\`)
	}
	if p == "/" {
		return c != "/"
	}
	return strings.HasPrefix(c, p+"/")
}

func wellKnownSystemDeletePaths(goos string) []string {
	if goos == "windows" {
		paths := []string{
			`C:\Windows`,
			`C:\Windows\System32`,
			`C:\Windows\SysWOW64`,
			`C:\Program Files`,
			`C:\Program Files (x86)`,
			`C:\ProgramData`,
			`C:\Users`,
		}
		if runtime.GOOS == "windows" {
			if v := strings.TrimSpace(os.Getenv("SystemRoot")); v != "" {
				paths = append(paths, v, filepath.Join(v, "System32"), filepath.Join(v, "SysWOW64"))
			}
			if v := strings.TrimSpace(os.Getenv("ProgramFiles")); v != "" {
				paths = append(paths, v)
			}
			if v := strings.TrimSpace(os.Getenv("ProgramFiles(x86)")); v != "" {
				paths = append(paths, v)
			}
			if v := strings.TrimSpace(os.Getenv("ProgramData")); v != "" {
				paths = append(paths, v)
			}
			if v := strings.TrimSpace(os.Getenv("SystemDrive")); v != "" {
				drive := strings.TrimRight(v, `/\`)
				if len(drive) == 2 && drive[1] == ':' {
					paths = append(paths,
						filepath.Join(drive+`\`, "Windows"),
						filepath.Join(drive+`\`, "Program Files"),
						filepath.Join(drive+`\`, "Program Files (x86)"),
						filepath.Join(drive+`\`, "ProgramData"),
						filepath.Join(drive+`\`, "Users"),
					)
				}
			}
		}
		return paths
	}

	paths := []string{
		"/bin", "/sbin", "/usr", "/usr/bin", "/usr/sbin", "/usr/lib", "/usr/lib64",
		"/lib", "/lib64", "/etc", "/boot", "/dev", "/proc", "/sys", "/run",
		"/root", "/home", "/opt", "/var", "/usr/local", "/Users",
		"/usr/local/bin", "/usr/local/sbin", "/usr/local/lib", "/usr/share",
		"/lib32", "/libx32", "/usr/lib32", "/var/lib", "/var/log", "/var/cache",
		"/srv", "/mnt", "/media", "/snap", "/tmp",
	}
	if goos == "darwin" {
		// /etc, /var and /tmp are symlinks into /private on macOS; protect
		// the real directories too.
		paths = append(paths,
			"/System", "/Library", "/Applications", "/Volumes",
			"/private", "/private/etc", "/private/var", "/private/tmp",
			"/Library/LaunchDaemons", "/Library/LaunchAgents",
		)
	}
	return paths
}

func isWellKnownSystemDeletePath(cleaned, goos string) bool {
	for _, known := range wellKnownSystemDeletePaths(goos) {
		if sameDeletePath(cleaned, known, goos) {
			return true
		}
	}
	return false
}

func isProtectedDeletePath(cleaned string, extraExact []string, ancestorsOf []string) bool {
	return isProtectedDeletePathFor(cleaned, extraExact, ancestorsOf, runtime.GOOS)
}

func isProtectedDeletePathFor(cleaned string, extraExact []string, ancestorsOf []string, goos string) bool {
	if isVolumeRootDeletePath(cleaned, goos) {
		return true
	}
	if isWellKnownSystemDeletePath(cleaned, goos) {
		return true
	}
	for _, ancestorOf := range ancestorsOf {
		if strings.TrimSpace(ancestorOf) == "" {
			continue
		}
		if sameDeletePath(cleaned, ancestorOf, goos) || isStrictDeleteAncestor(cleaned, ancestorOf, goos) {
			return true
		}
	}
	for _, extra := range extraExact {
		if sameDeletePath(cleaned, extra, goos) {
			return true
		}
	}
	return false
}

func removePathEntry(cleaned string) error {
	info, err := os.Lstat(cleaned)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(cleaned)
	}
	if info.IsDir() {
		return os.RemoveAll(cleaned)
	}
	return os.Remove(cleaned)
}

func removePathEntryWithRetry(cleaned string) error {
	err := removePathEntry(cleaned)
	if err == nil {
		return nil
	}
	if clearPathReadOnlyIfNeeded(cleaned) {
		return removePathEntry(cleaned)
	}
	return err
}

func mapDeleteError(err error) string {
	if err == nil {
		return ""
	}
	if os.IsNotExist(err) {
		return "path not found"
	}
	if os.IsPermission(err) {
		return "permission denied"
	}
	return "unable to delete path: " + osErrorReason(err)
}

// osErrorReason strips the path from the OS error.
func osErrorReason(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.Err.Error()
	}
	return err.Error()
}

func parseDeletePaths(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("missing paths")
	}

	if strings.HasPrefix(raw, "[") {
		var paths []string
		if err := json.Unmarshal([]byte(raw), &paths); err != nil {
			return nil, fmt.Errorf("invalid paths")
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("missing paths")
		}
		out := make([]string, 0, len(paths))
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path == "" {
				return nil, fmt.Errorf("invalid paths")
			}
			out = append(out, path)
		}
		return out, nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("invalid paths")
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("missing paths")
	}
	return out, nil
}

func fileMkdir(rawParentPath, rawName string) (map[string]interface{}, error) {
	name := strings.TrimSpace(rawName)
	if err := validateFileBrowserName(name); err != nil {
		return nil, fmt.Errorf("invalid name")
	}

	parentPath, err := validateUploadDestinationPath(rawParentPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path")
	}

	parentInfo, err := os.Stat(parentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path not found")
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		return nil, fmt.Errorf("unable to access path")
	}
	if !parentInfo.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	newPath := filepath.Join(parentPath, name)
	if existing, err := os.Lstat(newPath); err == nil {
		if existing.IsDir() {
			return nil, fmt.Errorf(
				"This destination already contains a folder named %q", name,
			)
		}
		return nil, fmt.Errorf(
			"This destination already contains a file named %q", name,
		)
	} else if !os.IsNotExist(err) {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		return nil, fmt.Errorf("unable to access path")
	}

	if err := os.Mkdir(newPath, 0o755); err != nil {
		if os.IsExist(err) {
			if existing, statErr := os.Lstat(newPath); statErr == nil && !existing.IsDir() {
				return nil, fmt.Errorf(
					"This destination already contains a file named %q", name,
				)
			}
			return nil, fmt.Errorf(
				"This destination already contains a folder named %q", name,
			)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		return nil, fmt.Errorf("unable to create folder")
	}

	invalidateFileBrowserListingsFor(newPath)
	return fileProperties(newPath, nil)
}

func fileRename(rawPath, rawNewName string) (map[string]interface{}, error) {
	newName := strings.TrimSpace(rawNewName)
	if err := validateFileBrowserName(newName); err != nil {
		return nil, fmt.Errorf("invalid name")
	}

	cleaned, err := validateUploadDestinationPath(rawPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path")
	}

	oldName := entryNameFromPath(cleaned)
	if oldName == newName {
		return nil, fmt.Errorf("new name must differ")
	}

	oldInfo, err := os.Lstat(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path not found")
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		return nil, fmt.Errorf("unable to access path")
	}

	newPath := filepath.Join(filepath.Dir(cleaned), newName)
	if existing, err := os.Lstat(newPath); err == nil {
		if !os.SameFile(oldInfo, existing) {
			if existing.IsDir() {
				return nil, fmt.Errorf(
					"This destination already contains a folder named %q", newName,
				)
			}
			return nil, fmt.Errorf(
				"This destination already contains a file named %q", newName,
			)
		}
	} else if !os.IsNotExist(err) {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		return nil, fmt.Errorf("unable to access path")
	}

	if err := os.Rename(cleaned, newPath); err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied")
		}
		if os.IsExist(err) {
			if existing, statErr := os.Lstat(newPath); statErr == nil && existing.IsDir() {
				return nil, fmt.Errorf(
					"This destination already contains a folder named %q", newName,
				)
			}
			return nil, fmt.Errorf(
				"This destination already contains a file named %q", newName,
			)
		}
		return nil, fmt.Errorf("unable to rename path: %s", osErrorReason(err))
	}

	invalidateFileBrowserListingsFor(cleaned, newPath)
	return fileProperties(newPath, nil)
}

func fileDelete(rawPaths []string, extraExact []string, ancestorsOf []string) (map[string]interface{}, error) {
	if len(rawPaths) == 0 {
		return nil, fmt.Errorf("missing paths")
	}

	results := make([]map[string]interface{}, 0, len(rawPaths))
	for _, rawPath := range rawPaths {
		result := map[string]interface{}{
			"path": strings.TrimSpace(rawPath),
		}

		cleaned, err := validateUploadDestinationPath(rawPath)
		if err != nil {
			result["success"] = false
			result["error"] = "invalid path"
			results = append(results, result)
			continue
		}
		result["path"] = cleaned

		if isProtectedDeletePath(cleaned, extraExact, ancestorsOf) {
			result["success"] = false
			result["error"] = "protected path"
			results = append(results, result)
			continue
		}

		if err := removePathEntryWithRetry(cleaned); err != nil {
			result["success"] = false
			result["error"] = mapDeleteError(err)
			results = append(results, result)
			continue
		}

		invalidateFileBrowserListingsFor(cleaned)
		result["success"] = true
		results = append(results, result)
	}

	return map[string]interface{}{"results": results}, nil
}
