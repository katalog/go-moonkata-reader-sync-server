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
	mPairingQr := systray.AddMenuItem("동기화 QR 보기", "안드로이드 앱으로 스캔해서 바로 연결합니다")
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

// 실행될 때마다 상태를 알려준다 — 예전엔 showMessage(모달, OK를 눌러야 다음으로 넘어감)였는데, 그냥
// 확인용 정보라 클릭을 강제할 이유가 없다는 실사용 피드백을 받고 showNotification(우측 하단 알림)으로
// 바꿨다. 안내 문구도 QR 페어링(스테이지 6)이 생긴 뒤로는 시크릿을 통째로 안 보여줘도 되므로 줄였다 —
// 시크릿은 여전히 클립보드에 복사해두고, 자세한 값이 필요하면 "공유 시크릿 복사" 메뉴로 언제든 다시
// 가져갈 수 있다.
func showStartupSecret(state *AppState) {
	folderPath, secret := state.Get()
	if folderPath == "" {
		showNotification("moonkata-sync-server", "공유할 폴더가 아직 설정되지 않았습니다 — 트레이 메뉴에서 \"공유 폴더 변경...\"을 선택하세요.")
		return
	}
	_ = copyToClipboard(secret)
	showNotification(
		"moonkata-sync-server 실행 중",
		fmt.Sprintf("포트 %d에서 대기 중입니다. 트레이 메뉴의 \"동기화 QR 보기\"로 안드로이드와 연결하세요(시크릿은 클립보드에 복사됨).", port),
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

// handleShowPairingQr는 기본 브라우저로 /pair 페이지를 연다 — 이 서버가 자체 서명 인증서를 쓰기
// 때문에(TOFU, tls.go 참고) 브라우저가 처음엔 "안전하지 않은 연결" 경고를 보여준다. 이건 이 서버가
// 쓰는 인증서 방식 자체의 성격이라 이 화면만 따로 없앨 방법은 없어서, 사용자에게 왜 그런지 미리
// 알려주고 진행하게 한다(.docs/SYNC_MULTIUSER_PLAN.md 스테이지 6).
func handleShowPairingQr() {
	showMessage(
		"moonkata-sync-server",
		"브라우저가 열립니다. 이 서버는 자체 서명 인증서를 쓰기 때문에 \"안전하지 않은 연결\" 경고가 뜰 수 있습니다 — \"고급\" → \"계속 진행\"을 누르면 QR이 보입니다(처음 한 번만).",
	)
	if err := openURL(fmt.Sprintf("https://127.0.0.1:%d/pair", port)); err != nil {
		showMessage("moonkata-sync-server", "브라우저를 여는 데 실패했습니다.")
	}
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
