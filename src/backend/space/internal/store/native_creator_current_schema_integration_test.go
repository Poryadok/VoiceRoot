//go:build spacebanintegration

package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	spacev1 "voice.app/voice/space/v1"
)

// Run each entrypoint only on its own controller-owned genuine trusted fixture:
// real34->44->45/Transition45->50/Transition50, then the qualified production
// NewNativeCatalog51Migration/CommitQualified pipeline for51. This test neither
// constructs nor adopts that history; Inspect authenticates its current source.
func TestNative50CreatorBornCurrentSchema(t *testing.T) { testCreatorBornCurrentSchema(t, 50) }
func TestNative51CreatorBornCurrentSchema(t *testing.T) { testCreatorBornCurrentSchema(t, 51) }

func creatorBornCheck(t *testing.T, check string, ok bool) {
	t.Helper()
	if !ok {
		t.Fatalf("CREATOR_BORN_V1 check=%s result=FAIL", check)
	}
	t.Logf("CREATOR_BORN_V1 check=%s result=PASS", check)
}

func testCreatorBornCurrentSchema(t *testing.T, version int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	creatorBornCheck(t, "OWNED_INPUT", dsn != "")
	admin, err := pgxpool.New(ctx, dsn)
	creatorBornCheck(t, "POOL", err == nil)
	t.Cleanup(admin.Close)
	facts, err := (&SpaceStore{Pool: admin}).InspectNativeInstallerSource(ctx, version)
	creatorBornCheck(t, "QUALIFIED_SOURCE", err == nil && facts != nil && facts.Version == version)
	serving := projectionServingLogin(t, ctx, admin)
	st := &SpaceStore{Pool: serving}

	create, account, profile := signedProjectionCreator(t, st, &spacev1.CreateSpaceRequest{Name: "current native creator", Visibility: "private"})
	row, err := create()
	creatorBornCheck(t, "COMMIT", err == nil && row != nil)
	var kind, generation, members int
	var state, boundary, registry string
	err = admin.QueryRow(ctx, `SELECT h.publication_kind,h.generation,h.state,e.boundary_state,g.state,s.member_count
 FROM public.space_ordinary_member_heads h JOIN public.space_effective_member_heads e USING(space_id,profile_id)
 JOIN native_membership_guard.spaces g USING(space_id) JOIN public.spaces s ON s.id=h.space_id
 WHERE h.space_id=$1 AND h.profile_id=$2 AND s.owner_profile_id=$2`, row.ID, profile).Scan(&kind, &generation, &state, &boundary, &registry, &members)
	creatorBornCheck(t, "OWNER_EFFECTIVE_COMPLETE", err == nil && kind == 3 && generation == 1 && state == "ACTIVE" && boundary == "KNOWN" && registry == "COMPLETE" && members == 1)
	var operations, frames int
	err = admin.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.space_native_creator_operations
 WHERE space_id=$1 AND account_id=$2 AND profile_id=$3 AND session_epoch=7 AND octet_length(request_sha256)=32
 AND published_at>=credential_issued_at AND published_at<credential_expires_at),
 (SELECT count(*) FROM native_membership_guard.frames)`, row.ID, account, profile).Scan(&operations, &frames)
	creatorBornCheck(t, "WITNESS_AND_DRAIN", err == nil && operations == 1 && frames == 0)
	_, err = create()
	creatorBornCheck(t, "REPLAY_DENIED", err != nil)
	var count int
	err = admin.QueryRow(ctx, `SELECT count(*) FROM public.spaces WHERE owner_profile_id=$1`, profile).Scan(&count)
	creatorBornCheck(t, "NO_REPLAY_SPACE", err == nil && count == 1)

	installerTx, err := admin.Begin(ctx)
	creatorBornCheck(t, "INSTALLER_TX", err == nil)
	err = requireBornCreatorServingRole(ctx, installerTx)
	creatorBornCheck(t, "INSTALLER_ROLE_DENIED", errors.Is(err, ErrMemberAuthorityUnavailable))
	creatorBornCheck(t, "INSTALLER_TX_ROLLBACK", installerTx.Rollback(ctx) == nil)
	denied, _, deniedProfile := signedProjectionCreator(t, &SpaceStore{Pool: admin}, &spacev1.CreateSpaceRequest{Name: "installer cannot serve"})
	_, err = denied()
	creatorBornCheck(t, "INSTALLER_DENIED", errors.Is(err, ErrMemberAuthorityUnavailable) || errors.Is(err, ErrAccountBanUnavailable))
	creatorBornNoEffects(t, ctx, admin, deniedProfile)

	fail, _, failedProfile := signedProjectionCreator(t, st, &spacev1.CreateSpaceRequest{Name: "constraint rollback", Visibility: "invalid"})
	_, err = fail()
	var constraint *pgconn.PgError
	creatorBornCheck(t, "REAL_INSERT_CONSTRAINT", errors.As(err, &constraint) && constraint.Code == "23514")
	creatorBornNoEffects(t, ctx, admin, failedProfile)
	_, err = fail()
	creatorBornCheck(t, "ROLLED_BACK_CREDENTIAL_REPLAY_DENIED", err != nil)
	creatorBornNoEffects(t, ctx, admin, failedProfile)
	// The fixture creates and grants a temporary login after installation. Remove
	// those test-only grants before requalifying the original installer catalog.
	role := pgx.Identifier{serving.Config().ConnConfig.User}.Sanitize()
	serving.Close()
	_, err = admin.Exec(ctx, "DROP OWNED BY "+role+"; DROP ROLE "+role)
	creatorBornCheck(t, "SERVING_FIXTURE_CLEANUP", err == nil)
	_, err = (&SpaceStore{Pool: admin}).InspectNativeInstallerSource(ctx, version)
	creatorBornCheck(t, "SOURCE_REMAINS_QUALIFIED", err == nil)
}

func creatorBornNoEffects(t *testing.T, ctx context.Context, pool *pgxpool.Pool, profile uuid.UUID) {
	t.Helper()
	var spaces, heads, effective, operations, frames int
	err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.spaces WHERE owner_profile_id=$1),
 (SELECT count(*) FROM public.space_ordinary_member_heads WHERE profile_id=$1),
 (SELECT count(*) FROM public.space_effective_member_heads WHERE profile_id=$1),
 (SELECT count(*) FROM public.space_native_creator_operations WHERE profile_id=$1),
 (SELECT count(*) FROM native_membership_guard.frames)`, profile).Scan(&spaces, &heads, &effective, &operations, &frames)
	creatorBornCheck(t, "NO_PARTIAL_EFFECTS", err == nil && spaces == 0 && heads == 0 && effective == 0 && operations == 0 && frames == 0)
}
