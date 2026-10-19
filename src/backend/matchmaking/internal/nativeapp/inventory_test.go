package nativeapp

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"testing"
	w "voice/backend/pkg/mmadmissionwire"
)

func pendingProviderFixture(t *testing.T, pool *pgxpool.Pool, count int) []w.ID {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `WITH m AS (INSERT INTO matches(id,game_id,mode,region) SELECT gen_random_uuid(),(SELECT id FROM games LIMIT 1),'owned-fixture','eu' FROM generate_series(1,$1::integer) RETURNING id),o AS (INSERT INTO matchmaking_match_squad_operations(match_id,operation_id,participant_manifest_sha256,participant_manifest_bytes,state,chat_operation_id,chat_request_sha256,chat_request_bytes,voice_operation_id,provision_next_attempt_at,provision_claim_generation,provision_claim_until,dispatch_protocol_version) SELECT id,gen_random_uuid(),decode(repeat('01',32),'hex'),'fixture'::bytea,'provisioning',gen_random_uuid(),decode(repeat('02',32),'hex'),'fixture'::bytea,gen_random_uuid(),clock_timestamp()+interval '1 day',1,clock_timestamp()+interval '1 hour',1 FROM m RETURNING chat_operation_id,voice_operation_id) SELECT chat_operation_id,voice_operation_id FROM o`, count)
	require.NoError(t, err)
	defer rows.Close()
	var ids []w.ID
	for rows.Next() {
		var a, b uuid.UUID
		require.NoError(t, rows.Scan(&a, &b))
		ids = append(ids, w.ID(a), w.ID(b))
	}
	require.NoError(t, rows.Err())
	return ids
}

func TestNativeProviderInventoryIncludesBackoffAndLiveClaims(t *testing.T) {
	pool := nativeDBFixture(t)
	expected := pendingProviderFixture(t, pool, 2)
	actual, err := unresolvedProviderOperations(context.Background(), pool)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, actual)
	// Snapshot is a census of retained intent, not a runnable/due-work selector.
	require.Len(t, actual, 4)
}

func TestNativeProviderInventoryOverflowAndMissingSchemaAreUnavailable(t *testing.T) {
	pool := nativeDBFixture(t)
	pendingProviderFixture(t, pool, 513)
	actual, err := unresolvedProviderOperations(context.Background(), pool)
	require.Error(t, err)
	require.Nil(t, actual)
	_, err = unresolvedProviderOperations(context.Background(), nil)
	require.Error(t, err)
}
