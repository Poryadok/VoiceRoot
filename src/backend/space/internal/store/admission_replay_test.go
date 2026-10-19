package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
)

func TestAdmissionPrincipalReplayDurableSignedIngress(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "up")
	pool := nativeReadinessLogin(t, ctx, db.Pool)
	st := &SpaceStore{Pool: pool}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	request := &spacev1.JoinSpaceRequest{SpaceId: uuid.NewString()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	requestID, account, profile := uuid.NewString(), uuid.NewString(), uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_JoinSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: account, ProfileID: profile, SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	ingress := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	verifier := func(store *SpaceStore) lifecycleprincipal.Verifier {
		return lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(ctx context.Context, issuer, jti string, expiry time.Time) error {
			err := store.ClaimAdmissionPrincipalReplay(ctx, issuer, jti, expiry)
			if errors.Is(err, ErrMemberAuthorityUnavailable) {
				return lifecycleprincipal.ErrDependencyUnavailable
			}
			return err
		}}
	}
	var calls atomic.Int32
	var acceptedJTI string
	var acceptedExpiry time.Time
	invoke := func(store *SpaceStore) error {
		_, err := verifier(store).Unary()(ingress, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_JoinSpace_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
			call, err := lifecycleprincipal.RequireAdmissionJoinSpace(ctx, request)
			if err != nil {
				return nil, err
			}
			actor, err := call.ActorFor(request)
			if err != nil {
				return nil, err
			}
			acceptedJTI, acceptedExpiry = actor.JWTID, actor.ExpiresAt
			calls.Add(1)
			return nil, nil
		})
		return err
	}
	require.NoError(t, invoke(st), "actual signed ingress needs a committed admission nonce before private capability")
	require.EqualValues(t, 1, calls.Load())
	// A new Store instance cannot accept the same credential after restart.
	require.Equal(t, codes.Unauthenticated, status.Code(invoke(&SpaceStore{Pool: pool})))
	require.EqualValues(t, 1, calls.Load())
	require.ErrorIs(t, st.ClaimCreatorPrincipalReplay(ctx, "gateway", acceptedJTI, acceptedExpiry), ErrMemberAdmissionConflict, "issuer/JTI is globally single use across Creator and admission")
	other := make([]byte, 20)
	_, err = rand.Read(other)
	require.NoError(t, err)
	jti := base64.RawURLEncoding.EncodeToString(other)
	expiry := time.Now().Add(20 * time.Second)
	secondPool, err := pgxpool.NewWithConfig(ctx, pool.Config().Copy())
	require.NoError(t, err)
	defer secondPool.Close()
	var leftPID, rightPID int
	require.NoError(t, pool.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&leftPID))
	require.NoError(t, secondPool.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&rightPID))
	require.NotEqual(t, leftPID, rightPID, "independent serving pools use distinct actual PostgreSQL sessions")
	results := make(chan error, 2)
	start := make(chan struct{})
	for _, candidate := range []*pgxpool.Pool{pool, secondPool} {
		go func(candidate *pgxpool.Pool) {
			<-start
			results <- (&SpaceStore{Pool: candidate}).ClaimAdmissionPrincipalReplay(ctx, "gateway", jti, expiry)
		}(candidate)
	}
	close(start)
	first, second := <-results, <-results
	require.True(t, (first == nil && errors.Is(second, ErrMemberAdmissionConflict)) || (second == nil && errors.Is(first, ErrMemberAdmissionConflict)), "two actual DB connections cannot reserve the same nonce twice")
	require.ErrorIs(t, st.ClaimAdmissionPrincipalReplay(ctx, "bot", jti, expiry), ErrMemberAdmissionConflict)
	require.ErrorIs(t, st.ClaimAdmissionPrincipalReplay(ctx, "gateway", "invalid", expiry), ErrMemberAdmissionConflict)
	require.ErrorIs(t, st.ClaimAdmissionPrincipalReplay(ctx, "gateway", base64.RawURLEncoding.EncodeToString(make([]byte, 20)), time.Now().Add(-time.Second)), ErrMemberAdmissionConflict)
	var rows int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_creator_principal_replays`).Scan(&rows))
	require.Equal(t, 2, rows)
	removed, err := st.CleanupAdmissionPrincipalReplays(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, removed, "shared bounded cleanup cannot delete live admission nonces")
	pool.Close()
	require.Equal(t, codes.Unavailable, status.Code(invoke(st)), "a replay storage failure is unavailable, never an in-memory fallback")
	require.EqualValues(t, 1, calls.Load())
}
