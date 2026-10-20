//go:build windows

/*
Copyright 2026 Amidaware Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v3/disk"
)

func ListDirectoryWindows(a *Agent, rawPath string, page, pageSize int, nameFilter string) (map[string]interface{}, error) {
	return listDirectory(a, rawPath, page, pageSize, nameFilter)
}

func FileExistsWindows(rawDir string, names []string) (map[string]interface{}, error) {
	return fileExistsNames(rawDir, names)
}

func FilePropertiesWindows(rawPath string, limits folderSummaryLimits) (map[string]interface{}, error) {
	return fileProperties(rawPath, &limits)
}

func FileMkdirWindows(rawParentPath, rawName string) (map[string]interface{}, error) {
	return fileMkdir(rawParentPath, rawName)
}

func FileRenameWindows(rawPath, rawNewName string) (map[string]interface{}, error) {
	return fileRename(rawPath, rawNewName)
}

func FileDeleteWindows(a *Agent, rawPaths []string) (map[string]interface{}, error) {
	return a.FileDelete(rawPaths)
}

func defaultWindowsFileBrowserPathCandidates() []string {
	var candidates []string

	if public := strings.TrimSpace(os.Getenv("PUBLIC")); public != "" {
		candidates = append(candidates, public)
	}

	systemDrive := strings.TrimSpace(os.Getenv("SystemDrive"))
	if systemDrive == "" {
		systemDrive = "C:"
	}
	driveRoot := systemDrive
	if len(driveRoot) == 2 && driveRoot[1] == ':' {
		driveRoot += `\`
	} else if !strings.HasSuffix(driveRoot, `\`) {
		driveRoot += `\`
	}

	candidates = append(candidates,
		filepath.Join(driveRoot, "Users", "Public"),
		filepath.Join(driveRoot, "Users"),
		driveRoot,
	)
	candidates = append(candidates, windowsFixedDriveRoots()...)
	return candidates
}

func windowsFixedDriveRoots() []string {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil
	}

	roots := make([]string, 0, len(partitions))
	for _, p := range partitions {
		typepath, err := syscall.UTF16PtrFromString(p.Device)
		if err != nil {
			continue
		}
		typeval, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(typepath)))
		if typeval != 3 {
			continue
		}

		root := strings.TrimSpace(p.Mountpoint)
		if root == "" {
			continue
		}
		if len(root) == 2 && root[1] == ':' {
			root += `\`
		}
		roots = append(roots, root)
	}
	return roots
}

func clearPathReadOnlyIfNeeded(path string) bool {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}

	attrs, err := syscall.GetFileAttributes(pathPtr)
	if err != nil {
		return false
	}
	if attrs&syscall.FILE_ATTRIBUTE_READONLY == 0 {
		return false
	}

	newAttrs := attrs &^ syscall.FILE_ATTRIBUTE_READONLY
	return syscall.SetFileAttributes(pathPtr, newAttrs) == nil
}

func fileTimes(info os.FileInfo) (modified, created, accessed string) {
	modified = formatFileTime(info.ModTime())
	created = modified
	accessed = modified

	sys, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return modified, created, accessed
	}

	created = formatFileTime(time.Unix(0, sys.CreationTime.Nanoseconds()))
	accessed = formatFileTime(time.Unix(0, sys.LastAccessTime.Nanoseconds()))
	modified = formatFileTime(time.Unix(0, sys.LastWriteTime.Nanoseconds()))
	return modified, created, accessed
}

func fileAttributeFlags(_ string, info os.FileInfo, _ string) (hidden, system, readOnly bool) {
	sys, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		readOnly = info.Mode()&0200 == 0
		return false, false, readOnly
	}

	attrs := sys.FileAttributes
	hidden = attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0
	system = attrs&syscall.FILE_ATTRIBUTE_SYSTEM != 0
	readOnly = attrs&syscall.FILE_ATTRIBUTE_READONLY != 0
	return hidden, system, readOnly
}
