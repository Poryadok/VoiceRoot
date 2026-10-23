package grpcsvc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/botpinprincipal"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func botPinTestPKI(t *testing.T) (*x509.CertPool, []byte, tls.Certificate, tls.Certificate, tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(24735), Subject: pkix.Name{CommonName: "BE170 disposable test CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	require.NoError(t, err)
	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	server := issueGISMappingCert(t, certificate, key, "localhost", nil, []string{"localhost"})
	bot := issueGISMappingCert(t, certificate, key, "test Bot", []string{"urn:voice:test:bot-pin"}, nil)
	messaging := issueGISMappingCert(t, certificate, key, "test Messaging", []string{"urn:voice:test:messaging-pin"}, nil)
	return roots, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), server, bot, messaging
}

func botPinRealReplayURL(t *testing.T) string {
	t.Helper()
	ctx := t.Context()
	integrationtest.ConfigureDockerTesting()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp")}, Started: true})
	if container != nil {
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			require.NoError(t, container.Terminate(cleanup))
		})
	}
	require.NoError(t, err)
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	return "redis://" + net.JoinHostPort(host, port.Port()) + "/0"
}

func signedBotPinContext(t *testing.T, ctx context.Context, issuer *principal.Issuer, request *messagingv1.ApplyBotChatPinRemovalRequest, rpc string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "messaging", RPC: rpc, RequestID: request.OperationId, RequestHash: hash})
	require.NoError(t, err)
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", request.OperationId)
}

func botPinReadonlyChatPool(t *testing.T, owning *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	login := "be170_pin_read_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	password := uuid.NewString()
	quoted := pgx.Identifier{login}.Sanitize()
	_, err := owning.Exec(t.Context(), "CREATE ROLE "+quoted+" LOGIN PASSWORD '"+password+"'; GRANT SELECT ON chats,chat_scope_authority_heads,chat_space_lifecycle_fences,chat_match_squad_operations TO "+quoted)
	require.True(t, err == nil, "create disposable least-privilege Chat reader")
	destination, err := url.Parse(owning.Config().ConnString())
	require.True(t, err == nil && destination.Scheme == "postgres", "parse private fixture URI")
	// Serialize the explicit URI instead of mutating pgx.ConnConfig and using
	// ConnString(), which retains the old administrative source string.
	destination.User = url.UserPassword(login, password)
	reader, err := pgxpool.New(t.Context(), destination.String())
	require.True(t, err == nil, "open readonly Chat connection")
	t.Cleanup(reader.Close)
	var sessionUser, currentUser string
	var superuser bool
	err = reader.QueryRow(t.Context(), `SELECT session_user,current_user,(SELECT rolsuper FROM pg_roles WHERE rolname=current_user)`).Scan(&sessionUser, &currentUser, &superuser)
	require.True(t, err == nil, "verify reopened readonly identity")
	require.Equal(t, login, sessionUser)
	require.Equal(t, login, currentUser)
	require.False(t, superuser)
	return reader
}

