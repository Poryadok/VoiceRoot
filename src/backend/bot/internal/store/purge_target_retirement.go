package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

func purgeNativeMatchesTarget(q *spacev1.ReconcileBotPurgeInstallationRequest, t BotDeletionTarget) bool {
	n := q.GetNativeCancellation()
	v := n.GetInstallationIntent()
	return !t.LegacyEvidence && q.BotId == t.BotID.String() && q.SpaceId == t.SpaceID.String() && q.InstallationId == t.InstallationID.String() && n.ProfileId == t.ActorProfileID.String() && n.InstallOperationId == t.InstallOperationID.String() && v.Generation == uint64(t.BotGeneration) && v.OperationId == t.CancelOperationID.String()
}

func purgeTargetEffectsMatch(w BotPurgeRetainedWork) bool {
	space, role := 0, 0
	chats := map[uuid.UUID]bool{}
	for _, e := range w.Effects {
		switch e.Kind {
		case "SPACE_CLEANUP":
			if e.ChatID != nil {
				return false
			}
			space++
		case "ROLE_REMOVE":
			if e.ChatID != nil {
				return false
			}
			role++
		case "MESSAGING_UNPIN":
			if e.ChatID == nil || chats[*e.ChatID] {
				return false
			}
			chats[*e.ChatID] = true
		default:
			return false
		}
	}
	if space != 1 || role != 1 || len(chats) != len(w.Target.Chats) {
		return false
	}
	for _, id := range w.Target.Chats {
		if !chats[id] {
			return false
		}
		delete(chats, id)
	}
	return len(chats) == 0
}

