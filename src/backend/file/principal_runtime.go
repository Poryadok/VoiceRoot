package main

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	userv1 "voice.app/voice/user/v1"
	grpcsvc "voice/backend/file/internal/grpcsvc"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/principal"
)

// filePrincipalRuntime is deliberately independent from the inbound Story
// verifier: File's outbound authority has its own keys, HTTPS JWKS endpoint,
// and User-only TLS connection.
type filePrincipalRuntime struct {
	Issuer     *principal.Issuer
	keyID      string
	privateKey *rsa.PrivateKey
	UserOwner  *grpcsvc.UserProfileOwnerClient
	JWKS       *http.Server
	conn       *grpc.ClientConn
}

func (r *filePrincipalRuntime) Close() {
	if r != nil {
		if r.conn != nil {
			_ = r.conn.Close()
		}
		if r.JWKS != nil {
			_ = r.JWKS.Close()
		}
	}
}

type fileJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}
type fileJWKS struct {
	Keys []fileJWK `json:"keys"`
}

func loadFilePrincipalRuntime() (_ *filePrincipalRuntime, err error) {
	dir, dirSet := os.LookupEnv("FILE_PRINCIPAL_SIGNING_KEYS_DIR")
	kid, kidSet := os.LookupEnv("FILE_PRINCIPAL_ACTIVE_KID")
	required := dirSet || kidSet
	for _, n := range []string{"FILE_PRINCIPAL_JWKS_LISTEN", "FILE_PRINCIPAL_JWKS_TLS_CERT_FILE", "FILE_PRINCIPAL_JWKS_TLS_KEY_FILE", "USER_FILE_PRINCIPAL_GRPC_ADDR", "USER_FILE_PRINCIPAL_TLS_CA_FILE", "USER_FILE_PRINCIPAL_TLS_SERVER_NAME"} {
		if _, ok := os.LookupEnv(n); ok {
			required = true
		}
	}
	if !required {
		return nil, nil
	}
	dir, kid = strings.TrimSpace(dir), strings.TrimSpace(kid)
	if dir == "" || kid == "" {
		return nil, errors.New("FILE_PRINCIPAL_SIGNING_KEYS_DIR and FILE_PRINCIPAL_ACTIVE_KID are required")
	}
	keys, err := loadFilePrincipalKeys(dir)
	if err != nil {
		return nil, err
	}
	active := keys[kid]
	if active == nil {
		return nil, errors.New("FILE_PRINCIPAL_ACTIVE_KID is not one of the signing keys")
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: kid, PrivateKey: active})
	if err != nil {
		return nil, err
	}
	certFile, keyFile := strings.TrimSpace(os.Getenv("FILE_PRINCIPAL_JWKS_TLS_CERT_FILE")), strings.TrimSpace(os.Getenv("FILE_PRINCIPAL_JWKS_TLS_KEY_FILE"))
	if certFile == "" || keyFile == "" {
		return nil, errors.New("file principal JWKS TLS certificate and key are required")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("file principal JWKS TLS: %w", err)
	}
	addr := ":8443"
	if v, ok := os.LookupEnv("FILE_PRINCIPAL_JWKS_LISTEN"); ok {
		addr = strings.TrimSpace(v)
		if addr == "" {
			return nil, errors.New("FILE_PRINCIPAL_JWKS_LISTEN is empty")
		}
	}
	doc, err := json.Marshal(fileJWKSFromKeys(keys))
	if err != nil {
		return nil, err
	}
	if err := assertFileJWKSActiveKey(doc, kid, active); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=30")
		_, _ = w.Write(doc)
	})
	r := &filePrincipalRuntime{Issuer: issuer, keyID: kid, privateKey: active, JWKS: &http.Server{Addr: addr, Handler: mux, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}}}
	httpserver.ApplyHTTPServerTimeouts(r.JWKS)
	defer func() {
		if err != nil {
			r.Close()
		}
	}()
	target, ca, server := strings.TrimSpace(os.Getenv("USER_FILE_PRINCIPAL_GRPC_ADDR")), strings.TrimSpace(os.Getenv("USER_FILE_PRINCIPAL_TLS_CA_FILE")), strings.TrimSpace(os.Getenv("USER_FILE_PRINCIPAL_TLS_SERVER_NAME"))
	if target == "" || ca == "" || server == "" {
		return nil, errors.New("user File principal address, TLS CA and server name are required")
	}
	pemData, err := os.ReadFile(ca)
	if err != nil {
		return nil, fmt.Errorf("user File principal TLS CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pemData) {
		return nil, errors.New("user File principal TLS CA has no certificates")
	}
	r.conn, err = grpc.NewClient(grpcclient.DialTarget(target), grpc.WithDisableRetry(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: server})))
	if err != nil {
		return nil, err
	}
	r.UserOwner = &grpcsvc.UserProfileOwnerClient{Client: userv1.NewUserServiceClient(r.conn), Issuer: issuer}
	return r, nil
}

func loadFilePrincipalKeys(dir string) (map[string]*rsa.PrivateKey, error) {
	root, err := filepath.EvalSymlinks(strings.TrimSpace(dir))
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PrivateKey{}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), "..") {
			continue
		}
		if filepath.Ext(e.Name()) != ".pem" {
			return nil, errors.New("file principal signing key must end in .pem")
		}
		kid := strings.TrimSuffix(e.Name(), ".pem")
		if !validPrincipalKeyID(kid) {
			return nil, errors.New("invalid File principal signing key ID")
		}
		path, err := filepath.EvalSymlinks(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return nil, errors.New("file principal signing key escapes directory")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
			return nil, errors.New("file principal key must be one PKCS#8 PEM block")
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok || key.Validate() != nil || key.N.BitLen() < 2048 {
			return nil, errors.New("file principal key must be valid RSA >=2048")
		}
		keys[kid] = key
	}
	if len(keys) != 2 {
		return nil, errors.New("file principal signing directory must contain exactly two keys")
	}
	seen := map[string]bool{}
	for _, k := range keys {
		fp := k.N.Text(16)
		if seen[fp] {
			return nil, errors.New("file principal signing keys must be distinct")
		}
		seen[fp] = true
	}
	return keys, nil
}
func fileJWKSFromKeys(keys map[string]*rsa.PrivateKey) fileJWKS {
	kids := make([]string, 0, len(keys))
	for k := range keys {
		kids = append(kids, k)
	}
	sort.Strings(kids)
	out := fileJWKS{Keys: make([]fileJWK, 0, 2)}
	for _, kid := range kids {
		k := keys[kid].PublicKey
		out.Keys = append(out.Keys, fileJWK{Kty: "RSA", Kid: kid, Use: "sig", Alg: "RS256", N: base64.RawURLEncoding.EncodeToString(k.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes())})
	}
	return out
}

// assertFileJWKSActiveKey prevents a deploy from signing with an active key
// which is absent from, or differs from, the standing File JWKS document.
func assertFileJWKSActiveKey(document []byte, kid string, active *rsa.PrivateKey) error {
	if active == nil {
		return errors.New("file principal active signing key is missing")
	}
	keys, err := principal.ParseJWKS(document)
	if err != nil {
		return fmt.Errorf("file principal JWKS: %w", err)
	}
	public := keys[kid]
	if public == nil || public.N.Cmp(active.N) != 0 || public.E != active.E {
		return errors.New("file principal active KID is not published by JWKS")
	}
	return nil
}

func validPrincipalKeyID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index, character := range value {
		letterOrDigit := (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')
		if !letterOrDigit && character != '.' && character != '_' && character != '-' {
			return false
		}
		if index == 0 && !letterOrDigit {
			return false
		}
	}
	return true
}
