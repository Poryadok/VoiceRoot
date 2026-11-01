package sourcecustody

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

type ClientConfig struct {
	Address, CAFile, ServerName, CertFile, KeyFile string
	SigningKeysDir, ActiveKID                      string
	HeartbeatTimeout                               time.Duration
}

type ClientRuntime struct {
	conn   *grpc.ClientConn
	client *Client
	jwks   []byte
}

var custodyKID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var projectedRevision = regexp.MustCompile(`^\.\.[0-9]{4}_[0-9]{2}_[0-9]{2}_[0-9]{2}_[0-9]{2}_[0-9]{2}\.[0-9]+$`)

func LoadClientConfig(lookup func(string) (string, bool)) (ClientConfig, bool, error) {
	if lookup == nil {
		return ClientConfig{}, false, ErrUnavailable
	}
	names := []string{"USER_CUSTODY_ADDR", "USER_CUSTODY_TLS_CA_FILE", "USER_CUSTODY_TLS_SERVER_NAME", "USER_CUSTODY_TLS_CERT_FILE", "USER_CUSTODY_TLS_KEY_FILE", "USER_CUSTODY_SIGNING_KEYS_DIR", "USER_CUSTODY_ACTIVE_KID", "USER_CUSTODY_HEARTBEAT_TIMEOUT"}
	values := make([]string, len(names))
	count := 0
	for i, name := range names {
		v, ok := lookup(name)
		if ok {
			count++
			values[i] = v
		}
	}
	if count == 0 {
		return ClientConfig{}, false, nil
	}
	if count != len(names) {
		return ClientConfig{}, false, ErrUnavailable
	}
	for _, v := range values {
		if v == "" || strings.TrimSpace(v) != v {
			return ClientConfig{}, false, ErrUnavailable
		}
	}
	duration, err := time.ParseDuration(values[7])
	if err != nil {
		return ClientConfig{}, false, ErrUnavailable
	}
	cfg := ClientConfig{Address: values[0], CAFile: values[1], ServerName: values[2], CertFile: values[3], KeyFile: values[4], SigningKeysDir: values[5], ActiveKID: values[6], HeartbeatTimeout: duration}
	if !cfg.valid() {
		return ClientConfig{}, false, ErrUnavailable
	}
	return cfg, true, nil
}

func (c ClientConfig) valid() bool {
	for _, v := range []string{c.Address, c.CAFile, c.ServerName, c.CertFile, c.KeyFile, c.SigningKeysDir, c.ActiveKID} {
		if v == "" || strings.TrimSpace(v) != v {
			return false
		}
	}
	host, port, err := net.SplitHostPort(c.Address)
	n, perr := strconv.Atoi(port)
	if err != nil || perr != nil || n < 1 || n > 65535 || host == "" || host == "0.0.0.0" || host == "::" || strings.ContainsAny(host, "/@ \\?#") {
		return false
	}
	if strings.ContainsAny(c.ServerName, "/@ \\?#:") || !custodyKID.MatchString(c.ActiveKID) {
		return false
	}
	return c.HeartbeatTimeout > 0 && c.HeartbeatTimeout <= 10*time.Second
}

// OpenClient validates local material and creates a lazy gRPC channel. A
// successful return does not prove peer identity, source qualification, or
// custody. Only an actual authenticated RPC performs the TLS handshake.
func OpenClient(ctx context.Context, cfg ClientConfig) (*ClientRuntime, error) {
	if ctx == nil || ctx.Err() != nil || !cfg.valid() {
		return nil, ErrUnavailable
	}
	issuer, public, err := LoadUserSigner(cfg.SigningKeysDir, cfg.ActiveKID)
	if err != nil {
		return nil, ErrUnavailable
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil || len(cert.Certificate) == 0 {
		return nil, ErrUnavailable
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, ErrUnavailable
	}
	clientAuth := false
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			clientAuth = true
		}
	}
	if !clientAuth {
		return nil, ErrUnavailable
	}
	ca, err := os.ReadFile(cfg.CAFile)
	if err != nil || len(ca) > 1<<20 {
		return nil, ErrUnavailable
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, ErrUnavailable
	}
	if ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	conn, err := grpc.NewClient(cfg.Address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: cfg.ServerName, Certificates: []tls.Certificate{cert}})))
	if err != nil {
		return nil, ErrUnavailable
	}
	client, err := NewClient(userv1.NewUserSourceCustodyServiceClient(conn), issuer, cfg.HeartbeatTimeout)
	if err != nil {
		_ = conn.Close()
		return nil, ErrUnavailable
	}
	if ctx.Err() != nil {
		_ = conn.Close()
		return nil, ErrUnavailable
	}
	return &ClientRuntime{conn: conn, client: client, jwks: public}, nil
}

