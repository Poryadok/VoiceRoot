package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Called with this Space's exclusive lifecycle guard, before source row locks
// or erasure. Existing immutable journals already own their frozen obligations.
// Otherwise this ordinary Bot-level bit prevents absence from becoming proof.
// It is deliberately not a receipt or retained personal-source fingerprint.
func (s *BotStore) markRetiredCleanupUnresolved(ctx context.Context, tx pgx.Tx, space uuid.UUID) error {
	rows, err := tx.Query(ctx, `SELECT bot_id FROM bot_space_installations WHERE space_id=$1
UNION SELECT bot_id FROM bot_chat_whitelist WHERE space_id=$1
UNION SELECT bot_id FROM bot_installation_intents WHERE space_id=$1 AND state<>'CANCELED'
ORDER BY bot_id`, space)
	if err != nil {
		return err
	}
	var bots []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			break
		}
		bots = append(bots, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, bot := range bots {
		if _, err = s.GetBotByIDForUpdate(ctx, tx, bot); err != nil {
			return err
		}
		var frozen bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bot_deletion_installations WHERE bot_id=$1 AND space_id=$2)
OR EXISTS(SELECT 1 FROM bot_uninstall_installations t JOIN bot_uninstall_operations o ON o.operation_id=t.operation_id WHERE t.bot_id=$1 AND t.space_id=$2 AND o.state<>'COMPLETE')`, bot, space).Scan(&frozen); err != nil {
			return err
		}
		if !frozen {
			if _, err = tx.Exec(ctx, `UPDATE bots SET retired_cleanup_unresolved=true WHERE id=$1`, bot); err != nil {
				return err
			}
		}
	}
	return nil
}
