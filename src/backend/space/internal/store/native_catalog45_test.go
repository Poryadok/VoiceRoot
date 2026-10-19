package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNative45FixedCatalogQualification(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Exact45 upgrade", "", "public")
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, st.InitializeNativeMembership(ctx, space.ID))
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	request := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: owner.String()}
	_, err = serving.ReadMemberAdmissionAuthority(ctx, request)
	require.NoError(t, err, "actual44 initialized serving baseline")
	fresh, err := serving.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: uuid.NewString()})
	require.NoError(t, err, "prospective Bot profile has qualified NONE facts before any Add")
	require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE, fresh.Ordinary.State)
	require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE, fresh.Community.State)
	create, _, _ := signedNativeCreatorCurrent(t, serving, &spacev1.CreateSpaceRequest{Name: "Known44 before45", Visibility: "private"})
	_, err = create()
	require.NoError(t, err, "real born44 KNOWN history coexists with initialized UNKNOWN")
	const snapshotSQL = `SELECT jsonb_build_object(
	 'registry',(SELECT jsonb_agg(to_jsonb(s) ORDER BY s.space_id) FROM native_membership_guard.spaces s),
	 'heads',(SELECT jsonb_agg(to_jsonb(h) ORDER BY h.space_id,h.profile_id) FROM public.space_effective_member_heads h))::text`
	var before, after string
	var oldSeal []byte
	require.NoError(t, db.Pool.QueryRow(ctx, snapshotSQL).Scan(&before))
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT catalog_sha256 FROM native_membership_guard.epoch WHERE singleton`).Scan(&oldSeal))
	db.Run(true, "goto", "45")
	_, err = serving.ReadMemberAdmissionAuthority(ctx, request)
	require.Error(t, err, "new helper invalidates retained44 seal until fixed continuous transition")
	require.Error(t, nativeReadinessTx(ctx, serving.Pool, space.ID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, space.ID)
		return err
	}), "actual SQL gate refuses stale44 seal independently of Go's version selector")
	var structure string
	require.NoError(t, db.Pool.QueryRow(ctx, nativeReadinessStructureSQL).Scan(&structure))
	t.Logf("actual45 PostgreSQL16 canonical structure SHA256=%x", sha256.Sum256([]byte(structure)))
	require.NoError(t, os.WriteFile("/tmp/native45-structure.json", []byte(structure), 0600))
	require.NoError(t, nativeReadinessTx(ctx, db.Pool, space.ID, func(tx pgx.Tx) error {
		return checkMemberAuthorityCatalog(ctx, tx)
	}), "fixed45 native facts must qualify independently of the still-unavailable serving seal")
	for _, mutation := range []struct{ name, sql string }{
		{"future46", `UPDATE public.schema_migrations SET version=46`},
		{"dirty", `UPDATE public.schema_migrations SET dirty=true`},
		{"new_guard_disabled", `ALTER TABLE public.space_join_operations DISABLE TRIGGER space_join_legacy_selection_immutable`},
		{"native_capture_disabled", `ALTER TABLE public.space_members DISABLE TRIGGER a000_native_membership_capture`},
		{"new_guard_body", `CREATE OR REPLACE FUNCTION public.guard_join_legacy_selection_immutable() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NEW; END'`},
		{"new_constraint_missing", `ALTER TABLE public.space_join_operations DROP CONSTRAINT space_join_legacy_selection_pair`},
		{"private_schema_acl", `GRANT USAGE ON SCHEMA native_membership_guard TO PUBLIC`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			tx, err := db.Pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(ctx)
			_, err = tx.Exec(ctx, mutation.sql)
			require.NoError(t, err)
			require.ErrorIs(t, checkMemberAuthorityCatalog(ctx, tx), ErrMemberAuthorityUnavailable)
		})
	}
	_, err = serving.TransitionNativeMembershipCatalog45(ctx)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "serving identity cannot invoke installer reseal")
	_, err = db.Pool.Exec(ctx, `CREATE FUNCTION public.unqualified_transition_extension() RETURNS integer LANGUAGE sql AS 'SELECT 1'`)
	require.NoError(t, err)
	_, err = st.TransitionNativeMembershipCatalog45(ctx)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "an extra helper cannot enter the reconstructed old closure")
	_, err = db.Pool.Exec(ctx, `DROP FUNCTION public.unqualified_transition_extension()`)
	require.NoError(t, err)
	wrong := append([]byte(nil), oldSeal...)
	wrong[0] ^= 1
	_, err = db.Pool.Exec(ctx, `UPDATE native_membership_guard.epoch SET catalog_sha256=$1 WHERE singleton`, wrong)
	require.NoError(t, err)
	_, err = st.TransitionNativeMembershipCatalog45(ctx)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "wrong baseline is not self-resealed")
	// Controlled test corruption only: no source/guard was changed or serving
	// write permitted. Restore the actual retained baseline for the positive.
	_, err = db.Pool.Exec(ctx, `UPDATE native_membership_guard.epoch SET catalog_sha256=$1 WHERE singleton`, oldSeal)
	require.NoError(t, err)
	receipt, err := st.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err, "fixed trusted44 baseline plus exact migration must transition without recutover")
	require.True(t, receipt.Changed)
	require.Equal(t, hex.EncodeToString(oldSeal), receipt.OldSealSHA256)
	require.NotEqual(t, receipt.OldSealSHA256, receipt.NewSealSHA256)
	require.NotEmpty(t, receipt.DatabaseSystemID)
	require.NotEmpty(t, receipt.TransactionID)
	require.Equal(t, "f424f4f8afbf5a2a0aeb0530a1987ada0db6046a76aad769f736f19dce39ff62", receipt.MigrationSHA256)
	_, err = serving.ReadMemberAdmissionAuthority(ctx, request)
	require.NoError(t, err, "actual serving45 getter consumes new seal")
	require.NoError(t, db.Pool.QueryRow(ctx, snapshotSQL).Scan(&after))
	require.Equal(t, before, after, "UNKNOWN/KNOWN registry/head/history bytes are preserved")
	replay, err := st.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	require.False(t, replay.Changed)
	require.Equal(t, receipt.OldSealSHA256, replay.OldSealSHA256)
	require.Equal(t, receipt.NewSealSHA256, replay.NewSealSHA256)
	raw, err := json.MarshalIndent(receipt, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("/tmp/native45-transition-receipt.json", raw, 0600))
}

