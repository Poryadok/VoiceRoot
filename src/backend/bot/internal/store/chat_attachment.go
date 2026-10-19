package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type chatAttachment struct {
	id, bot, space, chat           uuid.UUID
	installation, intent, original *uuid.UUID
}

func readChatAttachment(ctx context.Context, tx pgx.Tx, id, bot, space, chat uuid.UUID) (chatAttachment, error) {
	var a chatAttachment
	err := tx.QueryRow(ctx, `SELECT attachment_id,bot_id,space_id,chat_id,installation_id,intent_operation_id,origin_scope_observation_id FROM bot_chat_attachments WHERE attachment_id=$1`, id).Scan(&a.id, &a.bot, &a.space, &a.chat, &a.installation, &a.intent, &a.original)
	if err != nil {
		return a, err
	}
	if a.id == uuid.Nil || a.bot != bot || a.space != space || a.chat != chat {
		return a, ErrChatScopeObservation
	}
	if a.original != nil {
		o, err := readChatScopeObservation(ctx, tx, bot, space, *a.original)
		if err != nil {
			return a, err
		}
		if o.observation.chat != chat {
			return a, ErrChatScopeObservation
		}
	}
	return a, nil
}
func chatScopeIDsByChat(ctx context.Context, tx pgx.Tx, bot, space uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	result := make(map[uuid.UUID]uuid.UUID, len(ids))
	for _, id := range ids {
		o, err := readChatScopeObservation(ctx, tx, bot, space, id)
		if err != nil {
			return nil, err
		}
		if _, ok := result[o.observation.chat]; ok {
			return nil, ErrChatScopeObservation
		}
		result[o.observation.chat] = id
	}
	return result, nil
}

