package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// RemoteFile은 안드로이드 앱의 SmbRemoteFile/PcSyncRemoteFile과 같은 모양(relativePath/sizeBytes/
// lastModifiedMillis)으로 맞춘다 — .docs/PC_SYNC_SERVER_PLAN.md §2.
type RemoteFile struct {
	RelativePath       string `json:"relativePath"`
	SizeBytes          int64  `json:"sizeBytes"`
	LastModifiedMillis int64  `json:"lastModifiedMillis"`
}

// listFilesRecursively는 root 아래를 재귀적으로 순회해서 .txt/.zip 파일만 나열한다 —
// 안드로이드 라이브러리 화면이 인식하는 확장자와 동일(SafFolderBrowser.kt 참고).
func listFilesRecursively(root string) ([]RemoteFile, error) {
	var result []RemoteFile
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// root 자체를 못 읽으면(폴더가 통째로 없어짐/권한 없음) 진짜 실패다. 그 아래 항목 하나가
			// 순회 도중 사라지거나(안드로이드가 목록을 요청한 바로 그 순간 PC에서 파일/폴더를 옮기거나
			// 지우는 경합) 일시적으로 접근할 수 없는 경우는 전체 스캔을 실패시키지 않고 그 항목만
			// 건너뛴다 — 실사용 중 파일 삭제 직후 동기화가 "PC에 연결할 수 없습니다"로 통째로 실패하던
			// 문제(android-moonkata-reader .docs/IDEAS.md) 대응.
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
		// 점(.)으로 시작하는 폴더/파일은 건너뛴다 — 실기기 테스트 중 Syncthing의 내부 마커 파일
		// (.stfolder/*.txt)이 그대로 나열되는 걸 확인해서 추가한 필터. 폴더면 하위 전체를 스킵.
		// root 자체는 검사 대상에서 제외(경로 맨 끝 세그먼트가 우연히 "."로 시작해도 전체를 건너뛰면 안 됨).
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

// resolveFilePath는 요청받은 상대경로를 공유 루트 밖으로 못 나가게(".." 등으로 경로 탈출 방지) 검증하며
// 실제 로컬 경로로 변환한다.
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
