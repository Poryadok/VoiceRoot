package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/searchprincipal"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/partyauthority"
	"voice/backend/pkg/principal"
)

// Native handler fixtures use real signed-subject verification and the actual
// source schema. The explicit snapshot fixture is not Voice coverage evidence.
func startNativeSearchTestDB(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	var pool *pgxpool.Pool
	if dsn := os.Getenv("BE011_TEST_PG_DSN"); dsn != "" {
		admin, err := pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		name := "mm_search_" + uuid.NewString()[:8]
		_, err = admin.Exec(ctx, `CREATE DATABASE `+name)
		require.NoError(t, err)
		cfg, err := pgxpool.ParseConfig(dsn)
		require.NoError(t, err)
		cfg.ConnConfig.Database = name
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(context.Background(), `DROP DATABASE `+name); admin.Close() })
		store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	} else {
		pool = startDB(t, ctx)
	}
	for _, name := range []string{"000019_a3_party_search.up.sql", "000022_party_member_authority.up.sql"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "matchmaking_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}

type nativeSearchFixtureReader struct{}

func (nativeSearchFixtureReader) Snapshot(_ context.Context, s partysource.Subject) (partysource.Snapshot, error) {
	return partysource.Snapshot{ProtocolVersion: 1, Kind: "SOLO", MemberProfileIDs: []uuid.UUID{s.ProfileID}, MemberAuthorityVersion: 1, MemberAuthorities: []partyauthority.Member{{ProfileID: s.ProfileID, AccountID: s.AccountID, SessionEpoch: s.SessionEpoch}}, SourceAuthorityHash: [32]byte{1}}, nil
}

func startProtectedSearchForTest(t *testing.T, svc *MatchmakingGRPC, ctx context.Context, req *matchmakingv1.StartSearchRequest) (*matchmakingv1.StartSearchResponse, error) {
	t.Helper()
	profile, profileOK := authctx.ProfileID(ctx)
	account, accountOK := authctx.AccountID(ctx)
	if !profileOK || !accountOK {
		return svc.StartSearch(ctx, req)
	}
	if svc.Parties == nil && svc.Sessions != nil {
		svc.Parties = &store.PartyStore{Pool: svc.Sessions.Pool}
	}
	svc.PartySnapshots = nativeSearchFixtureReader{}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "fixture", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	if req.OperationId == "" {
		req.OperationId = uuid.NewString()
	}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	op := req.OperationId
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "matchmaking", RPC: matchmakingv1.MatchmakingService_StartSearch_FullMethodName, RequestID: op, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, ClientExpiresAt: now.Add(time.Minute)})
	require.NoError(t, err)
	v := searchprincipal.Verifier{Clock: func() time.Time { return now }, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }}
	verified, err := v.Verify(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", op)), req)
	require.NoError(t, err)
	return svc.StartSearch(verified, req)
}
