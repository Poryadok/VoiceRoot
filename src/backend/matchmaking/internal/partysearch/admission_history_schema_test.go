package partysearch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/store"
)

// These deliberately synthetic bytes test retention only, never receipt validity
// or installation authority. Production readers must validate the wire chain.
func TestAdmissionHistorySchemaRetainsImmutableEvidence(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	hash := make([]byte, 32)
	hash[0] = 1
	_, err := pool.Exec(ctx, `INSERT INTO mm_native_admission_receipts(receipt_sha256,receipt_bytes,database_lineage_id,authority_incarnation,transition_id,admission_generation,environment_id,installation_id) VALUES($1,$2,$3,$4,$5,1,$6,$7)`, hash, []byte(`{"fixture":true}`), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)
	for _, query := range []string{
		`UPDATE mm_native_admission_receipts SET receipt_bytes='changed'::bytea`,
		`DELETE FROM mm_native_admission_receipts`,
		`TRUNCATE mm_native_admission_receipts CASCADE`,
	} {
		_, err = pool.Exec(ctx, query)
		require.Error(t, err, query)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_receipts`).Scan(&count))
	require.Equal(t, 1, count)
	other := make([]byte, 32)
	other[0] = 2
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_admission_receipts(receipt_sha256,receipt_bytes,database_lineage_id,authority_incarnation,transition_id,admission_generation,environment_id,installation_id) VALUES($1,$2,$3,$4,$5,1,$6,$7)`, other, []byte(`{"fixture":true}`), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_continuity_records VALUES($1,$2,$3,$4,$1)`, hash, []byte(`{"fixture":true}`), hash, other)
	require.NoError(t, err)
	party, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, partyParams(t, pool))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_party_origins VALUES($1,$2,$3,$4,$5,$6,$7)`, party.Party.ID, hash, []byte(`{"fixture":true}`), hash, uuid.New(), party.Party.SearchOperationID, hash)
	require.NoError(t, err)
	for _, table := range []string{"mm_native_continuity_records", "mm_native_party_origins"} {
		for _, query := range []string{"UPDATE " + table + " SET " + map[string]string{"mm_native_continuity_records": "record_bytes", "mm_native_party_origins": "origin_bytes"}[table] + "='changed'::bytea", "DELETE FROM " + table, "TRUNCATE " + table + " CASCADE"} {
			_, err = pool.Exec(ctx, query)
			require.Error(t, err, query)
		}
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count))
		require.Equal(t, 1, count)
	}
}

func TestAdmissionHistorySchemaDownRetainsBegunHeadOrEvidence(t *testing.T) {
	body, err := os.ReadFile("../../../migrations/matchmaking_db/000025_mm_source_origin.down.sql")
	require.NoError(t, err)
	for _, retained := range []string{"empty", "head", "receipt"} {
		t.Run(retained, func(t *testing.T) {
			pool := partyDB(t)
			ctx := context.Background()
			if retained == "head" {
				_, err := pool.Exec(ctx, `INSERT INTO mm_native_admission_head(protocol_version,state) VALUES(1,'CLOSED')`)
				require.NoError(t, err)
			}
			if retained == "receipt" {
				_, err := pool.Exec(ctx, `INSERT INTO mm_native_admission_receipts VALUES($1,$2,$3,$4,$5,1,$6,$7)`, make([]byte, 32), []byte("fixture"), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New())
				require.NoError(t, err)
			}
			conn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			_, err = conn.Exec(ctx, string(body))
			if retained == "empty" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				_, rollbackErr := conn.Exec(ctx, `ROLLBACK`)
				require.NoError(t, rollbackErr)
			}
			conn.Release()
			var exists bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.mm_native_admission_receipts') IS NOT NULL`).Scan(&exists))
			require.Equal(t, retained != "empty", exists)
		})
	}
}

func TestAdmissionHistorySchemaDownWaitsForUncommittedFirstReceipt(t *testing.T) {
	pool := partyDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	_, err = writer.Exec(ctx, `INSERT INTO mm_native_admission_receipts VALUES($1,$2,$3,$4,$5,1,$6,$7)`, make([]byte, 32), []byte("fixture"), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)
	body, err := os.ReadFile("../../../migrations/matchmaking_db/000025_mm_source_origin.down.sql")
	require.NoError(t, err)
	closer, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer closer.Release()
	finished := make(chan error, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		_, downErr := closer.Exec(ctx, string(body))
		rollback, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		_, _ = closer.Exec(rollback, `ROLLBACK`)
		finished <- downErr
	}()
	defer func() { cancel(); <-joined }()
	// Observe an actual retained-relation lock wait, not a timer assumption.
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND relation='mm_native_admission_receipts'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`, closer.Conn().PgConn().PID()).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.Error(t, <-finished)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_receipts`).Scan(&count))
	require.Equal(t, 1, count)
}
