package spacemedia

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// A migration executor may already have a snapshot when it runs a transactional
// SQL file. Writer exclusion must not make that older snapshot authoritative.
func TestNativeDownRejectsInheritedRepeatableReadSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name, migration, table, insert string
	}{
		{
			name: "roster013", migration: "000013_a3_roster_outbox", table: "voice_roster_outbox",
			insert: `INSERT INTO voice_roster_outbox(event_id,room_id,roster_version,profile_id,change,occurred_at,payload)
 VALUES($1,$2,1,$3,'JOINED',clock_timestamp(),'native-fact')`,
		},
		{
			name: "p3_014", migration: "000014_space_media_p3", table: "voice_space_media_room_heads",
			insert: `INSERT INTO voice_space_media_room_heads(voice_room_id,room_generation,space_id,call_room_id,creator_operation_id,state,lifecycle_generation)
 VALUES($1,1,$2,$3,$4,'OPEN',7)`,
		},
		{
			name: "owner015", migration: "000015_space_admission_ownership", table: "voice_space_media_admission_owners",
			insert: `INSERT INTO voice_space_media_admission_owners(operation_id,space_id,voice_room_id,owner_token,owner_epoch,lease_until)
 VALUES($1,$2,$3,$4,1,clock_timestamp()+interval '1 minute')`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			pool := startSpaceMediaAdmissionPostgres(t, ctx)
			rollback, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
			require.NoError(t, err)
			t.Cleanup(func() { _ = rollback.Rollback(ctx) })
			// Establish an old transaction snapshot without holding an access lock
			// on an ownership table that would block the concurrent writer.
			var snapshot string
			require.NoError(t, rollback.QueryRow(ctx, `SELECT txid_current_snapshot()::text`).Scan(&snapshot))
			args := []any{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
			if tc.name == "roster013" {
				args = args[:3]
			}
			if tc.name == "p3_014" {
				args[2] = uuid.NewString()
			}
			_, err = pool.Exec(ctx, tc.insert, args...)
			require.NoError(t, err, "owning writer commits after the executor snapshot")
			var expected string
			require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(t))::text FROM `+tc.table+` t`).Scan(&expected))
			down, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", tc.migration+".down.sql"))
			require.NoError(t, err)
			_, downErr := rollback.Exec(ctx, string(down))
			_ = rollback.Rollback(ctx)
			var exists bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, tc.table).Scan(&exists))
			t.Logf("snapshot=%s DOWN error=%v table retained=%v", snapshot, downErr, exists)
			require.Error(t, downErr, "DOWN must not trust an inherited old snapshot")
			require.True(t, exists)
			var actual string
			require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(t))::text FROM `+tc.table+` t`).Scan(&actual))
			require.JSONEq(t, expected, actual, "retained ownership bytes and P3 columns survive")
		})
	}
}

func TestRosterDownRetainsIndependentHistory(t *testing.T) {
	for _, history := range []string{"empty", "outbox", "counter"} {
		t.Run(history, func(t *testing.T) {
			ctx := t.Context()
			pool := startSpaceMediaAdmissionPostgres(t, ctx)
			switch history {
			case "outbox":
				_, err := pool.Exec(ctx, `INSERT INTO voice_roster_outbox(event_id,room_id,roster_version,profile_id,change,occurred_at,payload)
 VALUES($1,$2,1,$3,'JOINED',clock_timestamp(),'retained-fact')`, uuid.New(), uuid.New(), uuid.New())
				require.NoError(t, err)
			case "counter":
				_, err := pool.Exec(ctx, `INSERT INTO voice_space_media_room_heads(voice_room_id,room_generation,space_id,call_room_id,creator_operation_id,state,roster_version)
 VALUES($1,1,$2,$3,$4,'ENDED',9)`, uuid.New(), uuid.New(), uuid.NewString(), uuid.New())
				require.NoError(t, err)
			}
			body, err := os.ReadFile("../../../migrations/voice_db/000013_a3_roster_outbox.down.sql")
			require.NoError(t, err)
			downErr := execP3Down(ctx, pool, string(body))
			if history == "empty" {
				require.NoError(t, downErr)
				up, readErr := os.ReadFile("../../../migrations/voice_db/000013_a3_roster_outbox.up.sql")
				require.NoError(t, readErr)
				_, err = pool.Exec(ctx, string(up))
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, downErr, "cannot remove Voice roster history")
			var count int
			if history == "outbox" {
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox WHERE payload='retained-fact'`).Scan(&count))
			} else {
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_room_heads WHERE roster_version=9`).Scan(&count))
			}
			require.Equal(t, 1, count)
		})
	}
}

