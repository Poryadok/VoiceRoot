package reportthreshold_test

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	moderationv1 "voice.app/voice/moderation/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/grpcsvc"
	"voice/backend/moderation/internal/reportintake"
	"voice/backend/moderation/internal/reportthreshold"
	"voice/backend/moderation/internal/store"
	"voice/backend/pkg/automodclient"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
)

type audienceServer struct {
	userv1.UnimplementedUserServiceServer
	profile, account uuid.UUID
	outage           atomic.Bool
}

func (s *audienceServer) ResolveReportThresholdContext(_ context.Context, req *userv1.ResolveReportThresholdContextRequest) (*userv1.ResolveReportThresholdContextResponse, error) {
	if s.outage.Load() {
		return nil, status.Error(codes.Unavailable, "authority fixture unavailable")
	}
	if req.GetTargetProfileId() != s.profile.String() {
		return nil, status.Error(codes.InvalidArgument, "wrong target")
	}
	now := time.Now().UTC()
	return &userv1.ResolveReportThresholdContextResponse{State: userv1.ReportThresholdContextState_REPORT_THRESHOLD_CONTEXT_STATE_LIVE, EvaluationId: req.GetEvaluationId(), TargetProfileId: s.profile.String(), TargetAccountId: s.account.String(), ProfileRevision: 1, MetricVersion: store.ReportAudienceMetric, AudienceAccounts: 1000, SnapshotId: uuid.NewString(), ObservationStartedAt: timestamppb.New(now), ObservationCompletedAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(30 * time.Second))}, nil
}

type subjectServer struct {
	automodv1.UnimplementedAuthModerationAuthorityServiceServer
	account, subject uuid.UUID
	calls            atomic.Int64
}

func (s *subjectServer) ResolveModerationSubject(_ context.Context, req *automodv1.ResolveModerationSubjectRequest) (*automodv1.ResolveModerationSubjectResponse, error) {
	if req.GetAccountId() != s.account.String() {
		return nil, status.Error(codes.InvalidArgument, "wrong account")
	}
	s.calls.Add(1)
	return &automodv1.ResolveModerationSubjectResponse{Subject: &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: s.account.String(), SubjectId: s.subject.String(), LifecycleRevision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}, SessionEpoch: 1}, nil
}

type targetResolver func(context.Context, uuid.UUID, string, uuid.UUID) (string, uuid.UUID, error)

func (f targetResolver) Resolve(ctx context.Context, report uuid.UUID, kind string, target uuid.UUID) (string, uuid.UUID, error) {
	return f(ctx, report, kind, target)
}

func privateServer(t *testing.T, f integrationtest.SourceTLSFixture, target, method string, key *rsa.PrivateKey, register func(*grpc.Server)) string {
	t.Helper()
	cert, err := tls.LoadX509KeyPair(f.Config.TLSCertFile, f.Config.TLSKeyFile)
	require.NoError(t, err)
	ca, err := os.ReadFile(f.Config.ClientCAFile)
	require.NoError(t, err)
	clients := x509.NewCertPool()
	require.True(t, clients.AppendCertsFromPEM(ca))
	var lock sync.Mutex
	seen := map[string]bool{}
	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if info.FullMethod != method {
			return nil, status.Error(codes.PermissionDenied, "private method unavailable")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid metadata")
		}
		message, ok := req.(proto.Message)
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid body")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, err
		}
		verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "moderation", ExpectedAudience: target, ExpectedRPC: method, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash,
			KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
				if issuer != "moderation" || kid != "current" {
					return nil, errors.New("untrusted key")
				}
				return &key.PublicKey, nil
			},
			ReplayGuard: func(_ context.Context, _, jti string, _ time.Time) error {
				lock.Lock()
				defer lock.Unlock()
				if seen[jti] {
					return errors.New("replay")
				}
				seen[jti] = true
				return nil
			}})
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid service principal")
		}
		return next(principal.WithVerified(ctx, verified), req)
	}
	srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients})), grpc.UnaryInterceptor(interceptor))
	register(srv)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() { srv.Stop(); _ = listener.Close() })
	return listener.Addr().String()
}

