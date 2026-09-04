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

// QR pairing (.docs/SYNC_MULTIUSER_PLAN.md stage 6) — scanning this single QR code lets Android skip
// both host discovery (the "Find PC" subnet scan) and manually entering the secret, and since the
// fingerprint is also delivered up front via the QR code, it can connect straight away with pinned
// TLS instead of first going through a lenient-TLS TOFU step.

type pairPayload struct {
	Type        string `json:"type"`
	Host        string `json:"host"`
	Secret      string `json:"secret"`
	Fingerprint string `json:"fingerprint"`
}

// handlePair responds to /pair requests — no authentication is required. The secret itself is
// already inside this response, so there's nothing extra to protect here (this information only
// matters within the same LAN anyway), and requiring the secret up front the way /list and /file do
// would create a chicken-and-egg problem: needing the secret to view the QR code that's meant to hand
// out the secret.
func handlePair(w http.ResponseWriter, r *http.Request, state *AppState, fingerprint string) {
	_, secret := state.Get()
	host, err := localLanIP()
	if err != nil {
		http.Error(w, "Could not determine the local IP address — check your network connection.", http.StatusInternalServerError)
		return
	}

	// Host carries only the IP, with no port — Android's PcSyncClient already appends its own fixed
	// port (PC_SYNC_PORT) (it's designed to only accept a bare IP in the host field), so sending the
	// port here too would double up into "IP:port:port" and throw a MalformedURLException — a bug we
	// actually hit in real use.
	payload := pairPayload{
		Type:        "pc_sync",
		Host:        host,
		Secret:      secret,
		Fingerprint: fingerprint,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to build payload", http.StatusInternalServerError)
		return
	}

	png, err := qrcode.Encode(string(payloadJSON), qrcode.Medium, 280)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}
	// html/template runs a separate sanitizing filter on anywhere a URL goes, like <img src>, and
	// that filter doesn't trust data: URIs — passing the string in as-is gets it stripped out
	// entirely at render time (empty src, just a broken-image icon — found in real use). Wrapping it
	// in template.URL marks it as "this value has already been validated as a URL," so it bypasses
	// the filter — safe here because we constructed the value ourselves.
	qrDataURI := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pairPageTemplate.Execute(w, pairPageData{
		QrDataURI: qrDataURI,
		Host:      payload.Host,
		Secret:    secret,
	})
}

// localLanIP picks one IPv4 address to represent this PC. Simply walking net.InterfaceAddrs() in
// order can pick up a link-local address (169.254.0.0/16) from a VPN/virtual adapter/failed DHCP
// before the real one (found in real use — 169.254.x.x ended up in the QR code instead of the PC's
// actual Wi-Fi/Ethernet address). Instead this uses the standard Go trick of opening a UDP socket
// (no packet is actually sent — it only consults the routing table) to ask which interface traffic
// headed to the internet would actually go out on — this correctly picks the real LAN IP in most
// cases. If that also fails (e.g. fully offline), it falls back to walking the interfaces directly,
// skipping loopback and link-local addresses.
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
	return "", errors.New("no usable local IPv4 address was found")
}

func outboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr.IP.IsUnspecified() {
		return "", errors.New("could not determine the outbound IP")
	}
	return localAddr.IP.String(), nil
}

type pairPageData struct {
	QrDataURI template.URL
	Host      string
	Secret    string
}

// html/template auto-escapes Host/Secret — both are values this program generates itself, so
// there's no real risk, but since they're rendered as HTML straight into the user's browser, they're
// handled safely here as a matter of habit.
var pairPageTemplate = template.Must(template.New("pair").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<title>moonkata-sync-server — Pairing QR code</title>
<style>
	body { font-family: sans-serif; text-align: center; padding: 32px 16px; background: #fafafa; color: #222; }
	.qr { background: white; display: inline-block; padding: 16px; border-radius: 8px; box-shadow: 0 1px 4px rgba(0,0,0,0.15); }
	code { user-select: all; word-break: break-all; background: #eee; padding: 4px 8px; border-radius: 4px; }
</style>
</head>
<body>
	<h2>Scan this from the Moonkata Reader app</h2>
	<p>Library screen → PC file sync → "Connect via QR"</p>
	<div class="qr"><img src="{{.QrDataURI}}" alt="Pairing QR code" width="280" height="280" /></div>
	<p>If you can't use a camera, enter these values manually instead:</p>
	<p>Address: <code>{{.Host}}</code></p>
	<p>Secret: <code>{{.Secret}}</code></p>
</body>
</html>`))
