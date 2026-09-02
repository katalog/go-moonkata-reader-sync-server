package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// Config는 트레이 앱(Phase P2)이 아직 없는 지금은 %APPDATA%\MoonkataSyncServer\config.json에
// 직접 저장/로드된다. 공유 시크릿은 사용자가 직접 만들어 입력하는 게 아니라 이 프로그램이 처음 실행될
// 때 자동 생성해서 보여주는 방식(.docs/PC_SYNC_SERVER_PLAN.md §1) — 일반 사용자가 안전한 문자열을
// 직접 만들 필요가 없게 한다.
type Config struct {
	FolderPath string `json:"folderPath"`
	Secret     string `json:"secret"`
}

func configDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", os.ErrNotExist
	}
	dir := filepath.Join(appData, "MoonkataSyncServer")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, err
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func saveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func generateSecret() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