func TestP3DownRetainsIndependentLifecycleEvidence(t *testing.T) {
	for _, evidence := range []string{"head_open", "head_closing", "head_ended", "native_generation", "fence_frozen", "fence_live", "fence_purge", "pending_receipt", "sealed_receipt", "retirement_receipt", "completed_receipt", "target"} {
		t.Run(evidence, func(t *testing.T) {
			ctx := t.Context()
			pool := startSpaceMediaAdmissionPostgres(t, ctx)
			space, operation, receipt := uuid.New(), uuid.New(), uuid.New()
			var table string
			switch evidence {
			case "head_open", "head_closing", "head_ended", "native_generation":
				table = "voice_space_media_room_heads"
				state, generation := "OPEN", 0
				if evidence == "head_closing" {
					state = "CLOSING"
				}
				if evidence == "head_ended" {
					state = "ENDED"
				}
				if evidence == "native_generation" {
					generation = 7
				}
				_, err := pool.Exec(ctx, `INSERT INTO voice_space_media_room_heads(voice_room_id,room_generation,space_id,call_room_id,creator_operation_id,state,lifecycle_generation)
 VALUES($1,1,$2,$3,$4,$5,$6)`, uuid.New(), space, uuid.NewString(), operation, state, generation)
				require.NoError(t, err)
			case "fence_frozen", "fence_live", "fence_purge":
				table = "voice_space_lifecycle_fence_heads"
				state := "FROZEN"
				if evidence == "fence_live" {
					state = "LIVE"
				}
				if evidence == "fence_purge" {
					state = "PURGE_DECIDED"
				}
				_, err := pool.Exec(ctx, `INSERT INTO voice_space_lifecycle_fence_heads(space_id,generation,deletion_operation_id,state,manifest_id,manifest_sha256,manifest_item_count)
 VALUES($1,1,$2,$3,$4,decode(repeat('ab',32),'hex'),0)`, space, operation, state, uuid.New())
				require.NoError(t, err)
			default:
				table = "voice_space_lifecycle_fence_receipts"
				_, err := pool.Exec(ctx, `INSERT INTO voice_space_lifecycle_fence_receipts(space_id,generation,deletion_operation_id,request_bytes,request_sha256,receipt_id,applied_state,coverage_sealed,retirement_proven)
 VALUES($1,1,$2,'original-request',decode(repeat('ab',32),'hex'),$3,'FROZEN',$4,$5)`, space, operation, receipt, evidence == "sealed_receipt", evidence == "retirement_receipt")
				require.NoError(t, err)
				if evidence == "completed_receipt" {
					_, err = pool.Exec(ctx, `UPDATE voice_space_lifecycle_fence_receipts SET receipt_bytes='exact-receipt',receipt_sha256=decode(repeat('cd',32),'hex'),applied_at=clock_timestamp()`)
					require.NoError(t, err)
				}
				if evidence == "target" {
					_, err = pool.Exec(ctx, `INSERT INTO voice_space_lifecycle_targets(space_id,generation,target_id,source_kind,deletion_operation_id,request_sha256,target)
 VALUES($1,1,'exact-target','head',$2,decode(repeat('ab',32),'hex'),'{}')`, space, operation)
					require.NoError(t, err)
				}
			}
			var expected string
			require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(t))::text FROM `+table+` t`).Scan(&expected))
			require.Error(t, execP3Down(ctx, pool, p3DownSQL(t)))
			var actual string
			require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(t))::text FROM `+table+` t`).Scan(&actual))
			require.JSONEq(t, expected, actual)
			var grants, targets bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('voice_space_media_grants') IS NOT NULL,to_regclass('voice_space_lifecycle_targets') IS NOT NULL`).Scan(&grants, &targets))
			require.True(t, grants)
			require.True(t, targets)
			if evidence == "target" {
				var target string
				require.NoError(t, pool.QueryRow(ctx, `SELECT target_id FROM voice_space_lifecycle_targets`).Scan(&target))
				require.Equal(t, "exact-target", target)
			}
		})
	}
}
