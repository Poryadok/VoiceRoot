package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/spacemutationlock"
)

// Caller holds the owning Space lifecycle lock and has verified this exact
// sealed manifest. No account identifiers enter a Space receipt or RPC.
func eraseAutoModSpaceCoverageTx(ctx context.Context, tx pgx.Tx, space, operation uuid.UUID, generation uint64) error {
	if _, err := tx.Exec(ctx, `DELETE FROM automod_message_coverage c USING messaging_space_chat_manifest_items i
WHERE c.chat_id=i.chat_id AND i.space_id=$1 AND i.deletion_operation_id=$2 AND i.schedule_generation=$3`, space, operation, generation); err != nil {
		return err
	}
	var remaining bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_message_coverage c JOIN messaging_space_chat_manifest_items i ON i.chat_id=c.chat_id WHERE i.space_id=$1 AND i.deletion_operation_id=$2 AND i.schedule_generation=$3)`, space, operation, generation).Scan(&remaining); err != nil {
		return err
	}
	if remaining {
		return ErrSpacePurgeReceiptBinding
	}
	return nil
}

// ReconcileAutoModOriginCoverage removes restored forbidden origin bridges.
// The bootstrap coordinator must first recover authoritative Space/managed-chat
// lifecycle history; agreement between two old backups is not that proof.
// This local cleanup alone never opens readiness or evidence delivery.
func (s *MessagesStore) ReconcileAutoModOriginCoverage(ctx context.Context) error {
	rows, err := s.Pool.Query(ctx, `SELECT space_id FROM messaging_space_lifecycle_fences ORDER BY space_id`)
	if err != nil {
		return err
	}
	var spaces []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		spaces = append(spaces, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, space := range spaces {
		if err = s.reconcileAutoModSpaceCoverage(ctx, space); err != nil {
			return err
		}
	}
	rows, err = s.Pool.Query(ctx, `SELECT DISTINCT chat_id FROM managed_chat_purge_operations ORDER BY chat_id`)
	if err != nil {
		return err
	}
	var chats []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		chats = append(chats, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, chat := range chats {
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = lockChatAndSpaceForPurge(ctx, tx, chat); err == nil {
			_, err = tx.Exec(ctx, `DELETE FROM automod_message_coverage c USING managed_chat_purge_operations p WHERE c.chat_id=$1 AND p.chat_id=c.chat_id AND c.created_at<=p.created_at`, chat)
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		_ = tx.Rollback(context.Background())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *MessagesStore) reconcileAutoModSpaceCoverage(ctx context.Context, space uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(space)); err != nil {
		return err
	}
	var operation uuid.UUID
	var generation uint64
	var state string
	if err = tx.QueryRow(ctx, `SELECT deletion_operation_id,source_schedule_generation,state FROM messaging_space_lifecycle_fences WHERE space_id=$1 FOR SHARE`, space).Scan(&operation, &generation, &state); err != nil {
		return err
	}
	if state != "LIVE" {
		if err = eraseAutoModSpaceCoverageTx(ctx, tx, space, operation, generation); err != nil {
			return err
		}
	} else {
		// A product restore retains the old freeze cut. Preserve newly committed
		// post-restore links, but never reconstruct pre-freeze origin history.
		var missingCut bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_message_coverage c JOIN messaging_space_chat_manifest_items i ON i.chat_id=c.chat_id WHERE i.space_id=$1 AND i.deletion_operation_id=$2 AND i.schedule_generation=$3) AND NOT EXISTS(SELECT 1 FROM messaging_space_lifecycle_operations WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3)`, space, operation, generation).Scan(&missingCut); err != nil {
			return err
		}
		if missingCut {
			return ErrSpaceLifecycleConflict
		}
		if _, err = tx.Exec(ctx, `DELETE FROM automod_message_coverage c USING messaging_space_chat_manifest_items i,messaging_space_lifecycle_operations o WHERE c.chat_id=i.chat_id AND i.space_id=$1 AND i.deletion_operation_id=$2 AND i.schedule_generation=$3 AND o.space_id=i.space_id AND o.deletion_operation_id=i.deletion_operation_id AND o.generation=i.schedule_generation AND c.created_at<=o.applied_at`, space, operation, generation); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
