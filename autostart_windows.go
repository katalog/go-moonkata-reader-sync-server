package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

// Windows 시작 시 자동 실행 — 관리자 권한이 필요 없는 사용자별 시작프로그램 레지스트리 키만 쓴다
// (HKCU\...\Run, 로그인한 사용자 계정에만 적용).
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const runValueName = "MoonkataSyncServer"

func isAutoStartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	_, _, err = key.GetStringValue(runValueName)
	return err == nil
}

func setAutoStartEnabled(enabled bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !enabled {
		return key.DeleteValue(runValueName)
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	return key.SetStringValue(runValueName, `"`+exePath+`"`)
}
