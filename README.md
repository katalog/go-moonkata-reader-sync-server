# moonkata-sync-server

**[English](README.md) | [한국어](README.ko.md)**

A small Windows tray app (plain Go, no framework) that shares a folder over HTTPS on your LAN so the [android-moonkata-reader](https://github.com/katalog/android-moonkata-reader) app can pull book files from your PC — no cloud storage, no account. Part of the [moonkata-reader-project](https://github.com/katalog/moonkata-reader-project) umbrella.

## What it does

- Shares one folder you pick, over HTTPS, authenticated by a secret it generates on first run
- The Android app mirrors that folder into its library one-way (PC → phone) with a "Sync now" button
- Pairing is one QR scan away — the tray menu's "동기화 QR 보기" opens a local page with a QR containing host + secret + TLS fingerprint in one shot, or you can copy/paste the secret manually
- The self-signed TLS certificate is trust-pinned SSH-style (trust-on-first-use) rather than CA-verified, since private LAN IPs can't get a real certificate
- All tray notifications are non-blocking Windows toasts — the server never sits waiting on a modal dialog
- Single-instance guarded (a named Windows mutex) so launching the exe twice doesn't spin up two competing servers

## Endpoints

| Route | Purpose |
|---|---|
| `/ping` | Identify this as a moonkata-sync-server instance (used for LAN discovery) |
| `/list` | Recursive file listing of the shared folder (requires the secret header) |
| `/file?path=...` | Stream one file's bytes (requires the secret header) |
| `/pair` | HTML page with a QR pairing code — no auth required, since the QR itself is the credential |

## Build & run

```bash
go build ./...
```

No CGO, no external runtime — the build output is a single `.exe`. Requires Windows to actually run (uses `NotifyIcon`/`FolderBrowserDialog` via PowerShell, and Windows-only autostart/mutex APIs), though `go build`/`go vet`/`go test` all run fine cross-platform.

Prebuilt Windows executables are published automatically on the [Releases](../../releases) page whenever a `vX.Y.Z` tag is pushed.

## Tests

```bash
go test ./...
```

Covers path-traversal prevention in the folder listing/file-serving logic, and the `/pair` QR payload's host field format (regression test for a bug where the port ended up duplicated).

## License

[Apache License 2.0](LICENSE)
