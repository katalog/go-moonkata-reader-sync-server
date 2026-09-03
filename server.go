package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

const appName = "moonkata-sync-server"
const appVersion = "1.5.0-beta.1"

type pingResponse struct {
	App     string `json:"app"`
	Version string `json:"version"`
}

// newServer는 .docs/PC_SYNC_SERVER_PLAN.md §2의 세 엔드포인트 + QR 페어링용 /pair
// (.docs/SYNC_MULTIUSER_PLAN.md 스테이지 6)를 등록한 http.Handler를 만든다. 폴더/시크릿은 고정값이
// 아니라 [AppState]에서 매 요청마다 읽는다 — 트레이 메뉴로 설정을 바꿔도 HTTP 리스너를 재시작할
// 필요가 없다. certFingerprint는 시작할 때 로드한 인증서에서 한 번만 계산해 넘긴다(재실행 전까지
// 안 바뀌므로).
func newServer(state *AppState, certFingerprint string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pingResponse{App: appName, Version: appVersion})
	})

	mux.HandleFunc("/pair", func(w http.ResponseWriter, r *http.Request) {
		handlePair(w, r, state, certFingerprint)
	})

	mux.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		folderPath, secret := state.Get()
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
		folderPath, secret := state.Get()
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
