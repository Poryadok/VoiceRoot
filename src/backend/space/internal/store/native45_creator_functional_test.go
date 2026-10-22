package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"path/filepath"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
)

// Helpers reproduce reviewed9b2 implementations (renamed only). Auth floor is
// controlled; this test makes no remote Auth/Role or public assembly claim.
func signedProjectionCreator(t *testing.T, st *SpaceStore, request *spacev1.CreateSpaceRequest) (func() (*SpaceRow, error), uuid.UUID, uuid.UUID) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, profile := uuid.New(), uuid.New()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	requestID := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	v := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, CreatorEpoch: func(context.Context, string, int64) error { return nil }, CreatorReplay: func(ctx context.Context, issuer, jti string, expiry time.Time) error {
		err := st.ClaimCreatorPrincipalReplay(ctx, issuer, jti, expiry)
		if errors.Is(err, ErrMemberAuthorityUnavailable) {
			return lifecycleprincipal.ErrDependencyUnavailable
		}
		return err
	}}
	return func() (*SpaceRow, error) {
		result, err := v.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateSpace_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
			creator, err := lifecycleprincipal.RequireCreator(ctx, request)
			if err != nil {
				return nil, err
			}
			return st.CreateSpaceWithCreator(ctx, creator, request)
		})
		if err != nil {
			return nil, err
		}
		return result.(*SpaceRow), nil
	}, account, profile
}

func projectionServingLogin(t *testing.T, ctx context.Context, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	role := "readiness_" + uuid.New().String()[:8]
	_, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD 'isolated-fixture-only'")
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{role}.Sanitize()+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = "isolated-fixture-only"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		_, _ = admin.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	var actual string
	require.NoError(t, pool.QueryRow(ctx, "SELECT session_user").Scan(&actual))
	require.Equal(t, role, actual)
	return pool
}

func TestNative45ProjectedCreatorPG18(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	db.Run(true, "up")
	var version int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version))
	require.EqualValues(t, 45, version)
	serving := projectionServingLogin(t, ctx, db.Pool)
	st := &SpaceStore{Pool: serving}
	// A fresh trusted installer is required; a pending45 seal cannot serve.
	before, _, _ := signedProjectionCreator(t, st, &spacev1.CreateSpaceRequest{Name: "unsealed45"})
	_, err := before()
	require.Error(t, err)
	receipt, err := (&SpaceStore{Pool: db.Pool}).TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err, "actual trusted current-source45 transition")
	require.True(t, receipt.Changed)
	create, account, profile := signedProjectionCreator(t, st, &spacev1.CreateSpaceRequest{Name: "projected45", Visibility: "private"})
	row, err := create()
	require.NoError(t, err, "signed5594 capability must publish born native45 atomically")
	require.NotNil(t, row)
	var state, boundary, registry string
	var kind, generation, members int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT h.publication_kind,h.generation,h.state,e.boundary_state,g.state,s.member_count FROM public.space_ordinary_member_heads h JOIN public.space_effective_member_heads e USING(space_id,profile_id) JOIN native_membership_guard.spaces g USING(space_id) JOIN public.spaces s ON s.id=h.space_id WHERE h.space_id=$1 AND h.profile_id=$2`, row.ID, profile).Scan(&kind, &generation, &state, &boundary, &registry, &members))
	require.Equal(t, 3, kind)
	require.Equal(t, 1, generation)
	require.Equal(t, "ACTIVE", state)
	require.Equal(t, "KNOWN", boundary)
	require.Equal(t, "COMPLETE", registry)
	require.Equal(t, 1, members)
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_native_creator_operations WHERE space_id=$1 AND account_id=$2 AND profile_id=$3`, row.ID, account, profile).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames`).Scan(&count))
	require.Zero(t, count)
	_, err = create()
	require.Error(t, err, "durable JTI replay cannot create another Space")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.spaces`).Scan(&count))
	require.Equal(t, 1, count)
	t.Run("owner session is not serving", func(t *testing.T) {
		bad, _, _ := signedProjectionCreator(t, &SpaceStore{Pool: db.Pool}, &spacev1.CreateSpaceRequest{Name: "owner forbidden"})
		_, err := bad()
		require.Error(t, err)
	})
	t.Run("real constraint rollback and inert credential retry", func(t *testing.T) {
		// Existing M1 CHECK rejects this at the actual INSERT, without altering
		// any catalog pin or installing a test-only source trigger.
		fail, _, p := signedProjectionCreator(t, st, &spacev1.CreateSpaceRequest{Name: "rollback45", Visibility: "invalid"})
		_, err = fail()
		require.Error(t, err)
		var check *pgconn.PgError
		require.ErrorAs(t, err, &check, "failure must reach the existing INSERT constraint")
		require.Equal(t, "23514", check.Code)
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_ordinary_member_heads WHERE profile_id=$1`, p).Scan(&count))
		require.Zero(t, count)
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames`).Scan(&count))
		require.Zero(t, count)
		_, err = fail()
		require.Error(t, err, "credential replay remains denied after business rollback")
	})
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.spaces`).Scan(&count))
	require.Equal(t, 1, count)
}
