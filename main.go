package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

// Phase P1 — 트레이 UI(Phase P2) 없이 커맨드라인으로 서버 코어만 먼저 검증한다
// (.docs/PC_SYNC_SERVER_PLAN.md 참고). -folder/-secret을 안 주면 %APPDATA%의 설정 파일을 쓰고,
// 그 파일도 없으면 시크릿을 새로 생성해서 저장 + 콘솔에 보여준다.
const port = 58221

func main() {
	folderFlag := flag.String("folder", "", "공유할 폴더 경로 (지정하면 설정 파일보다 우선)")
	secretFlag := flag.String("secret", "", "공유 시크릿 (지정하면 설정 파일보다 우선)")
	flag.Parse()

	cfg, loadErr := loadConfig()

	folder := *folderFlag
	if folder == "" {
		folder = cfg.FolderPath
	}
	if folder == "" {
		log.Fatal("공유할 폴더가 없습니다 — -folder 플래그로 지정하거나 설정 파일을 먼저 만드세요")
	}

	secret := *secretFlag
	if secret == "" {
		secret = cfg.Secret
	}
	if secret == "" {
		generated, err := generateSecret()
		if err != nil {
			log.Fatalf("시크릿 생성 실패: %v", err)
		}
		secret = generated
		fmt.Printf("새 공유 시크릿을 생성했습니다: %s\n", secret)
	}

	if loadErr != nil || cfg.FolderPath != folder || cfg.Secret != secret {
		if err := saveConfig(Config{FolderPath: folder, Secret: secret}); err != nil {
			log.Printf("설정 저장 실패(계속 진행): %v", err)
		}
	}

	fmt.Printf("공유 폴더: %s\n", folder)
	fmt.Printf("공유 시크릿: %s\n", secret)
	fmt.Printf("포트 %d 에서 대기 중...\n", port)

	handler := newServer(folder, secret)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), handler); err != nil {
		log.Fatalf("서버 시작 실패: %v", err)
	}
}
