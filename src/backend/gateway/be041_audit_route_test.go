package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// Invoked by Space's real network fixture. No replacement Space implementation
// or JWT verifier is linked into this Gateway process.
func TestBE041GatewayAuditRoute(t *testing.T) {
	address := os.Getenv("BE041_AUDIT_ADDR")
	if address == "" {
		t.Skip("invoked by Space TestBE041AuditRealGatewayRoute")
	}
	read := func(name string) string { value := os.Getenv(name); require.NotEmpty(t, value, name); return value }
	config := lifecycleClientConfig{address: address, ca: read("BE041_CA"), serverName: "localhost", cert: read("BE041_CLIENT_CERT"), key: read("BE041_CLIENT_KEY")}
	conn, err := config.dial()
	require.NoError(t, err)
	defer conn.Close()
	client := spacev1.NewSpaceServiceClient(conn)
	keyPEM, err := os.ReadFile(read("BE041_SIGNING_KEY"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, owner, member, outsider, space := read("BE041_ACCOUNT"), read("BE041_OWNER"), read("BE041_MEMBER"), read("BE041_OUTSIDER"), read("BE041_SPACE")
	mode := read("BE041_MODE")
	route := func(profile, query string, want int) map[string]any {
		h := newGateway(gatewayConfig{tokenClaims: map[string]tokenClaims{"fixture": {UserID: account, ProfileID: profile, SessionEpoch: 7, ExpiresAt: time.Now().Add(time.Minute)}}, transcoder: &transcoder{clients: grpcClients{spaceLifecycle: client}, lifecycleIssuer: issuer}})
		r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+space+"/audit-log"+query, nil)
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set("X-Voice-Profile-ID", outsider)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, want, w.Code, w.Body.String())
		require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
		require.NotContains(t, w.Body.String(), "SQLSTATE")
		var result map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		return result
	}
	switch mode {
	case "normal":
		first := route(owner, "?page_size=1", 200)["audit_log_list"].(map[string]any)
		require.Len(t, first["entries"], 1)
		cursor := first["next_cursor"].(string)
		require.NotEmpty(t, cursor)
		second := route(owner, "?page_size=1&cursor="+url.QueryEscape(cursor), 200)["audit_log_list"].(map[string]any)
		require.Len(t, second["entries"], 1)
		require.NotEqual(t, first["entries"], second["entries"])
		route(member, "?page_size=1", 200)
		route(member, "?page_size=1&cursor="+url.QueryEscape(cursor), 400)
		route(outsider, "", 404)
	case "owner-only":
		route(owner, "", 200)
		route(member, "", 403)
	case "role-deny":
		route(member, "", 403)
	case "role-outage", "floor-missing", "floor-corrupt", "floor-outage":
		route(owner, "", 503)
	case "stale":
		route(owner, "", 401)
	case "frozen":
		route(owner, "", 409)
		route(member, "", 404)
		route(outsider, "", 404)
	case "admission":
		request := &spacev1.GetAuditLogRequest{SpaceId: space, Page: &commonv1.CursorPageRequest{PageSize: 1}}
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		sign := func() string {
			token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_GetAuditLog_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, AccountID: account, ProfileID: owner, SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
			require.NoError(t, err)
			return token
		}
		claimsOf := func(token string) map[string]any {
			parts := strings.Split(token, ".")
			raw, err := base64.RawURLEncoding.DecodeString(parts[1])
			require.NoError(t, err)
			var claims map[string]any
			require.NoError(t, json.Unmarshal(raw, &claims))
			return claims
		}
		ctxFor := func(token string) context.Context {
			return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", claimsOf(token)["request_id"].(string)))
		}
		token := sign()
		_, err = client.GetAuditLog(ctxFor(token), request)
		require.NoError(t, err)
		_, err = client.GetAuditLog(ctxFor(token), request)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		mutate := func(change func(map[string]any)) string {
			parts := strings.Split(sign(), ".")
			claims := claimsOf(strings.Join(parts, "."))
			change(claims)
			raw, err := json.Marshal(claims)
			require.NoError(t, err)
			parts[1] = base64.RawURLEncoding.EncodeToString(raw)
			digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
			require.NoError(t, err)
			return parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(signature)
		}
		for name, change := range map[string]func(map[string]any){
			"issuer": func(c map[string]any) { c["iss"] = "role" }, "kind": func(c map[string]any) { c["principal_type"] = "service" },
			"audience": func(c map[string]any) { c["aud"] = "role" }, "rpc": func(c map[string]any) { c["rpc"] = "/other/read" },
			"hash":        func(c map[string]any) { c["request_hash"] = "sha256:" + strings.Repeat("0", 64) },
			"nil profile": func(c map[string]any) { c["profile_id"] = uuid.Nil.String() }, "alias profile": func(c map[string]any) { c["profile_id"] = "urn:uuid:" + owner },
			"nil account": func(c map[string]any) { c["account_id"] = uuid.Nil.String(); c["sub"] = uuid.Nil.String() },
			"zero epoch":  func(c map[string]any) { c["session_epoch"] = 0 }, "fraction epoch": func(c map[string]any) { c["session_epoch"] = 1.5 },
			"string epoch": func(c map[string]any) { c["session_epoch"] = "7" }, "overflow epoch": func(c map[string]any) { c["session_epoch"] = 1e30 },
			"expired": func(c map[string]any) { c["exp"] = time.Now().Add(-time.Second).Unix() },
			"future": func(c map[string]any) {
				c["iat"] = time.Now().Add(time.Minute).Unix()
				c["nbf"] = c["iat"]
				c["exp"] = time.Now().Add(80 * time.Second).Unix()
			},
		} {
			t.Run(name, func(t *testing.T) {
				_, err := client.GetAuditLog(ctxFor(mutate(change)), request)
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			})
		}
		for _, name := range []string{"raw only", "signed and raw", "duplicate bearer", "duplicate request", "tampered request"} {
			t.Run(name, func(t *testing.T) {
				ctx := ctxFor(sign())
				q := request
				switch name {
				case "raw only":
					ctx = metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", owner))
				case "signed and raw":
					ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-profile-id", owner)
				case "duplicate bearer":
					ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer other")
				case "duplicate request":
					ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", uuid.NewString())
				case "tampered request":
					q = &spacev1.GetAuditLogRequest{SpaceId: space}
				}
				_, err := client.GetAuditLog(ctx, q)
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			})
		}
		roots, err := os.ReadFile(config.ca)
		require.NoError(t, err)
		pool := x509.NewCertPool()
		require.True(t, pool.AppendCertsFromPEM(roots))
		bad, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool})))
		require.NoError(t, err)
		defer bad.Close()
		ctx, cancel := context.WithTimeout(ctxFor(sign()), time.Second)
		defer cancel()
		_, err = spacev1.NewSpaceServiceClient(bad).GetAuditLog(ctx, request)
		require.Error(t, err)
	default:
		t.Fatalf("unknown fixture mode %q", mode)
	}
}