func TestBotPinCleanupProtectedTransportWithOfficialChatSource(t *testing.T) {
	if testing.Short() {
		t.Skip("private Linux official PG/real Redis/mTLS/HTTPS-JWKS")
	}
	ctx := t.Context()
	messagingDB := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src", "backend", "migrations", "messaging_db"))
	messagingDB.Run(true, "up")
	chatDB := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src", "backend", "migrations", "chat_db"))
	chatDB.Run(true, "up")
	chat, space, actor, message := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := chatDB.Pool.Exec(ctx, `INSERT INTO chats(id,type,space_id,creator_profile_id,name) VALUES($1,'group',$2,$3,'Bot cleanup fixture')`, chat, space, uuid.New())
	require.NoError(t, err)
	_, err = messagingDB.Pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content) VALUES($1,$2,'group',$3,'retained Bot message')`, message, chat, actor)
	require.NoError(t, err)
	reader := &store.SQLChatThreadPolicy{Pool: botPinReadonlyChatPool(t, chatDB.Pool)}
	pins := &store.PinsStore{Pool: messagingDB.Pool, BotPinChatScope: reader}
	require.NoError(t, pins.UpsertPin(ctx, chat, message, uuid.New()))
	observationRequest := &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), OperationId: uuid.NewString()}
	scope, err := reader.LoadBotPinChatScope(ctx, chat, space)
	require.NoError(t, err)
	scope.OperationId = observationRequest.OperationId
	observationBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(observationRequest)
	require.NoError(t, err)
	observationSHA := sha256.Sum256(observationBytes)
	request := &messagingv1.ApplyBotChatPinRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: uuid.NewString(), DecisionSha256: make([]byte, 32), Purpose: "global_delete", BotId: uuid.NewString(), ActorProfileId: actor.String(), SpaceId: space.String(), ChatId: chat.String(), FrozenScopeSha256: make([]byte, 32), Evidence: &messagingv1.ApplyBotChatPinRemovalRequest_Native{Native: &messagingv1.BotRemovalNativeEvidence{InstallationId: uuid.NewString(), InstallOperationId: uuid.NewString(), Generation: 1}}, Attachments: []*messagingv1.BotChatPinRemovalAttachment{{AttachmentId: uuid.NewString(), OriginalScopeEvidence: &messagingv1.BotChatPinRemovalAttachment_OriginalScopeUnqualified{OriginalScopeUnqualified: true}, CurrentScope: &messagingv1.BotRemovalChatScopeObservation{Request: observationRequest, Response: &chatv1.GetBotChatScopeAuthorityResponse{Scope: scope, RequestSha256: observationSHA[:]}}}}}
	roots, caPEM, serverCert, botCert, messagingCert := botPinTestPKI(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := integrationtest.SourceJWKSDocument(t, key, next)
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/principal-jwks.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	https.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	https.StartTLS()
	t.Cleanup(https.Close)
	clientCertFile, clientKeyFile := writeGISMappingClientCert(t, messagingCert)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caFile, caPEM, 0600))
	serverCertFile, serverKeyFile := writeGISMappingClientCert(t, serverCert)
	runtime, err := StartBotPinCleanupServer(ctx, BotPinCleanupConfig{ListenAddr: "127.0.0.1:0", TLSCertFile: serverCertFile, TLSKeyFile: serverKeyFile, ClientCAFile: caFile, JWKSURL: strings.Replace(https.URL, "127.0.0.1", "localhost", 1) + "/.well-known/principal-jwks.json", JWKSClientCertFile: clientCertFile, JWKSClientKeyFile: clientKeyFile, JWKSCAFile: caFile, ReplayRedisURL: botPinRealReplayURL(t)}, pins)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	connection, err := grpc.NewClient(runtime.Address(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: roots, Certificates: []tls.Certificate{botCert}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "messaging", RPC: messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName, RequestID: request.OperationId, RequestHash: hash})
	require.NoError(t, err)
	callCtx, cancel := context.WithTimeout(metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", request.OperationId), 5*time.Second)
	defer cancel()
	response, err := messagingv1.NewMessagingServiceClient(connection).ApplyBotChatPinRemoval(callCtx, request)
	require.NoError(t, err)
	require.EqualValues(t, 1, response.GetReceipt().GetRemovedPinCount())
	client := messagingv1.NewMessagingServiceClient(connection)
	require.NoError(t, pins.UpsertPin(ctx, chat, message, uuid.New()))
	_, err = client.ApplyBotChatPinRemoval(metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "x-request-id", request.OperationId), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "same credential cannot replay")
	replay, err := client.ApplyBotChatPinRemoval(signedBotPinContext(t, ctx, issuer, request, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName), request)
	require.NoError(t, err)
	require.True(t, proto.Equal(response, replay), "fresh JTI replays exact immutable receipt")
	nextRequest := func() *messagingv1.ApplyBotChatPinRemovalRequest {
		next := proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest)
		next.OperationId, next.DecisionId = uuid.NewString(), uuid.NewString()
		next.Attachments[0].AttachmentId = uuid.NewString()
		return next
	}
	for _, mode := range []string{"missing", "wrong issuer", "wrong method", "changed body", "raw identity", "nested unknown", "current incarnation conflict"} {
		t.Run(mode, func(t *testing.T) {
			candidate := nextRequest()
			signer, method := issuer, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName
			wantCode := codes.Unauthenticated
			if mode == "wrong issuer" {
				signer, err = principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
				require.NoError(t, err)
			}
			if mode == "wrong method" {
				method = messagingv1.MessagingService_PinMessage_FullMethodName
			}
			if mode == "nested unknown" {
				candidate.Attachments[0].CurrentScope.Response.Scope.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
				wantCode = codes.InvalidArgument
			}
			if mode == "current incarnation conflict" {
				candidate.Attachments[0].CurrentScope.Response.Scope.ChatScopeIncarnation = uuid.NewString()
				wantCode = codes.FailedPrecondition
			}
			proof := signedBotPinContext(t, ctx, signer, candidate, method)
			if mode == "missing" {
				proof = ctx
			}
			if mode == "changed body" {
				candidate.FrozenScopeSha256[0] ^= 1
			}
			if mode == "raw identity" {
				proof = metadata.AppendToOutgoingContext(proof, "x-voice-profile-id", actor.String())
			}
			call, stop := context.WithTimeout(proof, 3*time.Second)
			defer stop()
			_, failure := client.ApplyBotChatPinRemoval(call, candidate)
			require.Equal(t, wantCode, status.Code(failure))
			var receipts int
			require.NoError(t, messagingDB.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_receipts WHERE operation_id=$1`, candidate.OperationId).Scan(&receipts))
			require.Zero(t, receipts)
		})
	}
	_, err = (&BotPinCleanupGRPC{Pins: pins}).ApplyBotChatPinRemoval(principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot"}), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "generic verified context cannot mint the private capability")
	_, err = pins.ApplyVerifiedBotChatPinRemoval(ctx, botpinprincipal.VerifiedCleanup{})
	require.ErrorIs(t, err, botpinprincipal.ErrInvalidProof)
	withoutCertificate, err := grpc.NewClient(runtime.Address(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: roots})))
	require.NoError(t, err)
	noCertRequest := nextRequest()
	noCertCtx, noCertCancel := context.WithTimeout(signedBotPinContext(t, ctx, issuer, noCertRequest, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName), time.Second)
	_, err = messagingv1.NewMessagingServiceClient(withoutCertificate).ApplyBotChatPinRemoval(noCertCtx, noCertRequest)
	noCertCancel()
	_ = withoutCertificate.Close()
	require.Error(t, err, "client certificate is mandatory before handler access")
	require.Contains(t, []codes.Code{codes.Unavailable, codes.DeadlineExceeded}, status.Code(err), "matched valid body must fail at the mTLS transport boundary")
	t.Run("expiry after actual CHAT wait", func(t *testing.T) {
		candidate := nextRequest()
		holder, acquireErr := messagingDB.Pool.Begin(ctx)
		require.NoError(t, acquireErr)
		defer holder.Rollback(context.Background())
		_, acquireErr = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "voice.messaging.message-mutation.v1/"+chat.String())
		require.NoError(t, acquireErr)
		issued := time.Now().UTC().Add(-27 * time.Second)
		short, issueErr := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return issued }})
		require.NoError(t, issueErr)
		proof, stop := context.WithTimeout(signedBotPinContext(t, ctx, short, candidate, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName), 6*time.Second)
		defer stop()
		done := make(chan error, 1)
		go func() { _, e := client.ApplyBotChatPinRemoval(proof, candidate); done <- e }()
		require.Eventually(t, func() bool {
			var blocked bool
			e := messagingDB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event='advisory' AND query LIKE '%hashtextextended%')`).Scan(&blocked)
			return e == nil && blocked
		}, time.Second, 10*time.Millisecond, "verified request must actually wait behind the CHAT guard")
		if wait := time.Until(time.Unix(issued.Unix()+30, 0).Add(50 * time.Millisecond)); wait > 0 {
			time.Sleep(wait)
		}
		require.NoError(t, holder.Rollback(ctx))
		require.Equal(t, codes.Unauthenticated, status.Code(<-done), "expired transport proof cannot mutate after the guard wait")
		var receipts int
		require.NoError(t, messagingDB.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_receipts WHERE operation_id=$1`, candidate.OperationId).Scan(&receipts))
		require.Zero(t, receipts)
		remaining, listErr := pins.ListPins(ctx, chat)
		require.NoError(t, listErr)
		require.Len(t, remaining, 1, "expired attempt preserves the later human pin")
		recovered, recoveryErr := client.ApplyBotChatPinRemoval(signedBotPinContext(t, ctx, issuer, candidate, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName), candidate)
		require.NoError(t, recoveryErr)
		require.EqualValues(t, 1, recovered.GetReceipt().GetRemovedPinCount(), "same immutable operation recovers with fresh transport authority")
		again, recoveryErr := client.ApplyBotChatPinRemoval(signedBotPinContext(t, ctx, issuer, candidate, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName), candidate)
		require.NoError(t, recoveryErr)
		require.True(t, proto.Equal(recovered, again))
	})
	remaining, listErr := pins.ListPins(ctx, chat)
	require.NoError(t, listErr)
	require.Empty(t, remaining)
	var revision int64
	require.NoError(t, messagingDB.Pool.QueryRow(ctx, `SELECT revision FROM bot_pin_removal_revisions WHERE chat_id=$1 AND actor_profile_id=$2`, chat, actor).Scan(&revision))
	require.EqualValues(t, 2, revision, "only the two fresh eligible effects advance cleanup revision")
	t.Run("retired official Chat source", func(t *testing.T) {
		// This target is frozen while the Chat still exists. Retirement is an
		// actual DELETE with official native-head trigger, not NotFound-as-done.
		candidate := nextRequest()
		require.NoError(t, pins.UpsertPin(ctx, chat, message, uuid.New()))
		_, deleteErr := chatDB.Pool.Exec(ctx, `DELETE FROM chats WHERE id=$1`, chat)
		require.NoError(t, deleteErr)
		retired, readErr := reader.LoadBotPinChatScope(ctx, chat, space)
		require.NoError(t, readErr)
		require.False(t, retired.Exists)
		require.Equal(t, scope.ChatScopeIncarnation, retired.ChatScopeIncarnation)
		require.Greater(t, retired.ChatAuthorityRevision, scope.ChatAuthorityRevision)
		completed, cleanupErr := client.ApplyBotChatPinRemoval(signedBotPinContext(t, ctx, issuer, candidate, messagingv1.MessagingService_ApplyBotChatPinRemoval_FullMethodName), candidate)
		require.NoError(t, cleanupErr)
		require.EqualValues(t, 1, completed.GetReceipt().GetRemovedPinCount(), "qualified retired head still requires actual pin cleanup")
		remaining, cleanupErr := pins.ListPins(ctx, chat)
		require.NoError(t, cleanupErr)
		require.Empty(t, remaining)
	})
}
