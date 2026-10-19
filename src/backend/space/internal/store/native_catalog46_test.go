package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNative46TrustedTransitionPreserves44And45(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL16/18 trusted migration transitions")
	}
	for _, major := range []int{16, 18} {
		for _, baseline := range []int64{44, 45} {
			t.Run(fmt.Sprintf("pg%d_baseline%d", major, baseline), func(t *testing.T) {
				ctx := context.Background()
				image := "postgres:16-bookworm"
				if major == 18 {
					image = "postgres:18-alpine"
				}
				db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), image)
				db.Run(true, "goto", "34")
				admin := &SpaceStore{Pool: db.Pool}
				space, err := admin.CreateSpace(ctx, uuid.New(), "Native46 preserved source", "", "public")
				require.NoError(t, err)
				db.Run(true, "goto", "44")
				require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
				db.Run(true, "goto", "45")
				if baseline == 45 {
					_, err = admin.TransitionNativeMembershipCatalog45(ctx)
					require.NoError(t, err)
				}
				const snapshotSQL = `SELECT jsonb_build_object('registry',(SELECT jsonb_agg(to_jsonb(s) ORDER BY space_id) FROM native_membership_guard.spaces s),
 'heads',(SELECT jsonb_agg(to_jsonb(h) ORDER BY space_id,profile_id) FROM public.space_effective_member_heads h),
 'ordinary',(SELECT jsonb_agg(to_jsonb(h) ORDER BY space_id,profile_id) FROM public.space_ordinary_member_heads h))::text`
				var before, after string
				var oldSeal []byte
				require.NoError(t, db.Pool.QueryRow(ctx, snapshotSQL).Scan(&before))
				require.NoError(t, db.Pool.QueryRow(ctx, `SELECT catalog_sha256 FROM native_membership_guard.epoch`).Scan(&oldSeal))
				db.Run(true, "goto", "46")
				serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
				request := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: space.OwnerProfileID.String()}
				_, err = serving.ReadMemberAdmissionAuthority(ctx, request)
				require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "UP46 alone cannot bless its old seal")
				_, err = serving.TransitionNativeMembershipCatalog46(ctx)
				require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "DDL-incapable serving LOGIN cannot reseal")
				_, err = db.Pool.Exec(ctx, `CREATE FUNCTION public.unqualified_native46_extension() RETURNS integer LANGUAGE sql AS 'SELECT 1'`)
				require.NoError(t, err)
				_, err = admin.TransitionNativeMembershipCatalog46(ctx)
				require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "unknown public helper stays in old projection and forbids transition")
				_, err = db.Pool.Exec(ctx, `DROP FUNCTION public.unqualified_native46_extension()`)
				require.NoError(t, err)
				r, err := admin.TransitionNativeMembershipCatalog46(ctx)
				require.NoError(t, err, "controlled immutable46 extension of actual trusted baseline")
				require.True(t, r.Changed)
				require.Equal(t, baseline, r.BaselineSchema)
				require.Equal(t, hex.EncodeToString(oldSeal), r.OldSealSHA256)
				require.NotEqual(t, r.OldSealSHA256, r.NewSealSHA256)
				raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000046_native_bot_global_removal.up.sql"))
				require.NoError(t, err)
				digest := sha256.Sum256(raw)
				require.Equal(t, hex.EncodeToString(digest[:]), r.MigrationSHA256)
				require.NoError(t, nativeReadinessTx(ctx, serving.Pool, space.ID, func(tx pgx.Tx) error { return checkMemberAuthorityCatalog(ctx, tx) }), "closed46 catalog metadata is readable without private schema USAGE")
				_, err = serving.ReadMemberAdmissionAuthority(ctx, request)
				require.NoError(t, err, "actual initialized nonowner serving source survives46 transition")
				fresh, err := serving.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: uuid.NewString()})
				require.NoError(t, err)
				require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE, fresh.Ordinary.State)
				require.NoError(t, db.Pool.QueryRow(ctx, snapshotSQL).Scan(&after))
				require.Equal(t, before, after, "no generation, source binding, local cutover or history rewrite")
				replay, err := admin.TransitionNativeMembershipCatalog46(ctx)
				require.NoError(t, err)
				require.False(t, replay.Changed)
				require.Equal(t, r.OldSealSHA256, replay.OldSealSHA256)
				require.Equal(t, r.NewSealSHA256, replay.NewSealSHA256)
				for _, mutation := range []string{`UPDATE public.schema_migrations SET dirty=true`, `UPDATE public.schema_migrations SET version=47`, `ALTER TABLE public.space_bot_installation_attempts DISABLE TRIGGER a001_bot_global_terminal`, `ALTER TABLE native_membership_guard.bot_terminals DISABLE TRIGGER bot_terminal_registration`, `GRANT SELECT ON native_membership_guard.bot_terminals TO PUBLIC`, `CREATE INDEX unqualified_bot_terminal_index ON native_membership_guard.bot_terminals(decision_sha256)`} {
					tx, e := db.Pool.Begin(ctx)
					require.NoError(t, e)
					_, e = tx.Exec(ctx, mutation)
					require.NoError(t, e)
					require.ErrorIs(t, checkMemberAuthorityCatalog(ctx, tx), ErrMemberAuthorityUnavailable)
					require.NoError(t, tx.Rollback(ctx))
				}
				db.Run(false, "down", "1")
				var count int
				require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.catalog46_transition`).Scan(&count))
				require.Equal(t, 1, count, "activation proof alone forbids destructive DOWN")
			})
		}
	}
}
