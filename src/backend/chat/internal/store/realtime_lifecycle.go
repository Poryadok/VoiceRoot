package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	chatv1 "voice.app/voice/chat/v1"

	"voice/backend/pkg/spacemutationlock"
)

// ResolveChatLifecycle is structural enrollment, never recipient authorization.
// It rechecks the native row under the same Space mutation barrier used by
// manifest capture. No missing row or failed native read becomes LIVE.
func (s *SpaceLifecycleStore) ResolveChatLifecycle(ctx context.Context, chatID uuid.UUID) (*chatv1.ResolveChatLifecycleResponse, error) {
	if s == nil || s.Pool == nil || chatID == uuid.Nil {
		return nil, ErrSpaceLifecycleRequest
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var initialSpace *uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT space_id FROM chats WHERE id=$1`, chatID).Scan(&initialSpace); err != nil {
		return nil, err
	}
	if initialSpace != nil {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(*initialSpace)); err != nil {
			return nil, err
		}
	}
	var currentSpace *uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT space_id FROM chats WHERE id=$1 FOR SHARE`, chatID).Scan(&currentSpace); err != nil {
		return nil, err
	}
	if (initialSpace == nil) != (currentSpace == nil) || initialSpace != nil && *initialSpace != *currentSpace {
		return nil, ErrSpaceLifecycleConflict
	}
	response := &chatv1.ResolveChatLifecycleResponse{ProtocolVersion: 1, SchemaVersion: 1, ChatId: chatID.String(), State: chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_LIVE}
	if currentSpace != nil {
		response.SpaceId = currentSpace.String()
		var state string
		var operation uuid.UUID
		err = tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id FROM chat_space_lifecycle_fences WHERE space_id=$1`, *currentSpace).Scan(&response.Generation, &state, &operation)
		if errors.Is(err, pgx.ErrNoRows) {
			// This is Chat's native documented unfenced initial state; the table read
			// must succeed, so an unmigrated/unavailable schema cannot reach this case.
			response.Generation = 0
		} else if err != nil {
			return nil, err
		} else {
			response.DeletionOperationId = operation.String()
			switch state {
			case "LIVE":
				response.State = chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_LIVE
			case "FROZEN":
				response.State = chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_FROZEN
			case "PURGE_DECIDED":
				response.State = chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_PURGE_DECIDED
			case "PURGED":
				response.State = chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_PURGED
			default:
				return nil, ErrSpaceLifecycleState
			}
		}
	} else {
		// Assert the authoritative lifecycle schema exists even for ordinary chats.
		if _, err = tx.Exec(ctx, `SELECT 1 FROM chat_space_lifecycle_fences LIMIT 0`); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}
