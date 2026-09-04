package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

// See .docs/PC_SYNC_SERVER_PLAN.md. By default this comes up as the tray app (Phase P2); passing
// -headless runs it the way Phase P1 did — printing status to the console only and blocking with no
// tray, kept around for testing/debugging.
const port = 58221

func main() {
	// Check this before anything else — if an instance is already running there's no reason to
	// re-read the config or try to open the port again (added from real-world feedback: running it
	// twice was confusing, e.g. two tray icons showing up). An actual port bind failure is already
	// handled (see the ListenAndServeTLS error handling below), but that only stops the server from
	// coming up — the tray UI would still duplicate, and this check is what prevents that.
	if !acquireSingleInstanceLock() {
		showNotification("moonkata-sync-server", "Already running — check the tray icon.")
		log.Println("Exiting because another instance is already running")
		os.Exit(1)
	}

	folderFlag := flag.String("folder", "", "Folder to share (overrides the config file if set)")
	secretFlag := flag.String("secret", "", "Shared secret (overrides the config file if set)")
	headless := flag.Bool("headless", false, "Run console-only with no tray icon (for testing)")
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
			log.Fatalf("Failed to generate secret: %v", err)
		}
		secret = generated
	}

	if loadErr != nil || cfg.FolderPath != folder || cfg.Secret != secret {
		if err := saveConfig(Config{FolderPath: folder, Secret: secret}); err != nil {
			log.Printf("Failed to save config (continuing anyway): %v", err)
		}
	}

	state := newAppState(Config{FolderPath: folder, Secret: secret})

	cert, err := loadOrCreateTLSCertificate()
	if err != nil {
		log.Fatalf("Failed to prepare TLS certificate: %v", err)
	}
	certFingerprint, err := certificateFingerprint(cert)
	if err != nil {
		log.Fatalf("Failed to compute certificate fingerprint: %v", err)
	}

	go func() {
		handler := newServer(state, certFingerprint)
		server := &http.Server{
			Addr:      fmt.Sprintf(":%d", port),
			Handler:   handler,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		}
		// The cert/key are already in the TLSConfig above, so no file path arguments are needed here.
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Printf("Failed to start server (port %d may already be in use): %v", port, err)
			if !*headless {
				showNotification("moonkata-sync-server", fmt.Sprintf("Failed to start the server — check whether another program is using port %d.", port))
			}
		}
	}()

	if *headless {
		if folder == "" {
			log.Fatal("No folder to share — pass the -folder flag or create a config file first")
		}
		fmt.Printf("Shared folder: %s\n", folder)
		fmt.Printf("Shared secret: %s\n", secret)
		fmt.Printf("Listening on port %d...\n", port)
		select {}
	}

	runTray(state)
}
