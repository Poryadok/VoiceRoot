package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/spaceaudit"
)

// checkAuditIngressLifecycle runs inside the ingestion transaction BEFORE its
// replay lookup. Holding the same Space lock as composite commit/freeze/purge
// prevents a prechecked delivery from crossing the terminal deletion boundary.
func checkAuditIngressLifecycle(ctx context.Context, tx pgx.Tx, producer string, event spaceaudit.ReplayEvent) error {
	if tx == nil || spaceaudit.ValidateEvent(producer, event) != nil {
		return spaceaudit.ErrInvalidEvent
	}
	spaceID, err := uuid.Parse(event.SpaceID)
	if err != nil {
		return spaceaudit.ErrInvalidEvent
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(spaceID)); err != nil {
		return err
	}
	var purged, frozen bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_permanent_purge_fences WHERE space_id=$1),EXISTS(SELECT 1 FROM space_lifecycle_aggregates WHERE space_id=$1 AND phase<>'LIVE')`, spaceID).Scan(&purged, &frozen); err != nil {
		return err
	}
	if purged {
		return ErrLifecycleFrozen
	}
	// The permanent audit ID reservation remains authoritative after ordinary
	// audit/receipt payloads have been minimized. It cannot be reused by Role or
	// by a different Chat fact, even while the containing Space is LIVE.
	var operationID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT operation_id FROM space_chat_creation_fences WHERE audit_event_id=$1`, uuid.MustParse(event.AuditEventID)).Scan(&operationID)
	if errors.Is(err, pgx.ErrNoRows) {
		if frozen {
			return ErrLifecycleFrozen
		}
		return nil
	}
	if err != nil {
		return err
	}
	if producer != "chat" || event.Action != "chat_created" || event.TargetType != "chat" || event.DetailsJSON != "{}" {
		return ErrCompositeConflict
	}
	saved, err := loadChatCreationOutcome(ctx, tx, spaceID, operationID, nil)
	if err != nil {
		return err
	}
	b := saved.GetBinding()
	if saved.GetState() != spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED || b.GetAuditEventId() != event.AuditEventID || b.GetSpaceId() != event.SpaceID || b.GetChatId() != event.TargetID || b.GetCreatorProfileId() != event.ActorProfileID {
		return ErrCompositeConflict
	}
	return nil
}
