package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// RemoteFile mirrors the shape of the Android app's SmbRemoteFile/PcSyncRemoteFile
// (relativePath/sizeBytes/lastModifiedMillis) — .docs/PC_SYNC_SERVER_PLAN.md §2.
type RemoteFile struct {
	RelativePath       string `json:"relativePath"`
	SizeBytes          int64  `json:"sizeBytes"`
	LastModifiedMillis int64  `json:"lastModifiedMillis"`
}

// listFilesRecursively recursively walks root and lists only .txt/.zip files —
// the same extensions the Android library screen recognizes (see SafFolderBrowser.kt).
func listFilesRecursively(root string) ([]RemoteFile, error) {
	var result []RemoteFile
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// If root itself can't be read (the folder is gone entirely / no permission), that's a
			// real failure. If a single item under it disappears mid-walk (a race where the PC moves
			// or deletes a file/folder at the exact moment Android requests the listing) or is
			// temporarily inaccessible, we skip just that item instead of failing the whole scan —
			// this addresses a real issue where sync would fail entirely with "Can't connect to PC"
			// right after deleting a file (android-moonkata-reader .docs/IDEAS.md).
			if path == root {
				return err
			}
			log.Printf("list: skipping %s: %v", path, err)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		// Skip folders/files starting with a dot (.) — added after observing Syncthing's internal
		// marker files (.stfolder/*.txt) showing up in the listing during real-device testing. If
		// it's a folder, skip everything under it. root itself is exempt from this check (even if
		// its final path segment happens to start with ".", the whole scan shouldn't be skipped).
		if path != root && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".txt") && !strings.HasSuffix(lower, ".zip") {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		result = append(result, RemoteFile{
			RelativePath:       filepath.ToSlash(relPath),
			SizeBytes:          info.Size(),
			LastModifiedMillis: info.ModTime().UnixMilli(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []RemoteFile{}
	}
	return result, nil
}

// resolveFilePath validates that the requested relative path can't escape the shared root
// (prevents path traversal via ".." etc.) and converts it into an actual local path.
func resolveFilePath(root string, relativePath string) (string, bool) {
	cleaned := filepath.Clean(filepath.FromSlash(relativePath))
	if cleaned == "." || strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return "", false
	}
	full := filepath.Join(root, cleaned)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return "", false
	}
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs, rootAbs+string(os.PathSeparator)) {
		return "", false
	}
	return fullAbs, true
}
