package store

import (
	"context"
	"fmt"
	"github.com/google/uuid"
)

// CleanupSpaceLifecycleEvidence uses the participant's first database
// completion, never a replay timestamp. Minimal Space/chat fences survive.
func (s *MessagesStore) CleanupSpaceLifecycleEvidence(ctx context.Context) error {
	if s == nil || s.Pool == nil {
		return ErrSpacePurgeReceiptBinding
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Delete only this parent's deterministic child operations. A chat may have
	// unrelated purge evidence; chat_id alone cannot authorize its expiry.
	rows, err := tx.Query(ctx, `SELECT r.space_id,r.deletion_operation_id,i.chat_id FROM messaging_space_purge_receipts r JOIN messaging_space_chat_manifest_items i ON i.space_id=r.space_id AND i.deletion_operation_id=r.deletion_operation_id WHERE r.completed_at<=clock_timestamp()-interval '30 days' FOR UPDATE OF r`)
	if err != nil {
		return err
	}
	var children []uuid.UUID
	for rows.Next() {
		var space, operation, chat uuid.UUID
		if err = rows.Scan(&space, &operation, &chat); err != nil {
			rows.Close()
			return err
		}
		children = append(children, uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("voice.messaging.v1.SpaceChatPurge\x00%s\x00%s\x00%s", space, operation, chat))))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(children) > 0 {
		if _, err = tx.Exec(ctx, `DELETE FROM managed_chat_purge_messages WHERE operation_id=ANY($1::uuid[])`, children); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM managed_chat_purge_operations WHERE operation_id=ANY($1::uuid[])`, children); err != nil {
			return err
		}
	}
	// The permanent compact fence keeps source identity/hash/generation. Empty
	// byte fields are deliberately unusable as full, post-window RPC receipts.
	for _, query := range []string{
		`DELETE FROM messaging_space_file_producers p USING messaging_space_lifecycle_operations o WHERE p.space_id=o.space_id AND p.deletion_operation_id=o.deletion_operation_id AND o.generation=p.schedule_generation+1 AND o.retain_until<=clock_timestamp() AND EXISTS(SELECT 1 FROM messaging_space_lifecycle_fences f WHERE f.space_id=p.space_id AND f.generation>=o.generation AND (f.generation>o.generation OR f.state='LIVE'))`,
		`DELETE FROM messaging_space_file_producers p USING messaging_space_purge_receipts r WHERE p.space_id=r.space_id AND p.deletion_operation_id=r.deletion_operation_id AND p.schedule_generation=r.source_schedule_generation AND r.completed_at<=clock_timestamp()-interval '30 days'`,
		`UPDATE messaging_space_chat_manifest_pages p SET page_bytes=''::bytea,request_bytes=''::bytea,receipt_bytes=''::bytea,next_page_token='' FROM messaging_space_purge_receipts r WHERE p.space_id=r.space_id AND p.deletion_operation_id=r.deletion_operation_id AND r.completed_at<=clock_timestamp()-interval '30 days'`,
		`DELETE FROM messaging_space_lifecycle_operations o USING messaging_space_purge_receipts r WHERE o.space_id=r.space_id AND o.deletion_operation_id=r.deletion_operation_id AND r.completed_at<=clock_timestamp()-interval '30 days'`,
		`UPDATE messaging_space_lifecycle_fences f SET request_bytes=''::bytea,receipt_bytes=''::bytea FROM messaging_space_purge_receipts r WHERE f.space_id=r.space_id AND f.deletion_operation_id=r.deletion_operation_id AND f.state='PURGED' AND r.completed_at<=clock_timestamp()-interval '30 days'`,
		`DELETE FROM messaging_space_purge_receipts WHERE completed_at<=clock_timestamp()-interval '30 days'`,
	} {
		if _, err = tx.Exec(ctx, query); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
