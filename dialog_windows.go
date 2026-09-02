package main

import (
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW   = user32.NewProc("MessageBoxW")
	mbIconInformation = 0x00000040
	mbOK              = 0x00000000
)

// showMessage는 트레이 앱 전용 알림창 — 별도 GUI 툴킷 없이 Win32 MessageBoxW를 syscall로 직접
// 호출한다(CGO 불필요, .docs/PC_SYNC_SERVER_PLAN.md의 "단일 네이티브 exe, 런타임 설치 불필요" 방침과
// 맞음). 이 호출 자체가 모달이라 호출한 goroutine만 블록되고 트레이/서버는 계속 동작한다.
func showMessage(title string, message string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), uintptr(mbIconInformation|mbOK))
}

// pickFolder는 Windows 기본 폴더 선택 대화상자를 띄운다 — 별도 다이얼로그 라이브러리(대부분 CGO 필요)
// 없이, 모든 Windows에 이미 있는 .NET WinForms를 PowerShell로 한 줄 실행해서 재사용한다. 취소하면 빈
// 문자열을 돌려준다.
func pickFolder(initialPath string) string {
	script := `Add-Type -AssemblyName System.Windows.Forms
$dlg = New-Object System.Windows.Forms.FolderBrowserDialog
$dlg.Description = "동기화할 폴더를 선택하세요"
if ("` + escapePowerShellString(initialPath) + `" -ne "") { $dlg.SelectedPath = "` + escapePowerShellString(initialPath) + `" }
if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $dlg.SelectedPath }`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func escapePowerShellString(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}

// copyToClipboard는 Windows 내장 clip.exe에 표준입력으로 흘려보내는 방식 — 별도 클립보드
// 라이브러리(대부분 CGO 필요) 없이 모든 Windows에 있는 도구를 재사용한다.
func copyToClipboard(text string) error {
	cmd := exec.Command("clip")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
