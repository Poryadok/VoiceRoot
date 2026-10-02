package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"voice/backend/federation/nodepublisher"
)

type configuration struct {
	MasterURL            string   `json:"master_url"`
	TrustFile            string   `json:"trust_file"`
	CredentialFile       string   `json:"credential_file"`
	CACertFile           string   `json:"ca_cert_file"`
	ClientCertFile       string   `json:"client_cert_file"`
	ClientKeyFile        string   `json:"client_key_file"`
	AuthorityDirectory   string   `json:"authority_directory"`
	Spaces               []string `json:"spaces"`
	IntervalMilliseconds int      `json:"interval_milliseconds"`
}
type trustConfiguration struct {
	Issuer      string            `json:"issuer"`
	Environment string            `json:"environment"`
	NodeID      string            `json:"node_id"`
	Keys        map[string]string `json:"keys"`
}

func readJSON(path string, target any) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return errors.New("configuration unavailable")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return errors.New("configuration unavailable")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("configuration invalid")
	}
	return nil
}
func load(path string) (*nodepublisher.Controller, error) {
	var config configuration
	var trust trustConfiguration
	if readJSON(path, &config) != nil || readJSON(config.TrustFile, &trust) != nil {
		return nil, nodepublisher.ErrConfig
	}
	credential, err := os.ReadFile(config.CredentialFile)
	if err != nil || len(credential) > 128 {
		return nil, nodepublisher.ErrConfig
	}
	ca, err := os.ReadFile(config.CACertFile)
	if err != nil || len(ca) > 1<<20 {
		return nil, nodepublisher.ErrConfig
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, nodepublisher.ErrConfig
	}
	certificate, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
	if err != nil {
		return nil, nodepublisher.ErrConfig
	}
	keys := map[string]ed25519.PublicKey{}
	for id, encoded := range trust.Keys {
		key, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return nil, nodepublisher.ErrConfig
		}
		keys[id] = key
	}
	sink, err := nodepublisher.NewFileSink(config.AuthorityDirectory)
	if err != nil {
		return nil, err
	}
	interval := time.Duration(config.IntervalMilliseconds) * time.Millisecond
	if config.IntervalMilliseconds == 0 {
		interval = 100 * time.Millisecond
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}}
	transport.MaxConnsPerHost = 8
	transport.MaxIdleConnsPerHost = 8
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	return nodepublisher.New(nodepublisher.Config{MasterURL: config.MasterURL, Issuer: trust.Issuer, Environment: trust.Environment, NodeID: trust.NodeID, Credential: strings.TrimSpace(string(credential)), Keys: keys, Spaces: config.Spaces, Client: client, Sink: sink, Interval: interval})
}
func main() {
	path := flag.String("config", "", "node authority configuration file")
	flag.Parse()
	controller, err := load(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "node authority controller configuration invalid")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// Emit transitions only. Sensitive credential/configuration/policy values
	// never enter logs; a failed refresh leaves signed expiry enforcement to SFU.
	states := map[string]bool{}
	initialized := map[string]bool{}
	var reportMutex sync.Mutex
	controller.Run(ctx, func(space string, err error) {
		reportMutex.Lock()
		defer reportMutex.Unlock()
		available := err == nil
		if initialized[space] && states[space] == available {
			return
		}
		initialized[space] = true
		states[space] = available
		fmt.Fprintf(os.Stderr, "node authority refresh space=%s available=%t\n", space, available)
	})
}
