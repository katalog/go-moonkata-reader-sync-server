# moonkata-sync-server

**[English](README.md) | [한국어](README.ko.md)**

PC에서 실행하는 작은 Windows 트레이 앱(순수 Go, 프레임워크 없음)입니다. 폴더 하나를 같은 Wi-Fi 안에서 HTTPS로 공유하면 [android-moonkata-reader](https://github.com/katalog/android-moonkata-reader) 앱이 그 안의 책 파일을 받아갑니다 — 클라우드 저장소도, 계정도 필요 없습니다. [moonkata-reader-project](https://github.com/katalog/moonkata-reader-project) 우산 프로젝트의 한 부분입니다.

## 뭘 하나

- 선택한 폴더 하나를 HTTPS로 공유, 첫 실행 시 자동 생성되는 시크릿으로 인증
- 안드로이드 앱이 그 폴더를 라이브러리로 단방향(PC→폰) 미러링, "지금 동기화" 버튼 하나로 동작
- 페어링은 QR 스캔 한 번으로 끝 — 트레이 메뉴의 "동기화 QR 보기"가 호스트+시크릿+TLS 지문을 한 번에 담은 QR 페이지를 열어줌(수동 복사/붙여넣기도 가능)
- 사설 LAN IP는 정식 인증서를 받을 수 없어서, TLS는 CA 검증 대신 SSH 방식(TOFU, 최초 접속 때 지문을 저장해두고 이후엔 그 지문과 정확히 같은지만 확인)으로 고정
- 모든 트레이 알림은 논블로킹 Windows 토스트라, 확인을 눌러야 넘어가는 모달 창에 서버가 멈춰있는 일이 없음
- 이름 있는 Windows 뮤텍스로 중복 실행 방지 — exe를 두 번 실행해도 서버가 두 개 뜨지 않음

## 엔드포인트

| 경로 | 역할 |
|---|---|
| `/ping` | 이게 moonkata-sync-server 인스턴스인지 식별(LAN 탐색용) |
| `/list` | 공유 폴더 재귀 목록(시크릿 헤더 필요) |
| `/file?path=...` | 파일 하나를 바이트 스트리밍(시크릿 헤더 필요) |
| `/pair` | QR 페어링 코드가 담긴 HTML 페이지 — 인증 불필요(QR 자체가 곧 인증 정보) |

## 빌드 & 실행

```bash
go build ./...
```

CGO도, 별도 런타임도 필요 없이 exe 파일 하나로 나옵니다. 실제 실행은 Windows에서만 가능하지만(`NotifyIcon`/`FolderBrowserDialog`를 PowerShell로, 자동실행·중복실행방지는 Windows 전용 API 사용), `go build`/`go vet`/`go test`는 크로스플랫폼으로 문제없이 돌아갑니다.

`vX.Y.Z` 형식의 태그를 push하면 [Releases](../../releases) 페이지에 Windows 실행 파일이 자동으로 올라갑니다.

## 테스트

```bash
go test ./...
```

폴더 목록·파일 서빙 로직의 경로 탈출 방지, `/pair` QR 페이로드의 호스트 필드 형식(포트가 중복으로 붙던 버그의 회귀 테스트)을 검증합니다.

## 라이선스

[Apache License 2.0](LICENSE)
