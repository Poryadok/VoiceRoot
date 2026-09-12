package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

type signedChatManifestClient struct {
	chatv1.ChatServiceClient
	issuer *principal.Issuer
}

func (c *signedChatManifestClient) GetSpacePurgeManifestPage(ctx context.Context, req *chatv1.GetSpacePurgeManifestPageRequest, opts ...grpc.CallOption) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, err
	}
	requestID := uuid.NewString()
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "chat", RPC: chatv1.ChatService_GetSpacePurgeManifestPage_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, err
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	return c.ChatServiceClient.GetSpacePurgeManifestPage(ctx, req, opts...)
}

type searchPrincipalJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}
type searchPrincipalJWKS struct {
	Keys []searchPrincipalJWK `json:"keys"`
}

func loadSignedChatManifestClientFromEnv() (chatv1.ChatServiceClient, *grpc.ClientConn, []byte, error) {
	names := []string{"SEARCH_CHAT_MANIFEST_GRPC_ADDR", "SEARCH_CHAT_MANIFEST_TLS_CA_FILE", "SEARCH_CHAT_MANIFEST_TLS_SERVER_NAME", "SEARCH_PRINCIPAL_SIGNING_KEYS_DIR", "SEARCH_PRINCIPAL_ACTIVE_KID"}
	enabled := false
	for _, name := range names {
		if _, ok := os.LookupEnv(name); ok {
			enabled = true
		}
	}
	if _, set := os.LookupEnv("S2S_SIGNING_KEY_PEM"); set {
		return nil, nil, nil, errors.New("S2S_SIGNING_KEY_PEM is forbidden")
	}
	if _, set := os.LookupEnv("S2S_SIGNING_KID"); set {
		return nil, nil, nil, errors.New("S2S_SIGNING_KID is forbidden")
	}
	if !enabled {
		return nil, nil, nil, nil
	}
	addr, dir, kid := strings.TrimSpace(os.Getenv("SEARCH_CHAT_MANIFEST_GRPC_ADDR")), strings.TrimSpace(os.Getenv("SEARCH_PRINCIPAL_SIGNING_KEYS_DIR")), strings.TrimSpace(os.Getenv("SEARCH_PRINCIPAL_ACTIVE_KID"))
	if addr == "" || dir == "" || kid == "" {
		return nil, nil, nil, errors.New("chat manifest address and Search principal key directory/active kid are required")
	}
	keys, err := loadSearchSigningKeys(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	active, ok := keys[kid]
	if !ok {
		return nil, nil, nil, errors.New("active Search principal kid is missing")
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "search", KeyID: kid, PrivateKey: active})
	if err != nil {
		return nil, nil, nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, nil, nil, err
	}
	if path := strings.TrimSpace(os.Getenv("SEARCH_CHAT_MANIFEST_TLS_CA_FILE")); path != "" {
		contents, readErr := os.ReadFile(path)
		if readErr != nil || !roots.AppendCertsFromPEM(contents) {
			return nil, nil, nil, errors.New("invalid Chat manifest TLS CA")
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: strings.TrimSpace(os.Getenv("SEARCH_CHAT_MANIFEST_TLS_SERVER_NAME"))}
	conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, nil, nil, err
	}
	document, err := json.Marshal(searchJWKS(keys))
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, err
	}
	return &signedChatManifestClient{ChatServiceClient: chatv1.NewChatServiceClient(conn), issuer: issuer}, conn, document, nil
}

func loadSearchSigningKeys(dir string) (map[string]*rsa.PrivateKey, error) {
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return nil, errors.New("search principal key directory is invalid")
	}
	entries, err := os.ReadDir(canonical)
	if err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PrivateKey{}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), "..") {
			continue
		}
		if filepath.Ext(entry.Name()) != ".pem" {
			return nil, fmt.Errorf("principal signing key file %q must end in .pem", entry.Name())
		}
		kid := strings.TrimSuffix(entry.Name(), ".pem")
		if !validSearchKID(kid) {
			return nil, errors.New("invalid Search principal kid")
		}
		path, err := filepath.EvalSymlinks(filepath.Join(canonical, entry.Name()))
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(canonical, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, errors.New("search principal key escapes configured directory")
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("search principal key is not a regular file")
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		block, rest := pem.Decode(contents)
		if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
			return nil, errors.New("search principal key must be one PKCS#8 PEM block")
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		key, ok := parsed.(*rsa.PrivateKey)
		if err != nil || !ok || key.Validate() != nil || key.N.BitLen() < 2048 {
			return nil, errors.New("search principal key must be RSA 2048+")
		}
		keys[kid] = key
	}
	if len(keys) != 2 {
		return nil, errors.New("search principal key directory must contain exactly two keys")
	}
	seen := map[string]struct{}{}
	for _, key := range keys {
		fingerprint := fmt.Sprintf("%s:%d", key.N.Text(16), key.E)
		if _, ok := seen[fingerprint]; ok {
			return nil, errors.New("search principal rotation keys must be distinct")
		}
		seen[fingerprint] = struct{}{}
	}
	return keys, nil
}
func validSearchKID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i, c := range value {
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'
		if !ok || (i == 0 && (c == '.' || c == '_' || c == '-')) {
			return false
		}
	}
	return true
}
func searchJWKS(keys map[string]*rsa.PrivateKey) searchPrincipalJWKS {
	kids := make([]string, 0, len(keys))
	for kid := range keys {
		kids = append(kids, kid)
	}
	sort.Strings(kids)
	out := searchPrincipalJWKS{Keys: make([]searchPrincipalJWK, 0, len(kids))}
	for _, kid := range kids {
		key := keys[kid].PublicKey
		out.Keys = append(out.Keys, searchPrincipalJWK{Kty: "RSA", Kid: kid, Use: "sig", Alg: "RS256", N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	return out
}