// This runs while the existing Space -> Bot guards are held, before Add/retry.
// It captures the persisted intent's original bytes, never a later fresh read.
func ensureInstallationAttachments(ctx context.Context, tx pgx.Tx, i *InstallationIntent) ([]uuid.UUID, error) {
	var raw, scopeRaw []byte
	if err := tx.QueryRow(ctx, `SELECT chat_attachment_ids,chat_scope_observation_ids FROM bot_installation_intents WHERE operation_id=$1 FOR UPDATE`, i.OperationID).Scan(&raw, &scopeRaw); err != nil {
		return nil, err
	}
	var ids, scopeIDs []uuid.UUID
	if json.Unmarshal(raw, &ids) != nil || json.Unmarshal(scopeRaw, &scopeIDs) != nil {
		return nil, ErrChatScopeObservation
	}
	scopes, err := chatScopeIDsByChat(ctx, tx, i.BotID, i.SpaceID, scopeIDs)
	if err != nil {
		return nil, err
	}
	if len(scopeIDs) > 0 && len(scopeIDs) != len(i.Chats) {
		return nil, ErrChatScopeObservation
	}
	for _, scope := range scopeIDs {
		o, err := readChatScopeObservation(ctx, tx, i.BotID, i.SpaceID, scope)
		if err != nil {
			return nil, err
		}
		if o.intent == nil || *o.intent != i.OperationID || o.origin != "install_admission" {
			return nil, ErrChatScopeObservation
		}
	}
	if len(ids) > 0 {
		if len(ids) != len(i.Chats) {
			return nil, ErrChatScopeObservation
		}
		seen := map[uuid.UUID]bool{}
		for index, id := range ids {
			if seen[id] {
				return nil, ErrChatScopeObservation
			}
			seen[id] = true
			a, err := readChatAttachment(ctx, tx, id, i.BotID, i.SpaceID, i.Chats[index])
			if err != nil {
				return nil, err
			}
			var original *uuid.UUID
			if scope, ok := scopes[i.Chats[index]]; ok {
				original = &scope
			}
			if a.intent == nil || *a.intent != i.OperationID || a.installation == nil || *a.installation != i.InstallationID || !sameOptionalUUID(a.original, original) {
				return nil, ErrChatScopeObservation
			}
		}
		return ids, nil
	}
	ids = make([]uuid.UUID, 0, len(i.Chats))
	for _, chat := range i.Chats {
		var original *uuid.UUID
		if scope, ok := scopes[chat]; ok {
			original = &scope
		}
		id := uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO bot_chat_attachments(attachment_id,bot_id,space_id,chat_id,installation_id,intent_operation_id,origin_scope_observation_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(intent_operation_id,chat_id) DO NOTHING`, id, i.BotID, i.SpaceID, chat, i.InstallationID, i.OperationID, original)
		if err != nil {
			return nil, err
		}
		if err := tx.QueryRow(ctx, `SELECT attachment_id FROM bot_chat_attachments WHERE intent_operation_id=$1 AND chat_id=$2`, i.OperationID, chat).Scan(&id); err != nil {
			return nil, err
		}
		a, err := readChatAttachment(ctx, tx, id, i.BotID, i.SpaceID, chat)
		if err != nil {
			return nil, err
		}
		if a.installation == nil || *a.installation != i.InstallationID || !sameOptionalUUID(a.original, original) {
			return nil, ErrChatScopeObservation
		}
		ids = append(ids, id)
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_installation_intents SET chat_attachment_ids=$2::jsonb WHERE operation_id=$1`, i.OperationID, encoded); err != nil {
		return nil, err
	}
	return ids, nil
}

// created is the actual INSERT outcome, not a timestamp. Existing unqualified
// rows retain an explicitly unqualified origin even after a new current read.
func ensureWhitelistAttachment(ctx context.Context, tx pgx.Tx, bot, space, chat uuid.UUID, created, preserveKnown bool, freshScope, preferred *uuid.UUID) (uuid.UUID, error) {
	var id, storedInstallation, original, physical *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT w.attachment_id,w.source_installation_id,w.origin_scope_observation_id,i.id FROM bot_chat_whitelist w LEFT JOIN bot_space_installations i ON i.bot_id=w.bot_id AND i.space_id=w.space_id WHERE w.bot_id=$1 AND w.space_id=$2 AND w.chat_id=$3 FOR UPDATE OF w`, bot, space, chat).Scan(&id, &storedInstallation, &original, &physical)
	if err != nil {
		return uuid.Nil, err
	}
	if id != nil {
		a, err := readChatAttachment(ctx, tx, *id, bot, space, chat)
		if err != nil {
			return uuid.Nil, err
		}
		if preserveKnown || sameOptionalUUID(a.installation, physical) {
			return *id, nil
		}
		// A genuinely different current installation starts another lifetime.
		created = true
	}
	installation := physical
	if !created && storedInstallation != nil {
		installation = storedInstallation
	}
	if created {
		original = freshScope
	}
	selected := uuid.New()
	if created && preferred != nil {
		a, err := readChatAttachment(ctx, tx, *preferred, bot, space, chat)
		if err != nil {
			return uuid.Nil, err
		}
		if !sameOptionalUUID(a.installation, installation) {
			return uuid.Nil, ErrChatScopeObservation
		}
		selected = *preferred
	} else {
		var knownIntent *uuid.UUID
		if original != nil {
			o, err := readChatScopeObservation(ctx, tx, bot, space, *original)
			if err != nil {
				return uuid.Nil, err
			}
			if o.observation.chat != chat {
				return uuid.Nil, ErrChatScopeObservation
			}
			// Only an actual retained original install read plus its exact persisted
			// installation binding proves this local row and intent are one lifetime.
			if !created && o.intent != nil && o.origin == "install_admission" && storedInstallation != nil {
				var operation uuid.UUID
				lookupErr := tx.QueryRow(ctx, `SELECT operation_id FROM bot_installation_intents WHERE operation_id=$1 AND bot_id=$2 AND space_id=$3 AND installation_id=$4 AND chat_scope_observation_ids ? $5::text`, *o.intent, bot, space, *storedInstallation, original.String()).Scan(&operation)
				if lookupErr == nil {
					knownIntent = &operation
				} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
					return uuid.Nil, lookupErr
				}
			}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO bot_chat_attachments(attachment_id,bot_id,space_id,chat_id,installation_id,intent_operation_id,origin_scope_observation_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(intent_operation_id,chat_id) DO NOTHING`, selected, bot, space, chat, installation, knownIntent, original); err != nil {
			return uuid.Nil, err
		}
		if knownIntent != nil {
			if err = tx.QueryRow(ctx, `SELECT attachment_id FROM bot_chat_attachments WHERE intent_operation_id=$1 AND chat_id=$2`, *knownIntent, chat).Scan(&selected); err != nil {
				return uuid.Nil, err
			}
			a, err := readChatAttachment(ctx, tx, selected, bot, space, chat)
			if err != nil {
				return uuid.Nil, err
			}
			if !sameOptionalUUID(a.installation, installation) || !sameOptionalUUID(a.original, original) {
				return uuid.Nil, ErrChatScopeObservation
			}
		}
	}
	result, err := tx.Exec(ctx, `UPDATE bot_chat_whitelist SET attachment_id=$4 WHERE bot_id=$1 AND space_id=$2 AND chat_id=$3 AND attachment_id IS NOT DISTINCT FROM $5::uuid`, bot, space, chat, selected, id)
	if err != nil {
		return uuid.Nil, err
	}
	if result.RowsAffected() != 1 {
		return uuid.Nil, ErrChatScopeObservation
	}
	return selected, nil
}

