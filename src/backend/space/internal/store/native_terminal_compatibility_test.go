package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNativeTerminal47PreservesCurrentWritersAnd46Receipts(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL16/18 and signed native writers")
	}
	for _, major := range []int{16, 18} {
		t.Run(fmt.Sprint(major), func(t *testing.T) {
			ctx := context.Background()
			db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), fmt.Sprintf("postgres:%d-alpine", major))
			db.Run(true, "goto", "34")
			admin := &SpaceStore{Pool: db.Pool}
			legacy, err := admin.CreateSpace(ctx, uuid.New(), "retained46", "", "public")
			require.NoError(t, err)
			db.Run(true, "goto", "46")
			_, err = admin.TransitionNativeMembershipCatalog46(ctx)
			require.NoError(t, err)
			require.NoError(t, admin.InitializeNativeMembership(ctx, legacy.ID))
			serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, admin.Pool)}
			call := signedNativeBotCaller(t, serving)
			actor := uuid.NewString()
			add := &spacev1.AddBotMemberRequest{SpaceId: legacy.ID.String(), ProfileId: actor, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: legacy.ID.String(), ActorProfileId: actor, Generation: 1}}
			_, err = call(add)
			require.NoError(t, err)
			root := sha256.Sum256([]byte(uuid.NewString()))
			removal := nativeGlobalRequest(nativeGlobalCancel(add), uuid.NewString(), root[:])
			receipt, err := call(removal)
			require.NoError(t, err)
			var retainedBefore, retainedAfter string
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT jsonb_build_array(
 (SELECT to_jsonb(t) FROM native_membership_guard.catalog46_transition t),
 (SELECT to_jsonb(t) FROM native_membership_guard.bot_removal_receipts t WHERE operation_id=$1),
 (SELECT cutover_id FROM native_membership_guard.spaces WHERE space_id=$2))::text`, removal.OperationId, legacy.ID).Scan(&retainedBefore))
			db.Run(true, "up", "1")
			_, err = call(removal)
			require.Error(t, err, "unadmitted47 cannot replay a saved46 positive receipt")
			transition, err := admin.TransitionNativeMembershipCatalog47(ctx)
			require.NoError(t, err)
			require.True(t, transition.Changed)
			replayed, err := call(removal)
			require.NoError(t, err)
			require.True(t, proto.Equal(receipt, replayed))
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT jsonb_build_array(
 (SELECT to_jsonb(t) FROM native_membership_guard.catalog46_transition t),
 (SELECT to_jsonb(t) FROM native_membership_guard.bot_removal_receipts t WHERE operation_id=$1),
 (SELECT cutover_id FROM native_membership_guard.spaces WHERE space_id=$2))::text`, removal.OperationId, legacy.ID).Scan(&retainedAfter))
			require.Equal(t, retainedBefore, retainedAfter)
			again, err := admin.TransitionNativeMembershipCatalog47(ctx)
			require.NoError(t, err)
			require.False(t, again.Changed)
			require.Equal(t, transition.OldSealSHA256, again.OldSealSHA256)
			require.Equal(t, transition.NewSealSHA256, again.NewSealSHA256)
			create, _, owner := signedNativeCreatorCurrent(t, serving, &spacev1.CreateSpaceRequest{Name: "born47", Visibility: "private"})
			born, err := create()
			require.NoError(t, err)
			t.Run("nickname writer", func(t *testing.T) {
				var before, after string
				require.NoError(t, db.Pool.QueryRow(ctx, `SELECT to_jsonb(h)::text FROM public.space_ordinary_member_heads h WHERE space_id=$1 AND profile_id=$2`, born.ID, owner).Scan(&before))
				member, err := serving.UpdateMemberNickname(ctx, born.ID, owner, owner, "owner alias")
				require.NoError(t, err, "actual ordinary nickname writer must participate in the guarded full cohort")
				require.Equal(t, "owner alias", *member.Nickname)
				require.NoError(t, db.Pool.QueryRow(ctx, `SELECT to_jsonb(h)::text FROM public.space_ordinary_member_heads h WHERE space_id=$1 AND profile_id=$2`, born.ID, owner).Scan(&after))
				require.Equal(t, before, after, "metadata does not republish or rebind the native source")
				var committed bool
				require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.audit_log WHERE space_id=$1 AND actor_profile_id=$2 AND action='member_nickname_updated')
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$1)
 AND EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND boundary_state='KNOWN' AND generation=1)`, born.ID, owner).Scan(&committed))
				require.True(t, committed)
			})
			observation, err := serving.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: born.ID.String(), ProfileId: owner.String()})
			require.NoError(t, err)
			require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE, observation.GetOrdinary().GetState())
			late := proto.Clone(add).(*spacev1.AddBotMemberRequest)
			late.SpaceId, late.InstallationIntent.SpaceId = born.ID.String(), born.ID.String()
			late.InstallationIntent.Generation++
			late.InstallationIntent.OperationId, late.InstallationIntent.InstallationId = uuid.NewString(), uuid.NewString()
			_, err = call(late)
			require.Error(t, err, "the retained46 global terminal still denies new generation/new Space under47")
			for name, corrupt := range map[string]string{
				"dirty":               `UPDATE public.schema_migrations SET dirty=true`,
				"future":              `UPDATE public.schema_migrations SET version=48`,
				"disabled guard":      `ALTER TABLE public.space_lifecycle_participants DISABLE TRIGGER a000_native_membership_capture`,
				"changed helper body": `CREATE OR REPLACE FUNCTION native_membership_guard.terminal_varint(v bigint) RETURNS bytea LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$ BEGIN RETURN ''::bytea; END $$`,
			} {
				t.Run(name, func(t *testing.T) {
					tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
					require.NoError(t, err)
					defer tx.Rollback(ctx)
					_, err = tx.Exec(ctx, corrupt)
					require.NoError(t, err)
					require.Error(t, checkMemberAuthorityCatalog(ctx, tx), "fixed47 qualification cannot learn/skip a changed closure")
				})
			}
			// Removing this admitted closure would destroy its proof, even with no
			// terminal compaction performed yet.
			db.Run(false, "down", "1")
			var count int
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.catalog47_transition`).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}
