package presence

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
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/principal"
)

const notificationPresenceMethod = userv1.UserService_GetNotificationRoutingPresence_FullMethodName

var notificationKIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// GRPCChecker calls User's single-purpose, authenticated routing-presence RPC.
type GRPCChecker struct {
	client userv1.UserServiceClient
	issuer *principal.Issuer
	conn   *grpc.ClientConn
}

type UnavailableChecker struct{}

func (UnavailableChecker) IsOnline(context.Context, uuid.UUID) (bool, error) {
	return false, errors.New("Notification routing presence authority is not configured")
}

func (c *GRPCChecker) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

type PrincipalJWKSHTTPServer struct {
	Server   *http.Server
	CertFile string
	KeyFile  string
}

type notificationJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type notificationJWKS struct {
	Keys []notificationJWK `json:"keys"`
}

// LoadAuthenticatedGRPCCheckerFromEnv builds the outbound User client and the
// TLS JWKS endpoint that lets User verify Notification's request-bound token.
// Partial configuration is an error; callers must not silently route as offline.
func LoadAuthenticatedGRPCCheckerFromEnv(lookupEnv func(string) (string, bool)) (*GRPCChecker, *PrincipalJWKSHTTPServer, error) {
	names := []string{
		"NOTIFICATION_PRINCIPAL_SIGNING_KEYS_DIR",
		"NOTIFICATION_PRINCIPAL_ACTIVE_KID",
		"NOTIFICATION_PRINCIPAL_JWKS_LISTEN",
		"NOTIFICATION_PRINCIPAL_JWKS_TLS_CERT_FILE",
		"NOTIFICATION_PRINCIPAL_JWKS_TLS_KEY_FILE",
		"USER_NOTIFICATION_PRINCIPAL_GRPC_ADDR",
		"USER_NOTIFICATION_PRINCIPAL_TLS_CA_FILE",
		"USER_NOTIFICATION_PRINCIPAL_TLS_SERVER_NAME",
	}
	enabled := false
	for _, name := range names {
		if _, configured := lookupEnv(name); configured {
			enabled = true
		}
	}
	if !enabled {
		return nil, nil, nil
	}
	values := make(map[string]string, len(names))
	for _, name := range names {
		value, configured := lookupEnv(name)
		values[name] = strings.TrimSpace(value)
		if !configured || values[name] == "" {
			return nil, nil, errors.New("complete Notification presence principal and User TLS configuration is required")
		}
	}
	keys, err := loadNotificationSigningKeys(values["NOTIFICATION_PRINCIPAL_SIGNING_KEYS_DIR"])
	if err != nil {
		return nil, nil, err
	}
	activeKID := values["NOTIFICATION_PRINCIPAL_ACTIVE_KID"]
	activeKey := keys[activeKID]
	if activeKey == nil {
		return nil, nil, errors.New("active Notification principal key is missing")
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "notification", KeyID: activeKID, PrivateKey: activeKey})
	if err != nil {
		return nil, nil, err
	}
	roots := x509.NewCertPool()
	caBytes, err := os.ReadFile(values["USER_NOTIFICATION_PRINCIPAL_TLS_CA_FILE"])
	if err != nil || !roots.AppendCertsFromPEM(caBytes) {
		return nil, nil, errors.New("invalid User Notification principal TLS CA")
	}
	transport := credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
		ServerName: values["USER_NOTIFICATION_PRINCIPAL_TLS_SERVER_NAME"],
	})
	conn, err := grpc.NewClient(grpcclient.DialTarget(values["USER_NOTIFICATION_PRINCIPAL_GRPC_ADDR"]), grpc.WithTransportCredentials(transport))
	if err != nil {
		return nil, nil, err
	}
	checker := &GRPCChecker{client: userv1.NewUserServiceClient(conn), issuer: issuer, conn: conn}
	keyDocument, err := marshalNotificationJWKS(keys)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(keyDocument)
	})
	server := &http.Server{Addr: values["NOTIFICATION_PRINCIPAL_JWKS_LISTEN"], Handler: mux, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	httpserver.ApplyHTTPServerTimeouts(server)
	return checker, &PrincipalJWKSHTTPServer{
		Server:   server,
		CertFile: values["NOTIFICATION_PRINCIPAL_JWKS_TLS_CERT_FILE"],
		KeyFile:  values["NOTIFICATION_PRINCIPAL_JWKS_TLS_KEY_FILE"],
	}, nil
}

func loadNotificationSigningKeys(dir string) (map[string]*rsa.PrivateKey, error) {
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return nil, errors.New("Notification principal key directory is invalid")
	}
	entries, err := os.ReadDir(canonical)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PrivateKey, 2)
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), "..") {
			continue
		}
		if filepath.Ext(entry.Name()) != ".pem" {
			return nil, errors.New("Notification principal key files must use .pem extension")
		}
		kid := strings.TrimSuffix(entry.Name(), ".pem")
		if !notificationKIDPattern.MatchString(kid) {
			return nil, errors.New("invalid Notification principal key identifier")
		}
		path, err := filepath.EvalSymlinks(filepath.Join(canonical, entry.Name()))
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(canonical, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, errors.New("Notification principal key escapes configured directory")
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("Notification principal key is not a regular file")
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		block, rest := pem.Decode(contents)
		if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
			return nil, errors.New("Notification principal key must be one PKCS#8 PEM block")
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		key, ok := parsed.(*rsa.PrivateKey)
		if err != nil || !ok || key.Validate() != nil || key.N.BitLen() < 2048 {
			return nil, errors.New("Notification principal key must be RSA 2048+")
		}
		keys[kid] = key
	}
	if len(keys) != 2 {
		return nil, errors.New("Notification principal key directory must contain exactly two keys")
	}
	seen := map[string]struct{}{}
	for _, key := range keys {
		fingerprint := fmt.Sprintf("%s:%d", key.N.Text(16), key.E)
		if _, ok := seen[fingerprint]; ok {
			return nil, errors.New("Notification principal rotation keys must be distinct")
		}
		seen[fingerprint] = struct{}{}
	}
	return keys, nil
}

func marshalNotificationJWKS(keys map[string]*rsa.PrivateKey) ([]byte, error) {
	kids := make([]string, 0, len(keys))
	for kid := range keys {
		kids = append(kids, kid)
	}
	sort.Strings(kids)
	document := notificationJWKS{Keys: make([]notificationJWK, 0, len(kids))}
	for _, kid := range kids {
		key := &keys[kid].PublicKey
		exponent := big.NewInt(int64(key.E)).Bytes()
		document.Keys = append(document.Keys, notificationJWK{
			Kty: "RSA",
			Kid: kid,
			Use: "sig",
			Alg: "RS256",
			N:   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(exponent),
		})
	}
	return json.Marshal(document)
}

func (c *GRPCChecker) IsOnline(ctx context.Context, profileID uuid.UUID) (bool, error) {
	if c == nil || c.client == nil || c.issuer == nil {
		return false, errors.New("Notification routing presence authority is not configured")
	}
	req := &userv1.GetNotificationRoutingPresenceRequest{ProfileId: profileID.String()}
	hash, err := principal.RequestHash(proto.Message(req))
	if err != nil {
		return false, err
	}
	requestID := uuid.NewString()
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: notificationPresenceMethod, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return false, err
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	response, err := c.client.GetNotificationRoutingPresence(ctx, req)
	if err != nil {
		return false, err
	}
	if response == nil {
		return false, errors.New("User returned no Notification routing presence")
	}
	return response.GetHasActiveSession(), nil
}
