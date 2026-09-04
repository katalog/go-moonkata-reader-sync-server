package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

const appName = "moonkata-sync-server"
const appVersion = "1.5.1"

type pingResponse struct {
	App     string `json:"app"`
	Version string `json:"version"`
}

// newServer builds an http.Handler with the three endpoints from .docs/PC_SYNC_SERVER_PLAN.md §2
// plus /pair for QR pairing (.docs/SYNC_MULTIUSER_PLAN.md stage 6) registered. The folder/secret
// aren't fixed values — they're read from [AppState] on every request, so changing settings from
// the tray menu never requires restarting the HTTP listener. certFingerprint is computed once from
// the certificate loaded at startup and passed in (it doesn't change until the next restart).
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
