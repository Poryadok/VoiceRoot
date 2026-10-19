package grpcsvc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func TestBotPurgeChatCoverageActualProtectedManifestRead(t *testing.T) {
	if testing.Short() {
		t.Skip("private Linux official PG/real Redis/mTLS/JWKS")
	}
	ctx := t.Context()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/messaging_db"))
	db.Run(true, "up")
	chatDB := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/chat_db"))
	chatDB.Run(true, "up")
	pins := &store.PinsStore{Pool: db.Pool, BotPinChatScope: &store.SQLChatThreadPolicy{Pool: botPinReadonlyChatPool(t, chatDB.Pool)}}
	st := &store.MessagesStore{Pool: db.Pool}
	space, op, chat := uuid.New(), uuid.New(), uuid.MustParse("00000000-0000-4000-8000-000000000010")
	foreign := uuid.MustParse("00000000-0000-4000-8000-000000000020")
	preimage := append([]byte("voice.chat.v1.SpaceDeletionManifest\x00"), space[:]...)
	preimage = append(preimage, op[:]...)
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], 7)
	preimage = append(preimage, n[:]...)
	binary.BigEndian.PutUint64(n[:], 1)
	preimage = append(preimage, n[:]...)
	preimage = append(preimage, chat[:]...)
	digest := sha256.Sum256(preimage)
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: digest[:], ItemCount: 1}
	_, err := st.ImportSpacePurgeManifestPage(ctx, store.SpacePurgeManifestPageInput{SpaceID: space, DeletionOperationID: op, ScheduleGeneration: 7, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, ItemIds: []string{chat.String()}, PageSha256: make([]byte, 32)}, SealsManifest: true, RequestBytes: []byte("page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("import")})
	require.NoError(t, err)
	for i, state := range []commonv1.LifecycleFenceState{commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED} {
		_, err = st.ApplySpaceLifecycleFence(ctx, store.SpaceLifecycleFenceInput{SpaceID: space, DeletionOperationID: op, Generation: uint64(7 + i), State: state, Manifest: manifest, RequestBytes: []byte(state.String()), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte(state.String())})
		require.NoError(t, err)
	}
	roots, caPEM, serverCert, botCert, messagingCert := botPinTestPKI(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := integrationtest.SourceJWKSDocument(t, key, next)
	var jwksMode, jwksRequests atomic.Int32
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/principal-jwks.json" {
			http.NotFound(w, r)
			return
		}
		jwksRequests.Add(1)
		if jwksMode.Load() == 1 {
			http.Error(w, "test provider unavailable", http.StatusServiceUnavailable)
			return
		}
		if jwksMode.Load() == 2 {
			_, _ = w.Write([]byte(`{"keys":[{"kid":"broken"}]}`))
			return
		}
		_, _ = w.Write(jwks)
	}))
	https.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	https.StartTLS()
	t.Cleanup(https.Close)
	cert, keyFile := writeGISMappingClientCert(t, messagingCert)
	serverFile, serverKey := writeGISMappingClientCert(t, serverCert)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caFile, caPEM, 0600))
	replayURL, replayServer := botCoverageReplayServer(t)
	runtime, err := StartBotPinCleanupServer(ctx, BotPinCleanupConfig{ListenAddr: "127.0.0.1:0", TLSCertFile: serverFile, TLSKeyFile: serverKey, ClientCAFile: caFile, JWKSURL: strings.Replace(https.URL, "127.0.0.1", "localhost", 1) + "/.well-known/principal-jwks.json", JWKSCAFile: caFile, JWKSClientCertFile: cert, JWKSClientKeyFile: keyFile, ReplayRedisURL: replayURL}, pins)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	conn, err := grpc.NewClient(runtime.Address(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: roots, Certificates: []tls.Certificate{botCert}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := messagingv1.NewMessagingServiceClient(conn)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	request := &messagingv1.GetBotPurgeChatCoverageRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: space.String(), DeletionOperationId: op.String(), Generation: 8, Manifest: manifest, ChatIds: []string{chat.String(), foreign.String()}}
	sign := func(q *messagingv1.GetBotPurgeChatCoverageRequest, rpc string) context.Context {
		hash, e := principal.RequestHash(q)
		require.NoError(t, e)
		token, e := issuer.IssueService(principal.ServiceInput{Audience: "messaging", RPC: rpc, RequestID: q.RequestId, RequestHash: hash})
		require.NoError(t, e)
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", q.RequestId)
	}
	proof := sign(request, messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName)
	response, err := client.GetBotPurgeChatCoverage(proof, request)
	require.NoError(t, err, "actual protected listener must provide only qualified manifest membership")
	require.EqualValues(t, 7, response.SourceScheduleGeneration)
	require.Len(t, response.Coverage, 2)
	require.Equal(t, chat.String(), response.Coverage[0].ChatId)
	require.True(t, response.Coverage[0].Covered)
	require.Equal(t, foreign.String(), response.Coverage[1].ChatId)
	require.False(t, response.Coverage[1].Covered)
	require.True(t, proto.Equal(request.Manifest, response.Manifest))
	require.Equal(t, request.RequestId, response.RequestId)
	_, err = client.GetBotPurgeChatCoverage(proof, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "real replay store denies same credential")
	_, err = client.GetBotPurgeChatCoverage(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	changed := proto.Clone(request).(*messagingv1.GetBotPurgeChatCoverageRequest)
	changed.Generation++
	_, err = client.GetBotPurgeChatCoverage(sign(changed, messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName), changed)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = client.GetBotPurgeChatCoverage(sign(request, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	call, stop := context.WithTimeout(sign(request, messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName), 3*time.Second)
	defer stop()
	_, err = client.GetBotPurgeChatCoverage(call, request)
	require.NoError(t, err, "fresh proof recovers exact readonly observation without new receipt")
	for _, mode := range []string{"wrong issuer", "wrong audience", "changed body", "raw identity", "outer unknown", "nested unknown", "duplicate chat", "mixed account", "mixed profile", "mixed epoch", "mixed actor", "mixed bot"} {
		t.Run(mode, func(t *testing.T) {
			q := proto.Clone(request).(*messagingv1.GetBotPurgeChatCoverageRequest)
			q.RequestId = uuid.NewString()
			want := codes.Unauthenticated
			if mode == "outer unknown" {
				q.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 1})
				want = codes.InvalidArgument
			}
			if mode == "nested unknown" {
				q.Manifest.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 1})
				want = codes.InvalidArgument
			}
			if mode == "duplicate chat" {
				q.ChatIds = []string{chat.String(), chat.String()}
				want = codes.InvalidArgument
			}
			signer := issuer
			audience := "messaging"
			if mode == "wrong issuer" {
				signer, err = principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
				require.NoError(t, err)
			}
			if mode == "wrong audience" {
				audience = "role"
			}
			hash, e := principal.RequestHash(q)
			require.NoError(t, e)
			token, e := signer.IssueService(principal.ServiceInput{Audience: audience, RPC: messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName, RequestID: q.RequestId, RequestHash: hash})
			require.NoError(t, e)
			if strings.HasPrefix(mode, "mixed ") {
				field := map[string]string{"mixed account": "account_id", "mixed profile": "profile_id", "mixed epoch": "session_epoch", "mixed actor": "actor_profile_id", "mixed bot": "bot_id"}[mode]
				var value any = uuid.NewString()
				if field == "session_epoch" {
					value = int64(9)
				}
				token = resignCoverageMixedClaim(t, key, token, field, value)
			}
			proof := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", q.RequestId)
			if mode == "changed body" {
				q.Manifest.ManifestSha256[0] ^= 1
			}
			if mode == "raw identity" {
				proof = metadata.AppendToOutgoingContext(proof, "x-voice-profile-id", uuid.NewString())
			}
			_, e = client.GetBotPurgeChatCoverage(proof, q)
			require.Equal(t, want, status.Code(e))
		})
	}
	_, err = (&BotPinCleanupGRPC{Pins: pins}).GetBotPurgeChatCoverage(principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot"}), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "generic verified context cannot mint private read authority")
	noCert, err := grpc.NewClient(runtime.Address(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: roots})))
	require.NoError(t, err)
	noCertCtx, cancelNoCert := context.WithTimeout(sign(request, messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName), time.Second)
	_, err = messagingv1.NewMessagingServiceClient(noCert).GetBotPurgeChatCoverage(noCertCtx, request)
	cancelNoCert()
	_ = noCert.Close()
	require.Error(t, err)
	t.Run("expiry behind actual SQL wait", func(t *testing.T) {
		holder, e := db.Pool.Begin(ctx)
		require.NoError(t, e)
		defer holder.Rollback(context.Background())
		_, e = holder.Exec(ctx, `LOCK TABLE messaging_space_lifecycle_fences IN ACCESS EXCLUSIVE MODE`)
		require.NoError(t, e)
		issued := time.Now().UTC().Add(-27 * time.Second)
		short, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return issued }})
		require.NoError(t, e)
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		token, e := short.IssueService(principal.ServiceInput{Audience: "messaging", RPC: messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName, RequestID: request.RequestId, RequestHash: hash})
		require.NoError(t, e)
		proof, cancel := context.WithTimeout(metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", request.RequestId), 6*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() { _, e := client.GetBotPurgeChatCoverage(proof, request); done <- e }()
		require.Eventually(t, func() bool {
			var blocked bool
			e := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'SELECT generation,state,deletion_operation_id,source_schedule_generation%')`).Scan(&blocked)
			return e == nil && blocked
		}, time.Second, 10*time.Millisecond)
		if wait := time.Until(time.Unix(issued.Unix()+30, 0).Add(50 * time.Millisecond)); wait > 0 {
			time.Sleep(wait)
		}
		require.NoError(t, holder.Rollback(ctx))
		require.Equal(t, codes.Unauthenticated, status.Code(<-done), "expired proof returns no membership observation after actual SQL wait")
		_, e = client.GetBotPurgeChatCoverage(sign(request, messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName), request)
		require.NoError(t, e)
	})
	var receipts int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_receipts`).Scan(&receipts))
	require.Zero(t, receipts, "coverage reads never create a cleanup receipt or pin effect")
	t.Run("dependency outages are retryable before SQL", func(t *testing.T) {
		unknown, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "unpublished", PrivateKey: key})
		require.NoError(t, e)
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		token, e := unknown.IssueService(principal.ServiceInput{Audience: "messaging", RPC: messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName, RequestID: request.RequestId, RequestHash: hash})
		require.NoError(t, e)
		_, e = client.GetBotPurgeChatCoverage(metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", request.RequestId), request)
		require.Equal(t, codes.Unauthenticated, status.Code(e), "complete approved JWKS missing kid is invalid proof, not outage")
		jwksMode.Store(1)
		t.Log("wait for actual production JWKS cache hard expiry; no test clock or resolver override")
		time.Sleep(61 * time.Second)
		assertUnavailableBeforeSQL := func() {
			holder, e := db.Pool.Begin(ctx)
			require.NoError(t, e)
			defer holder.Rollback(context.Background())
			_, e = holder.Exec(ctx, `LOCK TABLE messaging_space_lifecycle_fences IN ACCESS EXCLUSIVE MODE`)
			require.NoError(t, e)
			proof, cancel := context.WithTimeout(sign(request, messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName), 5*time.Second)
			defer cancel()
			_, e = client.GetBotPurgeChatCoverage(proof, request)
			require.Equal(t, codes.Unavailable, status.Code(e), "dependency error must be retryable before a locked SQL source can be entered")
		}
		before := jwksRequests.Load()
		assertUnavailableBeforeSQL()
		require.Greater(t, jwksRequests.Load(), before, "503 was fetched after real hard expiry")
		time.Sleep(5100 * time.Millisecond)
		jwksMode.Store(2)
		before = jwksRequests.Load()
		assertUnavailableBeforeSQL()
		require.Greater(t, jwksRequests.Load(), before, "malformed trusted key set was actually fetched")
		time.Sleep(5100 * time.Millisecond)
		jwksMode.Store(0)
		_, e = client.GetBotPurgeChatCoverage(sign(request, messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName), request)
		require.NoError(t, e, "fresh approved snapshot recovers the same read")
		require.NoError(t, replayServer.Stop(ctx, nil))
		assertUnavailableBeforeSQL()
		old := &messagingv1.ApplyBotChatPinRemovalRequest{OperationId: uuid.NewString()}
		_, e = client.ApplyBotChatPinRemoval(signedBotPinContext(t, ctx, issuer, old, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName), old)
		require.Equal(t, codes.Unauthenticated, status.Code(e), "old cleanup transport error domain remains unchanged")
	})
}

func botCoverageReplayServer(t *testing.T) (string, testcontainers.Container) {
	t.Helper()
	ctx := t.Context()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp")}, Started: true})
	if c != nil {
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			require.NoError(t, c.Terminate(cleanup))
		})
	}
	require.NoError(t, err)
	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	return "redis://" + net.JoinHostPort(host, port.Port()) + "/0", c
}

// Test-only re-signing proves the receiver rejects correctly signed mixed
// service/human claims, rather than only rejecting invalid RSA signatures.
func resignCoverageMixedClaim(t *testing.T, key *rsa.PrivateKey, token, field string, value any) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(raw, &claims))
	claims[field] = value
	raw, err = json.Marshal(claims)
	require.NoError(t, err)
	parts[1] = base64.RawURLEncoding.EncodeToString(raw)
	input := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}
