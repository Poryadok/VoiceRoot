package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
)

type memberAuthorityReader interface {
	ReadMemberAdmissionAuthority(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error)
}

func TestMemberAuthorityReadInitialLiveZeroAndMissingAggregateHistory(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner, profile := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Initial lifecycle", "", "public")
	require.NoError(t, err)
	_, err = st.JoinSpace(ctx, space.ID, profile, uuid.New())
	require.NoError(t, err)
	for _, name := range []string{"000015_audit_ledger.up.sql", "000022_lifecycle_runtime.up.sql", "000023_lifecycle_evidence_retention.up.sql", "000024_authority_revision.up.sql", "000030_entry_policy.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	applyMemberAuthorityIfPresent(t, st)
	_, err = pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(33,false)`)
	require.NoError(t, err)
	reader, ok := any(st).(memberAuthorityReader)
	require.True(t, ok, "native authority must query one owned-DB snapshot rather than reuse a Federation export")
	req := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: profile.String()}
	first, err := reader.ReadMemberAdmissionAuthority(ctx, req)
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE, first.GetLifecycle().GetState())
	require.Zero(t, first.GetLifecycle().GetGeneration(), "first P3 schedule still starts at1")
	require.EqualValues(t, 1, first.GetOwnership().GetGeneration())
	require.Equal(t, owner.String(), first.GetOwnership().GetCurrentOwnerProfileId())
	require.Positive(t, first.GetSpaceAuthorityRevision())
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_LEGACY_MIGRATED, first.GetOrdinary().GetPublicationKind())
	require.True(t, first.GetOrdinary().GetExplicitLegacy())
	require.Empty(t, first.GetOrdinary().GetRoleReceiptSha256())
	require.Nil(t, first.GetOrdinary().GetNativePublication())
	again, err := reader.ReadMemberAdmissionAuthority(ctx, req)
	require.NoError(t, err)
	require.Equal(t, first.GetSpaceAuthorityRevision(), again.GetSpaceAuthorityRevision(), "authority reads are read-only")
	require.Equal(t, first.GetOrdinary().GetPublishedBinding().String(), again.GetOrdinary().GetPublishedBinding().String())
	// Frozen actual172 migration, not merely a pretend higher version. Source:
	// 2c133336d8745d44ea382932463c8707ae8dfed0, Space34 additive participant11.
	compatible, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/space/internal/store/testdata/member_authority/space34-realtime.sql"))
	require.NoError(t, err)
	digest := sha256.Sum256(compatible)
	require.Equal(t, "476d3483e58db7aa9b95cb807f64ee58ffa6e7f81c62c453d825913a3134ca17", hex.EncodeToString(digest[:]))
	_, err = pool.Exec(ctx, string(compatible))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET version=34`)
	require.NoError(t, err)
	_, err = reader.ReadMemberAdmissionAuthority(ctx, req)
	require.NoError(t, err, "compatible additive participant migration does not invalidate native membership facts")
	applyJoinAttemptMigration(t, st)
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET version=35`)
	require.NoError(t, err)
	_, err = reader.ReadMemberAdmissionAuthority(ctx, req)
	require.NoError(t, err, "reviewed35 private journal is explicitly qualified with native33 and participant34")
	for _, future := range []int64{36, 999} {
		_, err = pool.Exec(ctx, `UPDATE schema_migrations SET version=$1`, future)
		require.NoError(t, err)
		_, err = reader.ReadMemberAdmissionAuthority(ctx, req)
		require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "unknown future migration requires actual source qualification")
	}
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET version=35`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE space_ordinary_member_heads DISABLE TRIGGER ordinary_member_head_guard`)
	require.NoError(t, err)
	_, err = reader.ReadMemberAdmissionAuthority(ctx, req)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "a clean version cannot compensate for weakened owned head guards")
	_, err = pool.Exec(ctx, `ALTER TABLE space_ordinary_member_heads ENABLE TRIGGER ordinary_member_head_guard`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO space_lifecycle_operations(operation_id,account_id,actor_profile_id,space_id,session_epoch,method,
	 request_sha256,confirmation_name_sha256,proof_digest_sha256,binding_sha256,state)
	 VALUES($1,$2,$3,$4,1,'DELETE',$5,$5,$5,$5,'SCHEDULE_PENDING')`, uuid.New(), uuid.New(), owner, space.ID, make([]byte, 32))
	require.NoError(t, err)
	_, err = reader.ReadMemberAdmissionAuthority(ctx, req)
	require.Error(t, err, "missing aggregate plus durable history is unknown, never initial LIVE0")
	_, err = pool.Exec(ctx, `INSERT INTO space_lifecycle_aggregates(space_id,deletion_operation_id,phase,generation) VALUES($1,$2,'LIVE',1)`, space.ID, uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO space_permanent_purge_fences(space_id,deletion_operation_id,purge_generation,purged_at) VALUES($1,$2,3,clock_timestamp())`, space.ID, uuid.New())
	require.NoError(t, err)
	terminal, err := reader.ReadMemberAdmissionAuthority(ctx, req)
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_RETIRED, terminal.GetLifecycle().GetState(), "permanent retirement dominates a recreated LIVE aggregate")
	require.EqualValues(t, 3, terminal.GetLifecycle().GetGeneration())
}
