package main

import (
	_ "embed"
	"fmt"
	"log"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var trayIcon []byte

// runTray is the program's main loop — systray.Run blocks, so the HTTP server must already be
// running on its own goroutine by this point (see main.go). The menu layout follows
// .docs/PC_SYNC_SERVER_PLAN.md §1 "PC tray app" as-is: change folder, copy/regenerate secret,
// auto-start checkbox, quit.
func runTray(state *AppState) {
	systray.Run(func() { onTrayReady(state) }, func() {})
}

func onTrayReady(state *AppState) {
	systray.SetIcon(trayIcon)
	systray.SetTitle("")
	updateTooltip(state)

	mStatus := systray.AddMenuItem("", "")
	mStatus.Disable()
	updateStatusLabel(mStatus, state)

	systray.AddSeparator()
	mFolder := systray.AddMenuItem("Change shared folder...", "Choose a different folder to sync")
	mPairingQr := systray.AddMenuItem("Show sync QR code", "Scan with the Android app to connect instantly")
	mCopySecret := systray.AddMenuItem("Copy shared secret", "Copy the secret to paste into the Android app")
	mRegenSecret := systray.AddMenuItem("Regenerate shared secret", "Invalidates the current secret and creates a new one")
	mAutoStart := systray.AddMenuItemCheckbox("Start automatically with Windows", "Run automatically when you sign in", isAutoStartEnabled())
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop the sync server")

	// Show the secret every time the app starts — this keeps it aligned with the Android side's
	// guidance text ("Running moonkata-sync-server on your PC will show the shared secret"), so it's
	// always right there without needing to dig through the menu.
	go showStartupSecret(state)

	go func() {
		for {
			select {
			case <-mFolder.ClickedCh:
				handleChangeFolder(state, mStatus)
			case <-mPairingQr.ClickedCh:
				handleShowPairingQr()
			case <-mCopySecret.ClickedCh:
				handleCopySecret(state)
			case <-mRegenSecret.ClickedCh:
				handleRegenerateSecret(state)
			case <-mAutoStart.ClickedCh:
				handleToggleAutoStart(mAutoStart)
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// Report the status every time the app starts — this used to be showMessage (a modal that blocked
// until you clicked OK), but that was just informational, and real-world feedback said there was no
// reason to force a click, so it was switched to showNotification (a bottom-right toast). All the
// other notifications (folder change, secret copy/regenerate, QR guidance, failure messages, etc.)
// were unified onto showNotification for the same reason afterward — so there isn't a single modal
// left that "freezes" the app while it's actually working fine. The wording was also trimmed once QR
// pairing (stage 6) arrived, since there's no longer a need to show the full secret up front — it's
// still copied to the clipboard, and the full value is always available again from the "Copy shared
// secret" menu item if needed.
func showStartupSecret(state *AppState) {
	folderPath, secret := state.Get()
	if folderPath == "" {
		showNotification("moonkata-sync-server", "No shared folder is set yet — choose \"Change shared folder...\" from the tray menu.")
		return
	}
	_ = copyToClipboard(secret)
	showNotification(
		"moonkata-sync-server is running",
		fmt.Sprintf("Listening on port %d. Use \"Show sync QR code\" from the tray menu to connect with Android (the secret has been copied to the clipboard).", port),
	)
}

func handleChangeFolder(state *AppState, mStatus *systray.MenuItem) {
	current, _ := state.Get()
	selected := pickFolder(current)
	if selected == "" {
		return
	}
	state.SetFolderPath(selected)
	if err := saveCurrentState(state); err != nil {
		log.Printf("Failed to save config: %v", err)
	}
	updateStatusLabel(mStatus, state)
	updateTooltip(state)
	showNotification("moonkata-sync-server", "Shared folder changed to:\n"+selected)
}

// handleShowPairingQr opens the /pair page in the default browser — because this server uses a
// self-signed certificate (TOFU, see tls.go), the browser will show an "unsafe connection" warning
// the first time. That's inherent to the certificate approach this server uses, so this warning
// screen can't be removed on its own; instead the user is told why beforehand and allowed to proceed
// (.docs/SYNC_MULTIUSER_PLAN.md stage 6).
func handleShowPairingQr() {
	showNotification(
		"moonkata-sync-server",
		"Opening your browser. This server uses a self-signed certificate, so you may see an \"unsafe connection\" warning — click \"Advanced\" then \"Proceed\" to see the QR code (only needed once).",
	)
	if err := openURL(fmt.Sprintf("https://127.0.0.1:%d/pair", port)); err != nil {
		showNotification("moonkata-sync-server", "Failed to open the browser.")
	}
}

func handleCopySecret(state *AppState) {
	_, secret := state.Get()
	if err := copyToClipboard(secret); err != nil {
		showNotification("moonkata-sync-server", "Failed to copy to the clipboard.")
		return
	}
	showNotification("moonkata-sync-server", "Copied the shared secret to the clipboard.")
}

func handleRegenerateSecret(state *AppState) {
	newSecret, err := generateSecret()
	if err != nil {
		showNotification("moonkata-sync-server", "Failed to generate a secret.")
		return
	}
	state.SetSecret(newSecret)
	if err := saveCurrentState(state); err != nil {
		log.Printf("Failed to save config: %v", err)
	}
	_ = copyToClipboard(newSecret)
	showNotification("moonkata-sync-server", "Created a new shared secret and copied it to the clipboard — devices using the old secret will need to paste the new one:\n\n"+newSecret)
}

func handleToggleAutoStart(item *systray.MenuItem) {
	enable := !isAutoStartEnabled()
	if err := setAutoStartEnabled(enable); err != nil {
		showNotification("moonkata-sync-server", "Failed to change the setting.")
		return
	}
	if enable {
		item.Check()
	} else {
		item.Uncheck()
	}
}

func updateStatusLabel(item *systray.MenuItem, state *AppState) {
	folderPath, _ := state.Get()
	if folderPath == "" {
		item.SetTitle("No shared folder set")
		return
	}
	item.SetTitle("Sharing: " + folderPath)
}

func updateTooltip(state *AppState) {
	folderPath, _ := state.Get()
	if folderPath == "" {
		systray.SetTooltip("moonkata-sync-server — no folder set")
		return
	}
	systray.SetTooltip(fmt.Sprintf("moonkata-sync-server — port %d\n%s", port, folderPath))
}

func saveCurrentState(state *AppState) error {
	folderPath, secret := state.Get()
	return saveConfig(Config{FolderPath: folderPath, Secret: secret})
}
