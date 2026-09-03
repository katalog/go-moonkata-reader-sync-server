package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"net"
	"net/http"

	qrcode "github.com/skip2/go-qrcode"
)

// QR 페어링(.docs/SYNC_MULTIUSER_PLAN.md 스테이지 6) — 안드로이드가 이 QR 하나만 스캔하면 호스트
// 찾기("PC 찾기" 서브넷 스캔)와 시크릿 입력을 둘 다 건너뛰고, 지문도 QR로 미리 받아서 lenient TLS로
// 먼저 접속해보는 TOFU 단계 없이 처음부터 pinned TLS로 바로 연결할 수 있다.

type pairPayload struct {
	Type        string `json:"type"`
	Host        string `json:"host"`
	Secret      string `json:"secret"`
	Fingerprint string `json:"fingerprint"`
}

// handlePair는 /pair 요청에 응답한다 — 인증이 필요 없다. 시크릿 자체가 이미 이 응답 안에 들어있어서
// 별도로 지킬 게 없고(같은 LAN 안에서만 의미 있는 정보), /list·/file처럼 매번 시크릿을 요구하면 QR을
// 보여주기 위한 QR을 보려고 시크릿이 먼저 필요해지는 모순이 생긴다.
func handlePair(w http.ResponseWriter, r *http.Request, state *AppState, fingerprint string) {
	_, secret := state.Get()
	host, err := localLanIP()
	if err != nil {
		http.Error(w, "로컬 IP를 찾지 못했습니다 — 네트워크 연결을 확인하세요.", http.StatusInternalServerError)
		return
	}

	// Host는 포트 없이 IP만 담는다 — 안드로이드의 PcSyncClient가 이미 고정 포트(PC_SYNC_PORT)를
	// 스스로 붙이는 구조라(호스트 입력칸에 IP만 받게 설계돼 있음), 여기서 포트까지 같이 보내면
	// "IP:포트:포트"로 겹쳐서 MalformedURLException이 난다 — 실사용 중 실제로 겪은 버그.
	payload := pairPayload{
		Type:        "pc_sync",
		Host:        host,
		Secret:      secret,
		Fingerprint: fingerprint,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "페이로드 생성 실패", http.StatusInternalServerError)
		return
	}

	png, err := qrcode.Encode(string(payloadJSON), qrcode.Medium, 280)
	if err != nil {
		http.Error(w, "QR 생성 실패", http.StatusInternalServerError)
		return
	}
	// html/template이 <img src>처럼 URL이 오는 자리는 별도 살균(sanitize) 필터를 거는데, 이 필터가
	// data: URI를 신뢰 안 해서 그냥 문자열로 넘기면 렌더링 시 통째로 지워진다(빈 src, 깨진 이미지
	// 아이콘만 뜸 — 실사용 중 발견). template.URL로 감싸면 "이 값은 이미 검증된 URL"이라고 표시돼
	// 필터를 안 탄다 — 우리가 직접 만든 값이라 안전.
	qrDataURI := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pairPageTemplate.Execute(w, pairPageData{
		QrDataURI: qrDataURI,
		Host:      payload.Host,
		Secret:    secret,
	})
}

// localLanIP는 이 PC를 가리킬 IPv4 주소 하나를 고른다. net.InterfaceAddrs()를 그냥 순서대로 훑으면
// VPN/가상 어댑터/DHCP 실패로 생긴 링크-로컬 주소(169.254.0.0/16)를 먼저 집을 수 있다(실사용 중
// 발견 — PC가 진짜 쓰는 Wi-Fi/이더넷 주소 대신 169.254.x.x가 QR에 실림). 대신 "실제로 인터넷 방향
// 트래픽이 나가는 인터페이스"를 UDP 소켓을 하나 만들어(실제로 패킷을 보내지 않음, 라우팅 테이블만
// 참조) 물어보는 표준 Go 트릭을 쓴다 — 이게 대부분의 경우 진짜 LAN IP를 정확히 골라준다. 완전히
// 오프라인이라 이것도 실패하면, 인터페이스를 직접 훑되 루프백/링크-로컬은 제외하는 방식으로 폴백한다.
func localLanIP() (string, error) {
	if ip, err := outboundIP(); err == nil {
		return ip, nil
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.IsLinkLocalUnicast() {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil {
			continue
		}
		return ip4.String(), nil
	}
	return "", errors.New("사용 가능한 로컬 IPv4 주소를 찾지 못했습니다")
}

func outboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr.IP.IsUnspecified() {
		return "", errors.New("아웃바운드 IP를 확인하지 못했습니다")
	}
	return localAddr.IP.String(), nil
}

type pairPageData struct {
	QrDataURI template.URL
	Host      string
	Secret    string
}

// html/template이 Host/Secret을 자동 이스케이프한다 — 둘 다 이 프로그램이 직접 만든 값이라 실질적
// 위험은 없지만, 이 값들이 그대로 사용자 브라우저에 렌더링되는 HTML이라 습관적으로 안전하게 처리한다.
var pairPageTemplate = template.Must(template.New("pair").Parse(`<!DOCTYPE html>
<html lang="ko">
<head>
<meta charset="UTF-8" />
<title>moonkata-sync-server — 페어링 QR</title>
<style>
	body { font-family: sans-serif; text-align: center; padding: 32px 16px; background: #fafafa; color: #222; }
	.qr { background: white; display: inline-block; padding: 16px; border-radius: 8px; box-shadow: 0 1px 4px rgba(0,0,0,0.15); }
	code { user-select: all; word-break: break-all; background: #eee; padding: 4px 8px; border-radius: 4px; }
</style>
</head>
<body>
	<h2>Moonkata Reader 앱에서 스캔하세요</h2>
	<p>서재 화면 → PC 파일 동기화 → "QR로 연결"</p>
	<div class="qr"><img src="{{.QrDataURI}}" alt="페어링 QR 코드" width="280" height="280" /></div>
	<p>카메라를 쓸 수 없다면 아래 값을 대신 직접 입력하세요:</p>
	<p>주소: <code>{{.Host}}</code></p>
	<p>시크릿: <code>{{.Secret}}</code></p>
</body>
</html>`))
