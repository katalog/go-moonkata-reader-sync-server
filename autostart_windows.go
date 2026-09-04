package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

// Auto-launch at Windows startup — uses only the per-user startup registry key, which needs no
// administrator privileges (HKCU\...\Run, applies only to the logged-in user account).
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
