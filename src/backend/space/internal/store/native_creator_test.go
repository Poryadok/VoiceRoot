package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
)

func nativeCreatorFixture(t *testing.T) *SpaceStore {
	t.Helper()
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	for _, name := range []string{"000015_audit_ledger.up.sql", "000022_lifecycle_runtime.up.sql", "000023_lifecycle_evidence_retention.up.sql", "000024_authority_revision.up.sql", "000030_entry_policy.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	st := &SpaceStore{Pool: pool}
	applyMemberAuthorityIfPresent(t, st)
	_, err := pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(33,false)`)
	require.NoError(t, err)
	return st
}

// This adapter uses actual Gateway RS256 verification and the actual committed
// PG replay store. The Auth floor is a controlled observation, not a claim about
// an assembled Java Auth runtime or a live Role readiness receipt.
func signedNativeCreator(t *testing.T, st *SpaceStore, request *spacev1.CreateSpaceRequest) (func() (*SpaceRow, error), uuid.UUID, uuid.UUID) {
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

func TestNativeCreatorPublishesPositiveOwnerProvenanceAndKeepsFormerOwnerEpisode(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	request := &spacev1.CreateSpaceRequest{Name: "Native human owner", Visibility: "private"}
	invoke, account, profile := signedNativeCreator(t, st, request)
	space, err := invoke()
	require.NoError(t, err)
	require.Equal(t, profile, space.OwnerProfileID)
	_, err = invoke()
	require.Error(t, err, "same transport credential cannot create a second Space")
	observed, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: profile.String()})
	require.NoError(t, err)
	require.EqualValues(t, 1, observed.GetOrdinary().GetHeadGeneration())
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE, observed.GetOrdinary().GetPublicationKind())
	require.Empty(t, observed.GetOrdinary().GetRoleReceiptSha256(), "native membership must not mint default-role READY")
	require.NotNil(t, observed.GetOrdinary().GetNativePublication())
	var count int
	var storedAccount uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT account_id FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&storedAccount))
	require.Equal(t, account, storedAccount)
	_, err = st.Pool.Exec(ctx, `UPDATE space_native_creator_operations SET session_epoch=session_epoch+1 WHERE space_id=$1`, space.ID)
	require.Error(t, err)
	_, err = st.Pool.Exec(ctx, `DELETE FROM space_native_creator_operations WHERE space_id=$1`, space.ID)
	require.Error(t, err)
	newOwner := uuid.New()
	publishAdmissionFixture(t, st, space.ID, profile, newOwner)
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, space.ID, newOwner)
	require.NoError(t, err)
	after, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: profile.String()})
	require.NoError(t, err)
	require.Equal(t, newOwner.String(), after.GetOwnership().GetCurrentOwnerProfileId())
	require.EqualValues(t, 2, after.GetOwnership().GetGeneration())
	require.Equal(t, observed.GetOrdinary().String(), after.GetOrdinary().String(), "former Owner keeps the same ordinary episode; no new Member/default grant")
}

func TestNativeCreatorMemberFailureRollsBackEveryNativeWriteButKeepsReplayReservation(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	_, err := st.Pool.Exec(ctx, `CREATE FUNCTION reject_native_creator_member() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected native member failure'; END $$; CREATE TRIGGER reject_native_creator_member BEFORE INSERT ON space_members FOR EACH ROW EXECUTE FUNCTION reject_native_creator_member()`)
	require.NoError(t, err)
	invoke, _, _ := signedNativeCreator(t, st, &spacev1.CreateSpaceRequest{Name: "Rollback owner"})
	_, err = invoke()
	require.Error(t, err)
	for _, table := range []string{"spaces", "space_members", "space_native_creator_operations", "space_native_member_publications", "space_ordinary_member_heads", "space_membership_ownership_heads"} {
		var count int
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count))
		require.Zero(t, count, table)
	}
	var reservations int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_creator_principal_replays`).Scan(&reservations))
	require.Equal(t, 1, reservations)
	_, err = invoke()
	require.Error(t, err, "a failed non-idempotent POST cannot reuse the admitted transport credential")
}