func preferredAttachmentsByChat(ctx context.Context, tx pgx.Tx, i *InstallationIntent, ids []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	result := map[uuid.UUID]uuid.UUID{}
	for _, id := range ids {
		var chat uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT chat_id FROM bot_chat_attachments WHERE attachment_id=$1`, id).Scan(&chat); err != nil {
			return nil, err
		}
		a, err := readChatAttachment(ctx, tx, id, i.BotID, i.SpaceID, chat)
		if err != nil {
			return nil, err
		}
		if a.intent == nil || *a.intent != i.OperationID {
			return nil, ErrChatScopeObservation
		}
		if _, duplicate := result[chat]; duplicate {
			return nil, ErrChatScopeObservation
		}
		result[chat] = id
	}
	return result, nil
}

func optionalChatID(ids map[uuid.UUID]uuid.UUID, chat uuid.UUID) *uuid.UUID {
	if id, ok := ids[chat]; ok {
		return &id
	}
	return nil
}

func insertWhitelistWithAttachment(ctx context.Context, tx pgx.Tx, bot, space, chat, actor uuid.UUID, scopes, preferred map[uuid.UUID]uuid.UUID) error {
	var created bool
	err := tx.QueryRow(ctx, `INSERT INTO bot_chat_whitelist(bot_id,chat_id,space_id,enabled,added_by_profile_id) VALUES($1,$2,$3,true,$4) ON CONFLICT(bot_id,chat_id) DO NOTHING RETURNING true`, bot, chat, space, actor).Scan(&created)
	if errors.Is(err, pgx.ErrNoRows) {
		result, updateErr := tx.Exec(ctx, `UPDATE bot_chat_whitelist SET enabled=true,added_by_profile_id=$4 WHERE bot_id=$1 AND chat_id=$2 AND space_id=$3`, bot, chat, space, actor)
		if updateErr != nil {
			return updateErr
		}
		if result.RowsAffected() != 1 {
			return ErrNotFound
		}
	} else if err != nil {
		return err
	}
	_, err = ensureWhitelistAttachment(ctx, tx, bot, space, chat, created, false, optionalChatID(scopes, chat), optionalChatID(preferred, chat))
	return err
}