// All proof is revalidated while rich lineage still exists. The resulting
// target/effect state is LOCAL, not a forged remote participant receipt.
func (s *BotStore) RetireBotPurgeHandoff(ctx context.Context, c *BotPurgeClaim) (*commonv1.SpacePurgeReceipt, error) {
	tx, err := s.purgeClaimTx(ctx, c)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	known := map[uuid.UUID]bool{}
	for _, source := range c.sources {
		known[source.bot] = true
	}
	rows, err := tx.Query(ctx, `SELECT bot_id FROM bot_space_installations WHERE space_id=$1 UNION SELECT bot_id FROM bot_chat_whitelist WHERE space_id=$1 UNION SELECT bot_id FROM bot_installation_intents WHERE space_id=$1 AND state<>'CANCELED' UNION SELECT t.bot_id FROM bot_uninstall_installations t JOIN bot_uninstall_operations o ON o.operation_id=t.operation_id WHERE t.space_id=$1 AND o.state<>'COMPLETE' UNION SELECT t.bot_id FROM bot_deletion_installations t JOIN bot_deletion_operations o ON o.operation_id=t.operation_id WHERE t.space_id=$1 AND o.state<>'COMPLETE'`, c.space)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var bot uuid.UUID
		if err = rows.Scan(&bot); err != nil {
			break
		}
		if !known[bot] {
			err = ErrSpaceLifecycleConflict
			break
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, source := range c.Sources() {
		envelope, e := s.purgeObservationTx(ctx, tx, c, source.BotID)
		if e != nil {
			return nil, e
		}
		plans, e := c.NativeDrainRequests(source.BotID)
		if e != nil {
			return nil, e
		}
		if len(envelope.Native) != len(plans) {
			return nil, ErrDeletionPending
		}
		for _, v := range envelope.Native {
			if len(v.ReceiptBytes) == 0 {
				return nil, ErrDeletionPending
			}
		}
		if source.Intent != nil && source.Intent.LegacyEvidence {
			return nil, ErrDeletionPending
		}
		if source.InstalledID != nil && len(plans) == 0 {
			return nil, ErrDeletionPending
		}
		allowedChats := map[uuid.UUID]bool{}
		batches, e := c.ChatBatches(source.BotID)
		if e != nil {
			return nil, e
		}
		for _, batch := range batches {
			for _, id := range batch {
				parsed, e := lifecycleID(id)
				if e != nil {
					return nil, e
				}
				allowedChats[parsed] = true
			}
		}
		attachments := map[uuid.UUID][]byte{}
		addAttachments := func(list []ChatScopeCleanupSource) error {
			for _, v := range list {
				raw, e := json.Marshal(v)
				if e != nil {
					return e
				}
				if old, ok := attachments[v.AttachmentID]; ok && !bytes.Equal(old, raw) {
					return ErrSpaceLifecycleConflict
				}
				attachments[v.AttachmentID] = raw
			}
			return nil
		}
		if e = addAttachments(source.ChatSources); e != nil {
			return nil, e
		}
		for _, w := range source.Retained {
			if e = addAttachments(w.Target.ChatSources); e != nil {
				return nil, e
			}
		}
		checkAttachments := func(list []ChatScopeCleanupSource) bool {
			for _, v := range list {
				raw, e := json.Marshal(v)
				if e != nil || !bytes.Equal(attachments[v.AttachmentID], raw) {
					return false
				}
			}
			return true
		}
		currentSources, e := snapshotChatScopeSources(ctx, tx, source.BotID, c.space)
		if e != nil {
			return nil, e
		}
		if !checkAttachments(currentSources) {
			return nil, ErrSpaceLifecycleConflict
		}
		var installed uuid.UUID
		e = tx.QueryRow(ctx, `SELECT id FROM bot_space_installations WHERE bot_id=$1 AND space_id=$2 FOR UPDATE`, source.BotID, c.space).Scan(&installed)
		if e == nil {
			matched := false
			for _, q := range plans {
				matched = matched || q.InstallationId == installed.String()
			}
			if !matched {
				return nil, ErrSpaceLifecycleConflict
			}
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return nil, e
		}
		i, e := scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE bot_id=$1 AND space_id=$2 FOR UPDATE`, source.BotID, c.space))
		if e == nil {
			matched := false
			for _, q := range plans {
				v := q.NativeCancellation.InstallationIntent
				if !i.LegacyEvidence && q.InstallationId == i.InstallationID.String() && q.NativeCancellation.InstallOperationId == i.OperationID.String() && q.NativeCancellation.ProfileId == i.ActorProfileID.String() && v.Generation == uint64(i.Generation) && (i.CancelOperationID == uuid.Nil || v.OperationId == i.CancelOperationID.String()) {
					matched = true
				}
			}
			if !matched {
				return nil, ErrSpaceLifecycleConflict
			}
			for _, chat := range i.Chats {
				if !allowedChats[chat] {
					return nil, ErrSpaceLifecycleConflict
				}
			}
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return nil, e
		}
		for _, purpose := range []string{"global_delete", "space_uninstall"} {
			var include []uuid.UUID
			for _, w := range source.Retained {
				if w.Purpose == purpose {
					include = append(include, w.OperationID)
				}
			}
			current, e := capturePurgeRetained(ctx, tx, source.BotID, c.space, purpose, include...)
			if e != nil {
				return nil, e
			}
			seen := map[uuid.UUID]bool{}
			for _, w := range current {
				seen[w.OperationID] = true
				root := &BotDeletion{Purpose: purpose, OperationID: w.OperationID}
				if _, _, e = lifecycleAccountingTx(ctx, tx, root); e != nil {
					return nil, e
				}
				for _, original := range source.Retained {
					if original.Purpose == purpose && original.OperationID == w.OperationID {
						if !bytes.Equal(original.TargetBytes, w.TargetBytes) || !bytes.Equal(original.DecisionBytes, w.DecisionBytes) {
							return nil, ErrSpaceLifecycleConflict
						}
					}
				}
				if !purgeTargetEffectsMatch(w) || !checkAttachments(w.Target.ChatSources) {
					return nil, ErrSpaceLifecycleConflict
				}
				for _, chat := range w.Target.Chats {
					if !allowedChats[chat] {
						return nil, ErrSpaceLifecycleConflict
					}
				}
				matched := false
				for _, q := range plans {
					matched = matched || purgeNativeMatchesTarget(q, w.Target)
				}
				if !matched {
					return nil, ErrDeletionPending
				}
				// The complete accepted identity ledger remains. Only this matched
				// target's rich projection and service-shaped effect payloads retire.
				if _, e = tx.Exec(ctx, lifecycleQuery(root, `UPDATE bot_deletion_effects SET state='LOCAL_P3_COVERED',kind=NULL,chat_id=NULL,request_bytes=NULL,request_sha256=NULL,receipt_bytes=NULL,receipt_sha256=NULL WHERE target_id=$1`), w.Target.TargetID); e != nil {
					return nil, e
				}
				if _, e = tx.Exec(ctx, lifecycleQuery(root, `UPDATE bot_deletion_installations SET disposition='LOCAL_P3_COVERED',local_result_version=1,local_result_id=$2,locally_completed_at=clock_timestamp(),bot_id=NULL,space_id=NULL,actor_profile_id=NULL,installation_id=NULL,install_operation_id=NULL,bot_generation=NULL,cancel_operation_id=NULL,chat_ids=NULL,legacy_evidence=NULL,scope_bytes=NULL,scope_sha256=NULL WHERE target_id=$1`), w.Target.TargetID, uuid.New()); e != nil {
					return nil, e
				}
			}
			for _, id := range include {
				if !seen[id] {
					return nil, ErrSpaceLifecycleConflict
				}
			}
		}
		if _, e = tx.Exec(ctx, `UPDATE bot_space_purge_sources SET state='COVERED',covered_at=clock_timestamp() WHERE space_id=$1 AND bot_id=$2 AND state='PENDING'`, c.space, source.BotID); e != nil {
			return nil, e
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_space_purge_handoffs SET state='COVERED' WHERE space_id=$1 AND state='PENDING'`, c.space); err != nil {
		return nil, err
	}
	hash := [32]byte{}
	copy(hash[:], c.requestSHA)
	receipt, err := s.finishPurgeSpaceTx(ctx, tx, c.purge, c.space, c.operation, c.generation, c.request, hash, true)
	if err != nil {
		return nil, err
	}
	var fresh bool
	if err = tx.QueryRow(ctx, `SELECT claimed_until>clock_timestamp() AND claim_generation=$2 FROM bot_space_purge_handoffs WHERE space_id=$1`, c.space, c.claimGeneration).Scan(&fresh); err != nil {
		return nil, err
	}
	if !fresh {
		return nil, ErrDeletionClaimLost
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receipt, nil
}