func TestNative45FreshInstallBornPublication(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "up")
	admin := &SpaceStore{Pool: db.Pool}
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	create, _, profile := signedNativeCreatorCurrent(t, serving, &spacev1.CreateSpaceRequest{Name: "Fresh45 born publication", Visibility: "private"})
	_, err := create()
	require.Error(t, err, "fresh45 also needs the authenticated installer transition")
	receipt, err := admin.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	require.True(t, receipt.Changed)
	create, _, profile = signedNativeCreatorCurrent(t, serving, &spacev1.CreateSpaceRequest{Name: "Fresh45 qualified publication", Visibility: "private"})
	space, err := create()
	require.NoError(t, err)
	facts, err := serving.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: profile.String()})
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE, facts.Ordinary.PublicationKind)
	var complete bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND state='COMPLETE')
 AND EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE' AND boundary_state='KNOWN')
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, space.ID, profile).Scan(&complete))
	require.True(t, complete)
}

func TestNative45TransitionSerializesActualWriter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	space, err := admin.CreateSpace(ctx, uuid.New(), "Transition concurrent source loss", "", "public")
	require.NoError(t, err)
	target := uuid.New()
	_, err = db.Pool.Exec(ctx, `INSERT INTO public.space_members(space_id,profile_id) VALUES($1,$2);`, space.ID, target)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `UPDATE public.spaces SET member_count=2 WHERE id=$1`, space.ID)
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	db.Run(true, "goto", "45")
	blocker, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(ctx)
	_, err = blocker.Exec(ctx, `SELECT singleton FROM native_membership_guard.epoch WHERE singleton FOR UPDATE`)
	require.NoError(t, err)
	type result struct {
		receipt *NativeMembershipCatalog45Receipt
		err     error
	}
	transition := make(chan result, 1)
	go func() {
		receipt, err := admin.TransitionNativeMembershipCatalog45(ctx)
		transition <- result{receipt, err}
	}()
	require.Eventually(t, func() bool {
		var locked bool
		err := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='public.schema_migrations'::regclass AND mode='AccessExclusiveLock' AND granted)`).Scan(&locked)
		return err == nil && locked
	}, 3*time.Second, 10*time.Millisecond, "installer holds the actual catalog barrier before seal CAS")
	writer := make(chan error, 1)
	go func() { writer <- serving.RemoveMember(ctx, space.ID, target) }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation IN ('public.schema_migrations'::regclass,'public.ownership_journal'::regclass,'public.space_lifecycle_aggregates'::regclass) AND mode='AccessShareLock' AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond, "real source writer waits behind installer, never bypasses stale seal")
	require.NoError(t, blocker.Commit(ctx))
	sealed := <-transition
	require.NoError(t, sealed.err)
	require.True(t, sealed.receipt.Changed)
	require.NoError(t, <-writer, "actual writer resumes only under the qualified new seal")
	var retired bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.space_members WHERE space_id=$1 AND profile_id=$2)
 AND EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='RETIRED' AND generation=retired_through)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$1)`, space.ID, target).Scan(&retired))
	require.True(t, retired)
}
