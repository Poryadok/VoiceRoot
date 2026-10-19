package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/principalgrpc"
	"voice/backend/bot/internal/principalruntime"
	"voice/backend/bot/internal/sessionfloor"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func readNativePublicJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, target))
}

func nativePublicConnection(t *testing.T, address, name, ca, cert, key string) *grpc.ClientConn {
	t.Helper()
	raw, err := os.ReadFile(ca)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(raw))
	identity, err := tls.LoadX509KeyPair(cert, key)
	require.NoError(t, err)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: name, Certificates: []tls.Certificate{identity}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// This fixture requires the actual independently built Role/Space/Chat/Auth
// sources. Its generation-one prehistory is a real native cancel-before-add,
// not a fabricated published source or public uninstall completion.
func TestBotNativePublicInstallFixture(t *testing.T) {
	dir := os.Getenv("BOT_GATEWAY_NATIVE_FIXTURE_DIR")
	if dir == "" {
		t.Skip("explicit private full public install fixture only")
	}
	ctx := t.Context()
	require.NoError(t, os.MkdirAll(dir, 0700))
	var role struct {
		Schema      int    `json:"role_schema"`
		SpaceSchema int    `json:"space_schema"`
		Address     string `json:"role_address"`
		Name        string `json:"tls_server_name"`
		CA          string `json:"tls_ca_file"`
		Cert        string `json:"tls_client_cert_file"`
		Key         string `json:"tls_client_key_file"`
		Signer      string `json:"bot_signer_key_file"`
		Kid         string `json:"bot_signer_key_id"`
		Owner       string `json:"owner_account_id"`
		Profile     string `json:"owner_profile_id"`
		Space       string `json:"space_id"`
	}
	readNativePublicJSON(t, filepath.Join(os.Getenv("ROLE_NATIVE_FIXTURE_EXPORT_DIR"), "ready.json"), &role)
	require.Equal(t, 25, role.Schema)
	require.Equal(t, 44, role.SpaceSchema)
	var space struct {
		Address string `json:"address"`
		Name    string `json:"server_name"`
		CA      string `json:"ca_file"`
		Cert    string `json:"client_cert_file"`
		Key     string `json:"client_key_file"`
		Space   string `json:"space_id"`
		Profile string `json:"profile_id"`
	}
	spaceDir := os.Getenv("VOICE_BOT_NATIVE_HANDOFF_DIR")
	readNativePublicJSON(t, filepath.Join(spaceDir, "ready.json"), &space)
	require.Equal(t, role.Space, space.Space)
	var chat struct {
		Address string `json:"address"`
		Name    string `json:"server_name"`
		CA      string `json:"ca_file"`
		Cert    string `json:"client_cert_file"`
		Key     string `json:"client_key_file"`
		Space   string `json:"space_id"`
		Chat    string `json:"chat_id"`
	}
	chatDir := os.Getenv("VOICE_BOT_CHAT_HANDOFF_DIR")
	readNativePublicJSON(t, filepath.Join(chatDir, "ready.json"), &chat)
	require.Equal(t, role.Space, chat.Space)
	var auth struct {
		Port    int    `json:"port"`
		Account string `json:"account_id"`
		Kid     string `json:"kid"`
		Mode    string `json:"clock_mode"`
	}
	authDir := os.Getenv("BOT_AUTH_FLOOR_INTEROP_DIR")
	readNativePublicJSON(t, filepath.Join(authDir, "ready.json"), &auth)
	require.Equal(t, "system", auth.Mode)
	require.Equal(t, role.Owner, auth.Account)
	require.Equal(t, role.Kid, auth.Kid)
	pool := integrationtest.StartPostgres(t, ctx, "bot-native-public", "")
	paths, err := filepath.Glob("../../../migrations/bot_db/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err, path)
	}
	st := &store.BotStore{Pool: pool}
	bot, _, err := st.CreateBot(ctx, uuid.MustParse(role.Owner), "Actual HTTP native Bot", "", `[]`, uuid.MustParse(space.Profile))
	require.NoError(t, err)
	// The Gateway has its own test key and mTLS identity. All pure Bot calls,
	// including Auth floor, use ONE actual Runtime issuer below.
	gatewayKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gatewayKeyFile := filepath.Join(dir, "gateway-key.pem")
	require.NoError(t, os.WriteFile(gatewayKeyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(gatewayKey)}), 0600))
	template := &x509.Certificate{SerialNumber: big.NewInt(24744), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &gatewayKey.PublicKey, gatewayKey)
	require.NoError(t, err)
	certFile := filepath.Join(dir, "transport-cert.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "fixture", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(gatewayKey.N.Bytes()), "e": "AQAB"}}})
	}))
	t.Cleanup(jwks.Close)
	jwksCA := filepath.Join(dir, "gateway-jwks-ca.pem")
	require.NoError(t, os.WriteFile(jwksCA, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.Certificate().Raw}), 0600))
	replay := miniredis.RunT(t)
	runtime, err := principalruntime.New(ctx, principalruntime.Config{ReplayRedisURL: "redis://" + replay.Addr(), BotPrivateKeyFile: role.Signer, BotKeyID: role.Kid, BotTLSCert: certFile, BotTLSKey: gatewayKeyFile, BotJWKSClientCA: certFile})
	require.NoError(t, err)
	t.Cleanup(func() { _ = runtime.Close() })
	floor, floorConn, err := (sessionfloor.Config{Address: net.JoinHostPort(os.Getenv("BOT_AUTH_FLOOR_INTEROP_HOST"), strconv.Itoa(auth.Port)), CAFile: filepath.Join(authDir, "server-cert.pem"), ServerName: "localhost", ClientCertFile: filepath.Join(authDir, "bot-cert.pem"), ClientKeyFile: filepath.Join(authDir, "bot-key.pem")}).Dial(runtime.Issuer())
	require.NoError(t, err)
	t.Cleanup(func() { _ = floorConn.Close() })
	service := NewBotGRPC(st, nil)
	service.PrincipalIssuer = runtime.Issuer()
	service.InstallationRole = rolev1.NewRoleServiceClient(nativePublicConnection(t, role.Address, role.Name, role.CA, role.Cert, role.Key))
	service.InstallationSpace = spacev1.NewSpaceServiceClient(nativePublicConnection(t, space.Address, space.Name, space.CA, space.Cert, space.Key))
	service.ChatScope = chatv1.NewChatScopeAuthorityServiceClient(nativePublicConnection(t, chat.Address, chat.Name, chat.CA, chat.Cert, chat.Key))
	// Real native prehistory allocates and fences generation one in both stores.
	initial, err := st.PrepareInstallation(store.WithChatScopeObservations(ctx, nil), bot.ID, uuid.MustParse(role.Space), uuid.MustParse(role.Profile), nil)
	require.NoError(t, err)
	cancel, err := st.BeginInstallationCancellation(ctx, bot.ID, initial.SpaceID, initial.InstallerProfileID)
	require.NoError(t, err)
	require.NoError(t, service.removeInstallationMember(ctx, cancel))
	require.NoError(t, st.CompleteInstallationCancellation(ctx, cancel))
	protected, err := runtime.NewGatewayRuntime(principalruntime.GatewayConfig{CertFile: certFile, KeyFile: gatewayKeyFile, ClientCAFile: certFile, JWKSURL: jwks.URL, JWKSCAFile: jwksCA}, floor.RequireCurrent)
	require.NoError(t, err)
	t.Cleanup(protected.Close)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(protected.TLS)), grpc.UnaryInterceptor(principalgrpc.GatewayLifecycleUnaryInterceptor(protected.Verifier)), grpc.StreamInterceptor(principalgrpc.GatewayLifecycleStreamInterceptor()))
	botv1.RegisterBotServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	body, err := json.Marshal(map[string]any{"address": listener.Addr().String(), "bot_id": bot.ID, "account_id": role.Owner, "profile_id": role.Profile, "space_id": role.Space, "chat_id": chat.Chat, "chat_fixture_dir": chatDir, "key_file": gatewayKeyFile, "cert_file": certFile})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ready.json"), body, 0600))
	t.Cleanup(func() { _ = os.Remove(filepath.Join(dir, "ready.json")); _ = os.Remove(gatewayKeyFile) })
	require.Eventually(t, func() bool { _, err := os.Stat(filepath.Join(dir, "done")); return err == nil }, 65*time.Second, 50*time.Millisecond)
	intent, err := st.ReusableInstallationIntent(context.Background(), bot.ID, initial.SpaceID, []uuid.UUID{uuid.MustParse(chat.Chat)})
	require.NoError(t, err)
	require.NotNil(t, intent)
	require.EqualValues(t, 2, intent.Generation)
	require.Equal(t, "INSTALLED", intent.State)
	var observations, installed int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_scope_observations WHERE bot_id=$1`, bot.ID).Scan(&observations))
	require.Equal(t, 2, observations, "install and re-enable qualify independently; exact install retry retains its original source")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_whitelist WHERE bot_id=$1 AND enabled`, bot.ID).Scan(&installed))
	require.Equal(t, 1, installed)
	complete, err := json.Marshal(map[string]any{"install_operation_id": intent.OperationID, "generation": 2})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(spaceDir, "complete.json"), complete, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(chatDir, "complete.json"), []byte(`{"complete":true}`), 0600))
}
