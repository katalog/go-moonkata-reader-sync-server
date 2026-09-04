package main

import (
	"os/exec"
	"strings"
	"syscall"
)

// showNotification pops up a bottom-right notification (toast/balloon tip) — the original
// showMessage (a Win32 MessageBoxW-based modal) had the problem of looking like the program had
// frozen until you clicked OK, so based on real-world feedback it was replaced everywhere across the
// tray app and fully removed. This runs .NET WinForms' NotifyIcon as a one-line PowerShell script —
// on Windows 10+ that automatically shows up as a modern bottom-right toast. It returns immediately
// without waiting for the child process (cmd.Start, not cmd.Run) — there's no reason for the calling
// goroutine to block while the notification stays up for a few seconds. The PowerShell script stays
// alive briefly on its own and cleans itself up.
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

// pickFolder shows the standard Windows folder-picker dialog — instead of pulling in a separate
// dialog library (most of which need CGO), it reuses .NET WinForms, which is already present on
// every Windows install, via a one-line PowerShell script. Returns an empty string if cancelled.
func pickFolder(initialPath string) string {
	script := `Add-Type -AssemblyName System.Windows.Forms
$dlg = New-Object System.Windows.Forms.FolderBrowserDialog
$dlg.Description = "Choose a folder to sync"
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

// copyToClipboard pipes the text into Windows' built-in clip.exe via stdin — instead of pulling in a
// separate clipboard library (most of which need CGO), it reuses a tool already present on every
// Windows install.
func copyToClipboard(text string) error {
	cmd := exec.Command("clip")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// openURL opens an address in the default browser — used by the "Show sync QR code" menu item
// (.docs/SYNC_MULTIUSER_PLAN.md stage 6). `cmd /c start` is a built-in command always present on
// Windows, so no separate library is needed. The first argument has to be an empty string because
// start interprets that position as the window title — otherwise the URL wouldn't be passed
// correctly as the second argument.
func openURL(url string) error {
	cmd := exec.Command("cmd", "/c", "start", "", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
