package federationmedia

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type fileConfig struct {
	MasterURL      string `json:"master_url"`
	MasterCAFile   string `json:"master_ca_file"`
	ClientCertFile string `json:"client_cert_file"`
	ClientKeyFile  string `json:"client_key_file"`
	NodeCAFile     string `json:"node_ca_file"`
	TrustFile      string `json:"trust_file"`
}
type trustConfig struct {
	Issuer      string            `json:"issuer"`
	Environment string            `json:"environment"`
	Keys        map[string]string `json:"keys"`
}

func readJSON(path string, value any) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return ErrUnavailable
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ErrUnavailable
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil || d.Decode(new(any)) != io.EOF {
		return ErrUnavailable
	}
	return nil
}
func roots(path string) (*x509.CertPool, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, ErrUnavailable
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(raw) {
		return nil, ErrUnavailable
	}
	return pool, nil
}

// LoadFromEnv is opt-in; an incomplete configured runtime fails startup.
func LoadFromEnv(getenv func(string) string) (*Client, error) {
	path := strings.TrimSpace(getenv("VOICE_FEDERATED_MEDIA_CONFIG"))
	if path == "" {
		return nil, nil
	}
	var config fileConfig
	var trust trustConfig
	if readJSON(path, &config) != nil || readJSON(config.TrustFile, &trust) != nil {
		return nil, ErrUnavailable
	}
	masterRoots, err := roots(config.MasterCAFile)
	if err != nil {
		return nil, err
	}
	nodeRoots, err := roots(config.NodeCAFile)
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	keys := map[string]ed25519.PublicKey{}
	for id, raw := range trust.Keys {
		key, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return nil, ErrUnavailable
		}
		keys[id] = key
	}
	master := http.DefaultTransport.(*http.Transport).Clone()
	master.TLSClientConfig = &tls.Config{RootCAs: masterRoots, Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	node := http.DefaultTransport.(*http.Transport).Clone()
	node.TLSClientConfig = &tls.Config{RootCAs: nodeRoots, MinVersion: tls.VersionTLS13}
	return New(Config{MasterURL: config.MasterURL, Issuer: trust.Issuer, Environment: trust.Environment, Keys: keys, MasterClient: &http.Client{Transport: master, Timeout: 2 * time.Second}, NodeClient: &http.Client{Transport: node, Timeout: 2 * time.Second}})
}
func (c *Client) Close() {
	if c != nil {
		c.config.MasterClient.CloseIdleConnections()
		c.config.NodeClient.CloseIdleConnections()
	}
}
