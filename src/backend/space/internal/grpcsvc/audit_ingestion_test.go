package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaudit"
	"voice/backend/space/internal/auditprincipal"
	"voice/backend/space/internal/store"
)

type auditImportSpy struct {
	calls    int
	producer string
	event    spaceaudit.ReplayEvent
	err      error
}

func (s *auditImportSpy) ImportAuditEvent(_ context.Context, p string, e spaceaudit.ReplayEvent) error {
	s.calls++
	s.producer = p
	s.event = e
	return s.err
}

func newAuditIngestionTestClient(t *testing.T, storage AuditIngestionStore) (spacev1.SpaceServiceClient, func(string, *spacev1.AppendAuditEventRequest) error) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	verifier := auditprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(verifier.Unary()))
	t.Cleanup(server.Stop)
	RegisterAuditIngestionServer(server, &AuditIngestionGRPC{Store: storage})
	listener := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = listener.Close() })
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///audit", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := spacev1.NewSpaceServiceClient(conn)
	call := func(producer string, req *spacev1.AppendAuditEventRequest) error {
		issuer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: producer, KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
		require.NoError(t, e)
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		id := uuid.NewString()
		token, e := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_AppendAuditEvent_FullMethodName, RequestID: id, RequestHash: hash})
		require.NoError(t, e)
		ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 3*time.Second)
		defer cancel()
		_, e = client.AppendAuditEvent(ctx, req)
		return e
	}
	return client, call
}

func TestAuditIngestionSignedRPCValidationAndErrorMapping(t *testing.T) {
	var err error
	spy := &auditImportSpy{}
	client, call := newAuditIngestionTestClient(t, spy)
	now := time.Now()
	makeRequest := func() *spacev1.AppendAuditEventRequest {
		return &spacev1.AppendAuditEventRequest{AuditEventId: uuid.NewString(), SpaceId: uuid.NewString(), ActorProfileId: uuid.NewString(), Action: "role_created", TargetType: "role", TargetId: uuid.NewString(), DetailsJson: "{}", OccurredAt: timestamppb.New(now)}
	}
	for _, producer := range []string{"role", "chat"} {
		req := makeRequest()
		if producer == "chat" {
			req.Action = "chat_created"
			req.TargetType = "chat"
		}
		require.NoError(t, call(producer, req))
		require.Equal(t, producer, spy.producer)
		require.Equal(t, req.ActorProfileId, spy.event.ActorProfileID)
		require.Equal(t, req.OccurredAt.Seconds, spy.event.OccurredAt.Seconds)
		require.Equal(t, req.OccurredAt.Nanos, spy.event.OccurredAt.Nanos)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*spacev1.AppendAuditEventRequest)
		code   codes.Code
	}{
		{"foreign producer action", func(r *spacev1.AppendAuditEventRequest) { r.Action = "chat_created"; r.TargetType = "chat" }, codes.PermissionDenied},
		{"noncanonical details", func(r *spacev1.AppendAuditEventRequest) { r.DetailsJson = "{ }" }, codes.InvalidArgument},
		{"wrong target", func(r *spacev1.AppendAuditEventRequest) { r.TargetType = "space" }, codes.InvalidArgument},
		{"missing timestamp", func(r *spacev1.AppendAuditEventRequest) { r.OccurredAt = nil }, codes.InvalidArgument},
		{"invalid actor", func(r *spacev1.AppendAuditEventRequest) { r.ActorProfileId = "" }, codes.InvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := makeRequest()
			tc.mutate(req)
			before := spy.calls
			require.Equal(t, tc.code, status.Code(call("role", req)))
			require.Equal(t, before, spy.calls)
		})
	}
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{{store.ErrAuditEventConflict, codes.AlreadyExists}, {errors.New("private database address"), codes.Unavailable}} {
		spy.err = tc.err
		err = call("role", makeRequest())
		require.Equal(t, tc.code, status.Code(err))
		require.NotContains(t, err.Error(), "private database")
	}
	_, err = client.GetSpace(context.Background(), &spacev1.GetSpaceRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err), "dedicated listener registers only ingestion")
	direct := &AuditIngestionGRPC{Store: spy}
	before := spy.calls
	_, err = direct.AppendAuditEvent(principal.WithVerified(context.Background(), principal.Principal{Issuer: "role", Kind: "service", Subject: "service:role"}), makeRequest())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, before, spy.calls)
}

func TestAuditIngestionSignedRPCPersistsOneEffect(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL ingestion migration")
	}
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	for _, name := range []string{"000015_audit_ledger.up.sql", "000025_tree_audit_actions.up.sql", "000027_audit_ingestion.up.sql"} {
		body, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	st := &store.SpaceStore{Pool: pool}
	actor := uuid.New()
	space, err := st.CreateSpace(ctx, actor, "Ingestion RPC", "", "private")
	require.NoError(t, err)
	_, call := newAuditIngestionTestClient(t, st)
	for _, producer := range []string{"role", "chat"} {
		req := &spacev1.AppendAuditEventRequest{AuditEventId: uuid.NewString(), SpaceId: space.ID.String(), ActorProfileId: actor.String(), Action: producer + "_created", TargetType: producer, TargetId: uuid.NewString(), DetailsJson: "{}", OccurredAt: &timestamppb.Timestamp{Seconds: time.Now().Unix(), Nanos: 123456001}}
		require.NoError(t, call(producer, req))
		require.NoError(t, call(producer, req), "new credential, identical durable fact")
		var ledger, outbox, receipt int
		var savedActor, savedOrigin string
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*),min(actor_profile_id::text) FROM audit_log WHERE id=$1`, req.AuditEventId).Scan(&ledger, &savedActor))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox WHERE audit_event_id=$1`, req.AuditEventId).Scan(&outbox))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*),min(origin) FROM audit_event_receipts WHERE audit_event_id=$1`, req.AuditEventId).Scan(&receipt, &savedOrigin))
		require.Equal(t, 1, ledger)
		require.Equal(t, 1, outbox)
		require.Equal(t, 1, receipt)
		require.Equal(t, req.ActorProfileId, savedActor)
		require.Equal(t, producer, savedOrigin)
		req.OccurredAt.Nanos++
		require.Equal(t, codes.AlreadyExists, status.Code(call(producer, req)))
		req.DetailsJson = "{} "
		require.Equal(t, codes.InvalidArgument, status.Code(call(producer, req)), "validation precedes replay lookup")
	}
}
