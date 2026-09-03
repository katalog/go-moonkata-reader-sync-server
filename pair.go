package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

	payload := pairPayload{
		Type:        "pc_sync",
		Host:        fmt.Sprintf("%s:%d", host, port),
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
	qrDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pairPageTemplate.Execute(w, pairPageData{
		QrDataURI: qrDataURI,
		Host:      payload.Host,
		Secret:    secret,
	})
}

// localLanIP는 로컬 서브넷에서 이 PC를 가리킬 IPv4 주소 하나를 고른다 — 루프백은 QR을 스캔하는
// 안드로이드 기기 입장에서 아무 의미가 없으므로 제외한다. 인터페이스가 여러 개(VPN, 가상 어댑터 등)면
// 그중 처음 나오는 유효한 사설 IPv4를 쓴다 — 대부분의 가정용 환경에선 문제없지만, 여러 개의 실제 LAN에
// 동시에 연결된 드문 구성에서는 사용자가 QR 대신 수동 입력으로 정확한 주소를 골라야 할 수 있다.
func localLanIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
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

type pairPageData struct {
	QrDataURI string
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
