package main

import (
	_ "embed"
	"fmt"
	"log"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var trayIcon []byte

// runTray는 프로그램의 메인 루프 — systray.Run이 블록되므로 HTTP 서버는 이미 별도 goroutine으로
// 떠 있어야 한다(main.go 참고). 메뉴 구성은 .docs/PC_SYNC_SERVER_PLAN.md §1 "PC 트레이 앱" 그대로:
// 폴더 변경, 시크릿 복사/재생성, 자동 실행 체크박스, 종료.
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
	mFolder := systray.AddMenuItem("공유 폴더 변경...", "동기화할 폴더를 다시 선택합니다")
	mCopySecret := systray.AddMenuItem("공유 시크릿 복사", "안드로이드 앱에 붙여넣을 시크릿을 클립보드로 복사합니다")
	mRegenSecret := systray.AddMenuItem("공유 시크릿 재생성", "기존 시크릿을 무효화하고 새로 만듭니다")
	mAutoStart := systray.AddMenuItemCheckbox("Windows 시작 시 자동 실행", "로그인할 때 자동으로 실행합니다", isAutoStartEnabled())
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("종료", "동기화 서버를 끕니다")

	// 시작할 때마다 시크릿을 보여준다 — 안드로이드 쪽 안내 문구("PC에서 moonkata-sync-server를
	// 실행하면 공유 시크릿이 표시됩니다")와 맞추기 위해 메뉴를 뒤지지 않아도 항상 바로 보이게.
	go showStartupSecret(state)

	go func() {
		for {
			select {
			case <-mFolder.ClickedCh:
				handleChangeFolder(state, mStatus)
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

func showStartupSecret(state *AppState) {
	folderPath, secret := state.Get()
	if folderPath == "" {
		showMessage("moonkata-sync-server", "공유할 폴더가 아직 설정되지 않았습니다.\n트레이 아이콘 메뉴에서 \"공유 폴더 변경...\"을 눌러 설정하세요.")
		return
	}
	_ = copyToClipboard(secret)
	showMessage(
		"moonkata-sync-server 실행 중",
		fmt.Sprintf("포트 %d에서 대기 중입니다.\n\n공유 폴더: %s\n\n공유 시크릿(클립보드에 복사됨):\n%s\n\n안드로이드 앱의 \"공유 시크릿\" 칸에 붙여넣으세요.", port, folderPath, secret),
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
		log.Printf("설정 저장 실패: %v", err)
	}
	updateStatusLabel(mStatus, state)
	updateTooltip(state)
	showMessage("moonkata-sync-server", "공유 폴더를 변경했습니다:\n"+selected)
}

func handleCopySecret(state *AppState) {
	_, secret := state.Get()
	if err := copyToClipboard(secret); err != nil {
		showMessage("moonkata-sync-server", "클립보드 복사에 실패했습니다.")
		return
	}
	showMessage("moonkata-sync-server", "공유 시크릿을 클립보드에 복사했습니다.")
}

func handleRegenerateSecret(state *AppState) {
	newSecret, err := generateSecret()
	if err != nil {
		showMessage("moonkata-sync-server", "시크릿 생성에 실패했습니다.")
		return
	}
	state.SetSecret(newSecret)
	if err := saveCurrentState(state); err != nil {
		log.Printf("설정 저장 실패: %v", err)
	}
	_ = copyToClipboard(newSecret)
	showMessage("moonkata-sync-server", "새 공유 시크릿을 만들어 클립보드에 복사했습니다 — 기존 시크릿을 쓰던 기기는 다시 붙여넣어야 합니다:\n\n"+newSecret)
}

func handleToggleAutoStart(item *systray.MenuItem) {
	enable := !isAutoStartEnabled()
	if err := setAutoStartEnabled(enable); err != nil {
		showMessage("moonkata-sync-server", "설정 변경에 실패했습니다.")
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
		item.SetTitle("공유 폴더 미설정")
		return
	}
	item.SetTitle("공유 중: " + folderPath)
}

func updateTooltip(state *AppState) {
	folderPath, _ := state.Get()
	if folderPath == "" {
		systray.SetTooltip("moonkata-sync-server — 폴더 미설정")
		return
	}
	systray.SetTooltip(fmt.Sprintf("moonkata-sync-server — 포트 %d\n%s", port, folderPath))
}

func saveCurrentState(state *AppState) error {
	folderPath, secret := state.Get()
	return saveConfig(Config{FolderPath: folderPath, Secret: secret})
}
