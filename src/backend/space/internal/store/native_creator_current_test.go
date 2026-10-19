package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
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

// This adapter uses actual Gateway RS256 verification and the actual committed
// PG replay store. The Auth floor is a controlled observation, not a claim about
// an assembled Java Auth runtime or a live Role readiness receipt.
func signedNativeCreatorCurrent(t *testing.T, st *SpaceStore, request *spacev1.CreateSpaceRequest) (func() (*SpaceRow, error), uuid.UUID, uuid.UUID) {
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

func TestNativeCreatorCurrent41PublishesEffectiveEpisodeAtomically(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real native PostgreSQL creator")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "up")
	st := &SpaceStore{Pool: db.Pool}
	create, _, profile := signedNativeCreatorCurrent(t, st, &spacev1.CreateSpaceRequest{Name: "Current41 creator", Visibility: "private"})
	space, err := create()
	require.NoError(t, err)
	var state, boundary string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state,boundary_state FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&state, &boundary), "native publication and effective history must commit together")
	require.Equal(t, "ACTIVE", state)
	require.Equal(t, "KNOWN", boundary)
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		capture, err := scoped.captureEffectiveMemberMutations(ctx, space.ID, []uuid.UUID{profile})
		require.NoError(t, err, "real current native source matches saved effective witness")
		require.NoError(t, scoped.finishEffectiveMemberMutations(ctx, capture))
		return nil
	}))
	_, err = create()
	require.Error(t, err, "credential replay cannot publish a second Space or ledger")
}
