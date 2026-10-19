package nativeapp

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	w "voice/backend/pkg/mmadmissionwire"
)

func unresolvedProviderOperations(ctx context.Context, pool *pgxpool.Pool) ([]w.ID, error) {
	if ctx == nil || ctx.Err() != nil || pool == nil {
		return nil, errUnavailable
	}
	// No due-time or claim predicate: a sleeping/leased operation is still part
	// of the retained dispatch census. Pending compaction and teardown are also
	// obligations; their live authorization is deliberately independent.
	rows, err := pool.Query(ctx, `SELECT operation_id FROM (
 SELECT chat_operation_id AS operation_id FROM matchmaking_match_squad_operations WHERE state IN ('provisioning','compensating')
 UNION SELECT voice_operation_id FROM matchmaking_match_squad_operations WHERE state IN ('provisioning','compensating')
 UNION SELECT operation_id FROM matchmaking_match_squad_teardown_participants WHERE state<>'COMPLETE'
 UNION SELECT operation_id FROM matchmaking_match_squad_compaction_intents WHERE completed_at IS NULL
 ) retained ORDER BY operation_id LIMIT 1025`)
	if err != nil {
		return nil, errUnavailable
	}
	defer rows.Close()
	result := make([]w.ID, 0)
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) != nil || id == uuid.Nil {
			return nil, errUnavailable
		}
		result = append(result, w.ID(id))
	}
	if rows.Err() != nil || len(result) > 1024 {
		return nil, errUnavailable
	}
	return result, nil
}
