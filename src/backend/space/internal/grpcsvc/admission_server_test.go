package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

type admissionSubjectFixture func(context.Context, *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error)

func (f admissionSubjectFixture) ObserveSpaceAdmissionSubject(ctx context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest, _ ...grpc.CallOption) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
	return f(ctx, r)
}

// This fixture is explicitly historical storage, not a production Creator or
// native admission bypass. Public method authentication below uses the real
// issuer, verifier and private capability; remote owner responses are controlled.
func admissionReadStore(t *testing.T) (*store.SpaceStore, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BE010_TEST_DATABASE_DSN")
	require.NotEmpty(t, dsn, "owned Linux PostgreSQL fixture required; never skip")
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	name := "be010_rpc_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		admin.Close()
	})
	applySpaceMigration(t, ctx, pool)
	for _, file := range []string{"000030_entry_policy.up.sql", "000035_join_attempts.up.sql", "000036_join_operation_namespace.up.sql", "000037_join_request_groups_forward.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", file))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	st := &store.SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Read fixture", "", "public")
	require.NoError(t, err)
	return st, space.ID
}

func TestAdmissionServer_GetAttemptUsesPrivateIngressAndFreshOwner(t *testing.T) {
	ctx := context.Background()
	st, space := admissionReadStore(t)
	account, profile := uuid.New(), uuid.New()
	a, err := st.StartJoinAttempt(ctx, store.JoinAttemptInput{SpaceID: space, AccountID: account, ProfileID: profile})
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	var elapsed atomic.Int64
	clock := func() time.Time { return now.Add(time.Duration(elapsed.Load())) }
	gatewayIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	ownerCalls := 0
	var signalOwner atomic.Bool
	ownerObserved := make(chan struct{}, 1)
	subjects := &admissionauthority.Client{Issuer: spaceIssuer, Clock: clock, Auth: admissionSubjectFixture(func(ctx context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		ownerCalls++
		if signalOwner.Load() {
			ownerObserved <- struct{}{}
		}
		require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ, r.Phase)
		require.Equal(t, authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, r.SubjectRole)
		require.Equal(t, space.String(), r.SpaceId)
		require.EqualValues(t, 7, r.GetInteractiveSessionEpoch())
		md, _ := metadata.FromOutgoingContext(ctx)
		transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(ctx, md))
		require.NoError(t, err)
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "auth", ExpectedRPC: authv1.AuthService_ObserveSpaceAdmissionSubject_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock})
		require.NoError(t, err)
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, err)
		digest := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: digest[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	server := NewAdmissionServer(&SpaceGRPC{Store: st}, st, AdmissionDependencies{Subjects: subjects})
	request := &spacev1.GetJoinAttemptRequest{SpaceId: space.String(), AttemptId: a.ID.String()}
	call := func(actorAccount, actorProfile uuid.UUID) (*spacev1.GetJoinAttemptResponse, error) {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := gatewayIssuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_GetJoinAttempt_FullMethodName, RequestID: id, RequestHash: hash, AccountID: actorAccount.String(), ProfileID: actorProfile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
		require.NoError(t, err)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
		response, err := verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetJoinAttempt_FullMethodName}, func(ctx context.Context, _ any) (any, error) { return server.GetJoinAttempt(ctx, request) })
		if err != nil {
			return nil, err
		}
		return response.(*spacev1.GetJoinAttemptResponse), nil
	}
	view, err := call(account, profile)
	require.NoError(t, err)
	require.Equal(t, a.ID.String(), view.Attempt.AttemptId)
	require.Equal(t, spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_COLLECTING, view.Attempt.Result.State)
	require.Equal(t, 1, ownerCalls)
	_, err = call(uuid.New(), profile)
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = server.GetJoinAttempt(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	// Owner reads happen before the actual waiting Space fence. Equality at
	// five seconds cannot disclose a snapshot after that SQL wait.
	locked, release := make(chan struct{}), make(chan struct{})
	blocker := make(chan error, 1)
	go func() {
		_, err := st.ReadOwnJoinAttempt(ctx, space, a.ID, account, profile, func() error { close(locked); <-release; return nil })
		blocker <- err
	}()
	<-locked
	signalOwner.Store(true)
	done := make(chan error, 1)
	go func() { _, err := call(account, profile); done <- err }()
	select {
	case <-ownerObserved:
	case <-time.After(3 * time.Second):
		t.Fatal("owner observation did not precede held Space read")
	}
	elapsed.Store(int64(5 * time.Second))
	close(release)
	require.NoError(t, <-blocker)
	require.Equal(t, codes.Unavailable, status.Code(<-done))
}
