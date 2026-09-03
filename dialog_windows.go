package main

import (
	"os/exec"
	"strings"
	"syscall"
)

// showNotification은 화면 우측 하단에 뜨는 알림(토스트/풍선 도움말)을 띄운다 — 원래 있던
// showMessage(Win32 MessageBoxW 기반 모달)는 확인을 누를 때까지 프로그램이 멈춰있는 것처럼 보이는
// 문제가 있어서(실사용 피드백으로, 트레이 앱의 모든 알림을 이걸로 교체) 완전히 걷어냈다. .NET
// WinForms의 NotifyIcon을 PowerShell로
// 한 줄 실행하는 방식 — Windows 10+에서는 이게 자동으로 현대적인 우측 하단 토스트로 뜬다. 별도
// 프로세스를 기다리지 않고 바로 리턴한다(cmd.Start, cmd.Run이 아님) — 알림이 몇 초 떠 있는 동안 이
// 함수를 호출한 goroutine이 멈춰있을 이유가 없다. PowerShell 스크립트 자신이 잠깐 살아있다가
// 스스로 정리한다.
func showNotification(title string, message string) {
	script := `Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$notify = New-Object System.Windows.Forms.NotifyIcon
$notify.Icon = [System.Drawing.SystemIcons]::Information
$notify.Visible = $true
$notify.BalloonTipTitle = "` + escapePowerShellString(title) + `"
$notify.BalloonTipText = "` + escapePowerShellString(message) + `"
$notify.ShowBalloonTip(10000)
Start-Sleep -Seconds 10
$notify.Dispose()`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
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

// openURL은 기본 브라우저로 주소를 연다 — "동기화 QR 보기" 메뉴용(.docs/SYNC_MULTIUSER_PLAN.md
// 스테이지 6). `cmd /c start`는 Windows에 항상 있는 내장 명령이라 별도 라이브러리가 필요 없다. 첫
// 인자는 start가 창 제목으로 해석하므로 빈 문자열을 그 자리에 넣어야 URL이 두 번째 인자로 정확히
// 전달된다.
func openURL(url string) error {
	cmd := exec.Command("cmd", "/c", "start", "", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
