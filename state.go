package main

import "sync"

// AppState는 지금 서버가 쓰고 있는 설정(폴더/시크릿)을 들고 있다 — 트레이 메뉴에서 폴더를 바꾸거나
// 시크릿을 재생성해도 HTTP 리스너를 재시작할 필요 없이 다음 요청부터 바로 반영되게 하기 위해 핸들러가
// 값을 직접 캡처하는 대신 이 구조체를 통해 매번 읽는다.
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
