package store

import (
	"context"
	"github.com/google/uuid"
)

type MatchSquadProvisioningOutcomeState string

const (
	MatchSquadOutcomeActive              MatchSquadProvisioningOutcomeState = "ACTIVE"
	MatchSquadOutcomeWaitingValidation   MatchSquadProvisioningOutcomeState = "WAITING_FOR_VALIDATION"
	MatchSquadOutcomeSettlingAbandoned   MatchSquadProvisioningOutcomeState = "SETTLING_ABANDONED"
	MatchSquadOutcomeCompensationPending MatchSquadProvisioningOutcomeState = "COMPENSATION_PENDING"
	MatchSquadOutcomeCompensationClosed  MatchSquadProvisioningOutcomeState = "COMPENSATION_CLOSED"
	MatchSquadOutcomeContractMismatch    MatchSquadProvisioningOutcomeState = "CONTRACT_MISMATCH"
)

// MatchSquadProvisioningOutcome is an observation, never a reusable dispatch
// authorization or provider absence receipt. Only ACTIVE carries resource IDs.
type MatchSquadProvisioningOutcome struct {
	MatchID     uuid.UUID
	State       MatchSquadProvisioningOutcomeState
	ChatID      string
	VoiceRoomID string
}

func (s *MatchStore) GetMatchSquadProvisioningOutcome(ctx context.Context, matchID uuid.UUID) (MatchSquadProvisioningOutcome, error) {
	if s == nil || s.Pool == nil || matchID == uuid.Nil {
		return MatchSquadProvisioningOutcome{}, ErrSourcePartyUnavailable
	}
	var status, state, disposition, chat, voice, chatID, roomID string
	var operationID *uuid.UUID
	var ready, mismatch bool
	err := s.Pool.QueryRow(ctx, `SELECT m.status,o.operation_id,COALESCE(o.state,''),
 COALESCE(to_jsonb(o)->>'provision_disposition',''),COALESCE(to_jsonb(o)->>'chat_dispatch_state',''),COALESCE(to_jsonb(o)->>'voice_dispatch_state',''),
 COALESCE(o.chat_id::text,''),COALESCE(o.voice_room_id::text,''),
 EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='matchmaking_match_squad_operations'::regclass AND attname='creation_evidence_version' AND NOT attisdropped),
 (to_jsonb(o)->>'chat_creation_mismatch_bytes' IS NOT NULL OR to_jsonb(o)->>'voice_creation_mismatch_bytes' IS NOT NULL
 OR EXISTS(SELECT 1 FROM matchmaking_match_squad_teardowns a JOIN matchmaking_match_squad_teardown_participants p USING(aggregate_id)
 WHERE a.match_id=m.id AND p.state='CONTRACT_MISMATCH'))
 FROM matches m LEFT JOIN matchmaking_match_squad_operations o ON o.match_id=m.id WHERE m.id=$1`, matchID).
		Scan(&status, &operationID, &state, &disposition, &chat, &voice, &chatID, &roomID, &ready, &mismatch)
	if err != nil {
		return MatchSquadProvisioningOutcome{}, err
	}
	if !ready {
		return MatchSquadProvisioningOutcome{}, ErrSourcePartyUnavailable
	}
	outcome := MatchSquadProvisioningOutcome{MatchID: matchID, State: MatchSquadOutcomeWaitingValidation}
	if operationID == nil {
		if status == MatchStatusAbandoned {
			outcome.State = MatchSquadOutcomeCompensationClosed
			return outcome, nil
		}
		if status == MatchStatusPendingAccept {
			return outcome, nil
		}
		return MatchSquadProvisioningOutcome{}, ErrMatchSquadPending
	}
	if mismatch {
		outcome.State = MatchSquadOutcomeContractMismatch
		return outcome, nil
	}
	if disposition == "" || chat == "" || voice == "" {
		return MatchSquadProvisioningOutcome{}, ErrSourcePartyUnavailable
	}
	if state == "active" {
		if status != MatchStatusActive || chatID == "" || roomID == "" || chat != "RECEIPT_KNOWN" || voice != "RECEIPT_KNOWN" {
			return MatchSquadProvisioningOutcome{}, ErrMatchSquadConflict
		}
		return MatchSquadProvisioningOutcome{MatchID: matchID, State: MatchSquadOutcomeActive, ChatID: chatID, VoiceRoomID: roomID}, nil
	}
	if disposition == "ABANDONED" || status == MatchStatusAbandoned {
		switch {
		case disposition == "ABANDONED" && state == "closed":
			outcome.State = MatchSquadOutcomeCompensationClosed
		case disposition != "ABANDONED" || chat == "MAY_HAVE_CREATED" || voice == "MAY_HAVE_CREATED":
			outcome.State = MatchSquadOutcomeSettlingAbandoned
		case state == "compensating":
			outcome.State = MatchSquadOutcomeCompensationPending
		default:
			return MatchSquadProvisioningOutcome{}, ErrMatchSquadConflict
		}
		return outcome, nil
	}
	if state != "provisioning" || status != MatchStatusPendingAccept {
		return MatchSquadProvisioningOutcome{}, ErrMatchSquadPending
	}
	return outcome, nil
}
