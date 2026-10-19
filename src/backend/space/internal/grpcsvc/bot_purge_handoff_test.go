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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/spacecore"
	"voice/backend/space/internal/store"
)

type botPurgeObservedStore struct {
	*store.SpaceStore
	reads atomic.Int32
}

func (s *botPurgeObservedStore) GetBotPurgeHandoff(ctx context.Context, space, op uuid.UUID, generation uint64, manifest *commonv1.ManifestBinding) (*store.BotPurgeHandoff, error) {
	s.reads.Add(1)
	return s.SpaceStore.GetBotPurgeHandoff(ctx, space, op, generation, manifest)
}

func TestBotPurgeHandoffProtectedActual47Ledger(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL18 Redis mTLS protected HTTPS JWKS")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &store.SpaceStore{Pool: db.Pool}
	account, owner, op := uuid.New(), uuid.New(), uuid.New()
	space, err := admin.CreateSpace(ctx, owner, "P3 original relay", "", "private")
	require.NoError(t, err)
	request := &spacev1.DeleteSpaceRequest{SpaceId: space.ID.String(), OperationId: op.String(), ConfirmationName: space.Name, Proof: "controlled-auth-proof"}
	_, err = admin.ReserveLifecycleSchedule(ctx, account, owner, 7, request)
	require.NoError(t, err)
	// Controlled Auth producer evidence is admitted by the actual Store; this
	// fixture qualifies the relay, not a live Auth or remote participant purge.
	auth := validDeletionProofResponse(account, owner, 7, space.ID, op, space.Name, request.Proof)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(auth)
	require.NoError(t, err)
	hash := lifecycleEvidenceHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", raw)
	_, err = admin.RecordLifecycleDeletionProofReceipt(ctx, owner, op, raw, hash)
	require.NoError(t, err)
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32), ItemCount: 2}
	a, err := admin.BeginLifecycleFreeze(ctx, space.ID, manifest)
	require.NoError(t, err)
	packages := []string{"role", "chat", "messaging", "file", "calls", "matchmaking", "search", "subscription", "bot", "notification", "realtime"}
	recordFences := func(state commonv1.LifecycleFenceState) {
		for i, p := range spacecore.CanonicalLifecycleParticipants() {
			r, err := a.FenceRequest(p)
			require.NoError(t, err)
			inner, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
			require.NoError(t, err)
			wrapper := protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), inner)
			h := lifecycleEvidenceHash("voice."+packages[i]+".v1.ApplySpaceLifecycleFenceRequest", wrapper)
			a, err = admin.RecordLifecycleFenceReceipt(ctx, &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: space.ID.String(), DeletionOperationId: op.String(), Generation: a.Generation(), ParticipantId: p, AppliedState: state, RequestSha256: h[:], ManifestSha256: manifest.ManifestSha256, AppliedAt: timestamppb.Now()})
			require.NoError(t, err)
		}
	}
	recordFences(commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	_, _, err = admin.CompleteLifecycleSchedule(ctx, space.ID)
	require.NoError(t, err)
	// Existing test-only seven-day setup precedes the irreversible decision.
	_, err = admin.Pool.Exec(ctx, `UPDATE public.space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	a, err = admin.DecideLifecycleRecovery(ctx, space.ID, 2)
	require.NoError(t, err)
	recordFences(commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	require.NoError(t, a.BeginPurging())
	require.NoError(t, admin.PersistLifecycle(ctx, a))
	retire, err := a.RoleRetirementRequest()
	require.NoError(t, err)
	raw, err = proto.MarshalOptions{Deterministic: true}.Marshal(retire)
	require.NoError(t, err)
	hash = lifecycleEvidenceHash("voice.role.v1.RetireSpaceRequest", raw)
	role := &rolev1.RetireSpaceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: space.ID.String(), DeletionOperationId: op.String(), Generation: 2, State: rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, RequestSha256: hash[:], ManifestSha256: manifest.ManifestSha256, RetiredAt: timestamppb.Now()}
	a, err = admin.RecordLifecycleRoleRetirementReceipt(ctx, role)
	require.NoError(t, err)
	msgRequest, err := a.PurgeRequest(commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING)
	require.NoError(t, err)
	inner, err := proto.MarshalOptions{Deterministic: true}.Marshal(msgRequest)
	require.NoError(t, err)
	wrapper := protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), inner)
	msgHash := lifecycleEvidenceHash("voice.messaging.v1.PurgeSpaceRequest", wrapper)
	messaging := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: space.ID.String(), DeletionOperationId: op.String(), Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: msgHash[:], CompletedAt: timestamppb.Now()}
	db.Run(true, "goto", "46")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	serving := &store.SpaceStore{Pool: nativeBotServingLogin(t, ctx, db.Pool)}
	certFile, keyFile, caFile, clientCert, roots := nativeBotTLSMaterial(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := []map[string]string{}
	for i, k := range []*rsa.PrivateKey{key, next} {
		kid := "current"
		if i == 1 {
			kid = "next"
		}
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()), "e": "AQAB"})
	}
	var jwksUnavailable atomic.Bool
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if jwksUnavailable.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	identity, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	redis, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForLog("Ready to accept connections")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, redis.Terminate(context.Background())) })
	redisAddr, err := redis.Endpoint(ctx, "")
	require.NoError(t, err)
	dir := t.TempDir()
	clientCertFile, clientKeyFile := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client-key.pem")
	clientKey, err := x509.MarshalPKCS8PrivateKey(clientCert.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(clientCertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(clientKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKey}), 0600))
	runtime, err := botmembershipprincipal.New(ctx, botmembershipprincipal.Config{ListenAddr: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: caFile, ReplayAddr: redisAddr, JWKSCAFile: caFile, JWKSClientCertFile: clientCertFile, JWKSClientKeyFile: clientKeyFile, JWKSURLs: map[string]string{"bot": jwks.URL}, RefreshAfter: 100 * time.Millisecond, HardExpiry: 200 * time.Millisecond, UnknownKIDCooldown: 50 * time.Millisecond})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	server := grpc.NewServer(runtime.ServerOptions()...)
	observed := &botPurgeObservedStore{SpaceStore: serving}
	RegisterBotMembershipServer(server, &BotMembershipGRPC{Store: observed})
	t.Cleanup(server.Stop)
	go func() { _ = server.Serve(listener) }()
	connect := func(certs []tls.Certificate) spacev1.SpaceServiceClient {
		c, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c.Close()) })
		return spacev1.NewSpaceServiceClient(c)
	}
	client := connect([]tls.Certificate{clientCert})
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	r := &spacev1.GetBotPurgeHandoffRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: space.ID.String(), DeletionOperationId: op.String(), Generation: 2, Manifest: manifest}
	sign := func(r *spacev1.GetBotPurgeHandoffRequest, method string) context.Context {
		h, err := principal.RequestHash(r)
		require.NoError(t, err)
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: r.RequestId, RequestHash: h})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", r.RequestId))
	}
	call := func(client spacev1.SpaceServiceClient, c context.Context, r *spacev1.GetBotPurgeHandoffRequest) (*spacev1.GetBotPurgeHandoffResponse, error) {
		c, cancel := context.WithTimeout(c, 3*time.Second)
		defer cancel()
		return client.GetBotPurgeHandoff(c, r)
	}
	_, err = call(client, ctx, r)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = call(client, sign(r, spacev1.SpaceService_RemoveBotMember_FullMethodName), r)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	oldToken := sign(r, spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName)
	got, err := call(client, oldToken, r)
	require.NoError(t, err)
	require.NotNil(t, got.Role)
	require.Nil(t, got.Messaging, "NOT_STARTED is absent, never a synthetic completion")
	require.True(t, proto.Equal(role, got.Role.OriginalReceipt))
	_, err = call(client, oldToken, r)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "real Redis rejects JTI replay")
	_, err = serving.RecordLifecyclePurgeReceipt(ctx, messaging)
	require.NoError(t, err)
	got, err = call(client, sign(r, spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName), r)
	require.NoError(t, err)
	require.True(t, proto.Equal(messaging, got.Messaging.OriginalReceipt))
	var storedRequest, storedReceipt []byte
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT request_bytes,receipt_bytes FROM public.space_lifecycle_participants WHERE space_id=$1 AND participant_id=$2 AND request_kind='PURGE' AND progress='COMPLETE'`, space.ID, int16(commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING)).Scan(&storedRequest, &storedReceipt))
	require.Equal(t, storedRequest, got.Messaging.OriginalRequestBytes)
	require.Equal(t, storedReceipt, got.Messaging.OriginalReceiptBytes)
	again, err := call(client, sign(r, spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName), r)
	require.NoError(t, err)
	require.True(t, proto.Equal(got, again))
	changed := proto.Clone(r).(*spacev1.GetBotPurgeHandoffRequest)
	changed.Generation++
	_, err = call(client, sign(changed, spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName), changed)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = call(connect(nil), sign(r, spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName), r)
	require.Error(t, err, "untrusted transport never reaches Store")
	var unchanged bool
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT phase='PURGING' AND NOT local_purge_completed AND NOT EXISTS(SELECT 1 FROM public.space_lifecycle_participants WHERE space_id=$1 AND participant_id=$2 AND request_kind='PURGE' AND progress='COMPLETE') FROM public.space_lifecycle_aggregates WHERE space_id=$1`, space.ID, int16(commonv1.ParticipantId_PARTICIPANT_ID_BOT)).Scan(&unchanged))
	require.True(t, unchanged)
	t.Run("trusted JWKS outage is retryable before Store", func(t *testing.T) {
		before := observed.reads.Load()
		jwksUnavailable.Store(true)
		defer jwksUnavailable.Store(false)
		time.Sleep(250 * time.Millisecond)
		_, err := call(client, sign(r, spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName), r)
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Equal(t, before, observed.reads.Load())
	})
	t.Run("unknown KID remains authentication failure", func(t *testing.T) {
		time.Sleep(60 * time.Millisecond)
		before := observed.reads.Load()
		unknown, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "unknown", PrivateKey: key})
		require.NoError(t, err)
		h, err := principal.RequestHash(r)
		require.NoError(t, err)
		token, err := unknown.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName, RequestID: r.RequestId, RequestHash: h})
		require.NoError(t, err)
		_, err = call(client, metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", r.RequestId)), r)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Equal(t, before, observed.reads.Load())
	})
	t.Run("real Redis outage is retryable before Store", func(t *testing.T) {
		time.Sleep(60 * time.Millisecond)
		before := observed.reads.Load()
		require.NoError(t, redis.Stop(ctx, nil))
		_, err := call(client, sign(r, spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName), r)
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Equal(t, before, observed.reads.Load())
	})
}
