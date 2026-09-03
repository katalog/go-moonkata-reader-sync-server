package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 실사용 중 실제로 겪은 버그의 재발 방지 — payload.Host에 포트까지 포함해서("IP:포트") 내보냈는데,
// 안드로이드의 PcSyncClient가 고정 포트(PC_SYNC_PORT)를 스스로 또 붙이는 구조라 최종 주소가
// "IP:포트:포트"로 겹쳐 MalformedURLException이 났다(.docs/SYNC_MULTIUSER_PLAN.md 스테이지 6 참고).
// /pair 응답에 실제로 보이는 호스트 값에 콜론(포트 구분자)이 없는지 확인한다.
func TestHandlePair_HostFieldHasNoPort(t *testing.T) {
	state := newAppState(Config{FolderPath: t.TempDir(), Secret: "test-secret"})
	req := httptest.NewRequest(http.MethodGet, "/pair", nil)
	rec := httptest.NewRecorder()

	handlePair(rec, req, state, "AA:BB:CC:DD")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	const marker = "주소: <code>"
	start := strings.Index(body, marker)
	if start == -1 {
		t.Fatalf("응답 HTML에서 주소 표시를 찾지 못함:\n%s", body)
	}
	start += len(marker)
	end := strings.Index(body[start:], "</code>")
	if end == -1 {
		t.Fatal("주소 코드 블록이 안 닫힘")
	}
	host := body[start : start+end]

	if host == "" {
		t.Error("host가 비어있음")
	}
	if strings.Contains(host, ":") {
		t.Errorf("host에 포트가 같이 들어있으면 안 됨(PcSyncClient가 스스로 고정 포트를 붙임): %q", host)
	}
}
