package main

import "sync"

// AppState holds the settings (folder/secret) the server is currently using — so that changing
// the folder or regenerating the secret from the tray menu takes effect starting with the very
// next request, without restarting the HTTP listener, handlers read the value through this
// struct every time instead of capturing it directly.
type AppState struct {
	mu         sync.RWMutex
	folderPath string
	secret     string
}

func newAppState(cfg Config) *AppState {
	return &AppState{folderPath: cfg.FolderPath, secret: cfg.Secret}
}

func (s *AppState) Get() (folderPath string, secret string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.folderPath, s.secret
}

func (s *AppState) SetFolderPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folderPath = path
}

func (s *AppState) SetSecret(secret string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secret = secret
}