func TestConfiguredRuntimeActivatesValidReceiptAfterAuthorityRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL and mTLS owner fixtures")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "thresholdruntime", "")
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	files, err := filepath.Glob(filepath.Join(root, "src/backend/migrations/moderation_db/*.up.sql"))
	require.NoError(t, err)
	for _, file := range files {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err, file)
	}
	f := integrationtest.NewSourceTLSFixture(t, authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "moderation", KeyID: "current", PrivateKey: f.Current})
	require.NoError(t, err)
	profile, account, subject := uuid.New(), uuid.New(), uuid.New()
	user := &audienceServer{profile: profile, account: account}
	user.outage.Store(true)
	userAddr := privateServer(t, f, "user", userv1.UserService_ResolveReportThresholdContext_FullMethodName, f.Current, func(s *grpc.Server) { userv1.RegisterUserServiceServer(s, user) })
	auth := &subjectServer{account: account, subject: subject}
	authAddr := privateServer(t, f, "auth", automodv1.AuthModerationAuthorityService_ResolveModerationSubject_FullMethodName, f.Current, func(s *grpc.Server) { automodv1.RegisterAuthModerationAuthorityServiceServer(s, auth) })
	authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(f.ClientCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = authConn.Close() })
	clientKey, err := x509.MarshalPKCS8PrivateKey(f.ClientCertificate.PrivateKey)
	require.NoError(t, err)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "client.crt"), filepath.Join(dir, "client.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.ClientCertificate.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKey}), 0600))
	t.Setenv("MODERATION_REPORT_THRESHOLD_USER_ADDR", userAddr)
	t.Setenv("MODERATION_REPORT_THRESHOLD_TLS_SERVER_NAME", "example.com")
	t.Setenv("MODERATION_REPORT_THRESHOLD_CA_FILE", f.Config.TLSCertFile)
	t.Setenv("MODERATION_REPORT_THRESHOLD_TLS_CERT_FILE", certPath)
	t.Setenv("MODERATION_REPORT_THRESHOLD_TLS_KEY_FILE", keyPath)
	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	require.NoError(t, err)
	go ns.Start()
	require.True(t, ns.ReadyForConnections(5*time.Second))
	t.Cleanup(func() { ns.Shutdown(); ns.WaitForShutdown() })
	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "moderation_events", Subjects: []string{"moderation.report_created", "moderation.sanction_applied", "moderation.appeal_submitted"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour})
	require.NoError(t, err)
	shared := automod.NewStore(pool)
	r, err := reportthreshold.NewRuntime(pool, issuer, automodclient.NewAuthClient(issuer, authConn), shared, ns.ClientURL())
	require.NoError(t, err)
	require.True(t, r.Configured)
	t.Cleanup(r.Close)
	reports := &store.ReportStore{Pool: pool}
	svc := &grpcsvc.ModerationGRPC{Reports: reports}
	for i := 0; i < 10; i++ {
		req := &moderationv1.CreateReportRequest{TargetType: "user", TargetId: profile.String(), Category: "spam"}
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		reporter := uuid.NewString()
		verified := principal.Principal{Kind: "delegated_user", Issuer: "gateway", Subject: reporter, AccountID: reporter, ProfileID: uuid.NewString(), SessionEpoch: 1, AccountType: "regular", Audience: "moderation", RPC: moderationv1.ModerationService_CreateReport_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash}
		receipt, err := svc.CreateReport(principal.WithVerified(ctx, verified), req)
		require.NoError(t, err)
		require.True(t, proto.Equal(receipt, &moderationv1.CreateReportResponse{Report: &moderationv1.Report{Id: receipt.GetReport().GetId()}}), "receipt stays opaque before authority is available")
	}
	validation := &reportintake.Worker{Reports: reports, Validated: r.Store, Targets: targetResolver(func(context.Context, uuid.UUID, string, uuid.UUID) (string, uuid.UUID, error) {
		return "LIVE", account, nil
	})}
	for i := 0; i < 10; i++ {
		worked, err := validation.ValidateOne(ctx)
		require.NoError(t, err)
		require.True(t, worked)
	}
	r.Start(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.Eventually(t, func() bool {
		var deferred int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM report_threshold_evaluations WHERE failure_code='AUTHORITY_UNAVAILABLE'`).Scan(&deferred)
		return err == nil && deferred > 0
	}, 10*time.Second, 50*time.Millisecond)
	var receipts, sanctions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM reports WHERE validation_state='VALID'),(SELECT count(*) FROM sanctions)`).Scan(&receipts, &sanctions))
	require.Equal(t, 10, receipts)
	require.Zero(t, sanctions)
	user.outage.Store(false)
	_, err = pool.Exec(ctx, `UPDATE report_threshold_evaluations SET next_at=clock_timestamp()-interval '1 second' WHERE completed_at IS NULL`)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		var delivered int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM report_threshold_outbox WHERE delivered_at IS NOT NULL`).Scan(&delivered)
		return err == nil && delivered == 1
	}, 15*time.Second, 50*time.Millisecond)
	r.Close()
	var cases int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM sanctions),(SELECT count(*) FROM report_threshold_cases)`).Scan(&sanctions, &cases))
	require.Equal(t, 1, sanctions)
	require.Equal(t, 1, cases)
	require.Positive(t, auth.calls.Load())
	var noMessagingEpoch bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_epoch IS NULL FROM automod_subjects WHERE subject_id=$1`, subject).Scan(&noMessagingEpoch))
	require.True(t, noMessagingEpoch, "a reported account need not send a message to acquire its shared subject")
	info, err := js.StreamInfo("moderation_events")
	require.NoError(t, err)
	require.Equal(t, uint64(1), info.State.Msgs)
}
