package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

const appName = "moonkata-sync-server"
const appVersion = "0.1.0"

type pingResponse struct {
	App     string `json:"app"`
	Version string `json:"version"`
}

// newServer는 .docs/PC_SYNC_SERVER_PLAN.md §2의 세 엔드포인트를 등록한 http.Handler를 만든다.
// folderPath/secret은 클로저로 캡처 — 트레이 앱(Phase P2)에서 설정이 바뀌면 이 함수를 다시 호출해서
// 새 핸들러로 갈아끼우면 된다(서버 재시작 없이 리로드하는 것도 나중에 고려 가능, 지금은 범위 밖).
func newServer(folderPath string, secret string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pingResponse{App: appName, Version: appVersion})
	})

	mux.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		if !checkSecret(w, r, secret) {
			return
		}
		files, err := listFilesRecursively(folderPath)
		if err != nil {
			http.Error(w, "folder read failed", http.StatusInternalServerError)
			log.Printf("list failed: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})

	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		if !checkSecret(w, r, secret) {
			return
		}
		relPath := r.URL.Query().Get("path")
		if relPath == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}
		fullPath, ok := resolveFilePath(folderPath, relPath)
		if !ok {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		f, err := os.Open(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
			} else {
				http.Error(w, "read failed", http.StatusInternalServerError)
			}
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := io.Copy(w, f); err != nil {
			log.Printf("file stream failed for %s: %v", relPath, err)
		}
	})

	return mux
}

func checkSecret(w http.ResponseWriter, r *http.Request, expected string) bool {
	got := r.Header.Get("x-moonkata-secret")
	if got == "" || got != expected {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}
