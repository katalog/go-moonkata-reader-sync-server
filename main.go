package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
)

// .docs/PC_SYNC_SERVER_PLAN.md 참고. 기본은 트레이 앱(Phase P2)으로 뜨고, -headless를 주면 Phase P1
// 때처럼 콘솔에만 상태를 찍고 트레이 없이 블로킹 실행한다(테스트/디버깅용으로 남겨둠).
const port = 58221

func main() {
	folderFlag := flag.String("folder", "", "공유할 폴더 경로 (지정하면 설정 파일보다 우선)")
	secretFlag := flag.String("secret", "", "공유 시크릿 (지정하면 설정 파일보다 우선)")
	headless := flag.Bool("headless", false, "트레이 아이콘 없이 콘솔에서만 실행(테스트용)")
	flag.Parse()

	cfg, loadErr := loadConfig()

	folder := *folderFlag
	if folder == "" {
		folder = cfg.FolderPath
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
	}

	if loadErr != nil || cfg.FolderPath != folder || cfg.Secret != secret {
		if err := saveConfig(Config{FolderPath: folder, Secret: secret}); err != nil {
			log.Printf("설정 저장 실패(계속 진행): %v", err)
		}
	}

	state := newAppState(Config{FolderPath: folder, Secret: secret})

	cert, err := loadOrCreateTLSCertificate()
	if err != nil {
		log.Fatalf("TLS 인증서 준비 실패: %v", err)
	}

	go func() {
		handler := newServer(state)
		server := &http.Server{
			Addr:      fmt.Sprintf(":%d", port),
			Handler:   handler,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		}
		// 인증서/키는 위 TLSConfig에 이미 들어있어서 파일 경로 인자는 안 씀.
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Printf("서버 시작 실패(포트 %d 사용 중일 수 있음): %v", port, err)
			if !*headless {
				showMessage("moonkata-sync-server", fmt.Sprintf("서버를 시작하지 못했습니다 — 포트 %d를 다른 프로그램이 쓰고 있는지 확인하세요.", port))
			}
		}
	}()

	if *headless {
		if folder == "" {
			log.Fatal("공유할 폴더가 없습니다 — -folder 플래그로 지정하거나 설정 파일을 먼저 만드세요")
		}
		fmt.Printf("공유 폴더: %s\n", folder)
		fmt.Printf("공유 시크릿: %s\n", secret)
		fmt.Printf("포트 %d 에서 대기 중...\n", port)
		select {}
	}

	runTray(state)
}
