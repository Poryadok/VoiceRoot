// node-media is the node-local Voice token edge. It never holds master private
// signing keys or accepts master user refresh/access tokens.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"voice/backend/federation/mediaauthority"
)

type configuration struct {
	ListenAddress        string `json:"listen_address"`
	TrustFile            string `json:"trust_file"`
	AuthorityDirectory   string `json:"authority_directory"`
	BootRequestDirectory string `json:"boot_request_directory"`
	CredentialsFile      string `json:"credentials_file"`
	TLSCertFile          string `json:"tls_cert_file"`
	TLSKeyFile           string `json:"tls_key_file"`
	MediaURL             string `json:"media_url"`
}
type trustConfiguration struct {
	Issuer      string            `json:"issuer"`
	Environment string            `json:"environment"`
	NodeID      string            `json:"node_id"`
	Keys        map[string]string `json:"keys"`
}
type credentials struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

func readJSON(path string, value any) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return mediaauthority.ErrDenied
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return mediaauthority.ErrDenied
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil || d.Decode(new(any)) != io.EOF {
		return mediaauthority.ErrDenied
	}
	return nil
}
func load(path string) (*http.Server, *mediaauthority.Registry, string, *mediaauthority.BootHeartbeat, error) {
	var config configuration
	var trust trustConfiguration
	var creds credentials
	if readJSON(path, &config) != nil || readJSON(config.TrustFile, &trust) != nil || readJSON(config.CredentialsFile, &creds) != nil {
		return nil, nil, "", nil, mediaauthority.ErrDenied
	}
	_, port, err := net.SplitHostPort(config.ListenAddress)
	if err != nil || port == "" {
		return nil, nil, "", nil, mediaauthority.ErrDenied
	}
	info, err := os.Stat(config.AuthorityDirectory)
	if err != nil || !info.IsDir() {
		return nil, nil, "", nil, mediaauthority.ErrDenied
	}
	keys := map[string]ed25519.PublicKey{}
	for id, value := range trust.Keys {
		key, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return nil, nil, "", nil, mediaauthority.ErrDenied
		}
		keys[id] = key
	}
	registry, err := mediaauthority.NewBootRegistry(mediaauthority.Verifier{Issuer: trust.Issuer, Environment: trust.Environment, NodeID: trust.NodeID, Keys: keys}, 250*time.Millisecond)
	if err != nil {
		return nil, nil, "", nil, err
	}
	heartbeat, err := mediaauthority.NewBootHeartbeat(registry, config.BootRequestDirectory)
	if err != nil {
		return nil, nil, "", nil, err
	}
	exchange, err := mediaauthority.NewExchange(registry, creds.APIKey, creds.APISecret, config.MediaURL)
	if err != nil {
		return nil, nil, "", nil, err
	}
	certificate, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
	if err != nil {
		return nil, nil, "", nil, mediaauthority.ErrDenied
	}
	return &http.Server{Addr: config.ListenAddress, Handler: exchange, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}}, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}, registry, config.AuthorityDirectory, heartbeat, nil
}
func main() {
	path := flag.String("config", "", "node media configuration file")
	flag.Parse()
	server, registry, directory, heartbeat, err := load(strings.TrimSpace(*path))
	if err != nil {
		fmt.Fprintln(os.Stderr, "node media configuration invalid")
		os.Exit(1)
	}
	defer func() { _ = heartbeat.Close() }()
	if heartbeat.Pulse(time.Now()) != nil {
		fmt.Fprintln(os.Stderr, "node media boot request unavailable")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	mediaauthority.RefreshDirectory(registry, directory, time.Now())
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				if heartbeat.Pulse(now) == nil {
					mediaauthority.RefreshDirectory(registry, directory, now)
				}
			}
		}
	}()
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		_ = server.Shutdown(shutdown)
	}()
	if err = server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "node media listener unavailable")
		os.Exit(1)
	}
}