// LoadUserSigner loads one explicit User principal signer and its complete
// public key set. It proves neither key custody nor source qualification.
func LoadUserSigner(dir, active string) (*principal.Issuer, []byte, error) {
	keys, public, err := loadCustodyKeys(dir, active)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: active, PrivateKey: keys})
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	return issuer, public, nil
}

// Keys use canonical LF-encoded PKCS#8 PEM, including its final newline.
// Mount immutability during loading and safe rotation require deployment custody.
func loadCustodyKeys(dir, active string) (*rsa.PrivateKey, []byte, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	type jwk struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	}
	keys := map[string]*rsa.PrivateKey{}
	var ids []string
	for _, entry := range entries {
		if entry.Name() == "..data" && entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if projectedRevision.MatchString(entry.Name()) && entry.IsDir() {
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pem") {
			return nil, nil, ErrUnavailable
		}
		kid := strings.TrimSuffix(entry.Name(), ".pem")
		if !custodyKID.MatchString(kid) {
			return nil, nil, ErrUnavailable
		}
		path, err := filepath.EvalSymlinks(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, nil, ErrUnavailable
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, nil, ErrUnavailable
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return nil, nil, ErrUnavailable
		}
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) > 1<<20 {
			return nil, nil, ErrUnavailable
		}
		if !bytes.HasPrefix(raw, []byte("-----BEGIN PRIVATE KEY-----")) {
			return nil, nil, ErrUnavailable
		}
		block, rest := pem.Decode(raw)
		if block == nil || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 || len(rest) != 0 || !bytes.Equal(raw, pem.EncodeToMemory(block)) {
			return nil, nil, ErrUnavailable
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		key, ok := parsed.(*rsa.PrivateKey)
		if err != nil || !ok || key.N.BitLen() < 2048 || key.Validate() != nil {
			return nil, nil, ErrUnavailable
		}
		keys[kid] = key
		ids = append(ids, kid)
	}
	if len(keys) != 2 || keys[active] == nil {
		return nil, nil, ErrUnavailable
	}
	sort.Strings(ids)
	if keys[ids[0]].PublicKey.Equal(&keys[ids[1]].PublicKey) {
		return nil, nil, ErrUnavailable
	}
	public := make([]jwk, 0, 2)
	for _, id := range ids {
		key := keys[id]
		public = append(public, jwk{Kty: "RSA", Kid: id, Use: "sig", Alg: "RS256", N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	data, err := json.Marshal(struct {
		Keys []jwk `json:"keys"`
	}{Keys: public})
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	return keys[active], data, nil
}

func (r *ClientRuntime) Client() *Client {
	if r == nil {
		return nil
	}
	return r.client
}

// Issuer returns the same immutable User signer used by this custody client.
// Sharing it avoids loading a second key snapshot during startup.
func (r *ClientRuntime) Issuer() *principal.Issuer {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.issuer
}
func (r *ClientRuntime) JWKS() []byte {
	if r == nil {
		return nil
	}
	return append([]byte(nil), r.jwks...)
}
func (r *ClientRuntime) Close() error {
	if r == nil || r.conn == nil {
		return ErrUnavailable
	}
	return r.conn.Close()
}
