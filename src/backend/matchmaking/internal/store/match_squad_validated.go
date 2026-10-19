package store

import (
	"bytes"
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/matchmaking/internal/mmadmission"
)

// ProvisionValidationProof is implemented by partysource's private freshly
// issued ValidatedAttempt. It is never a persisted authorization cache.
type ProvisionValidationProof interface {
	Hash() [32]byte
	Matches(ProvisionCandidate) bool
}

type MatchSquadDispatchTicket struct {
	Operation       MatchSquadProvisioningOperation
	Provider        string
	FirstDispatchID uuid.UUID
}

func (s *MatchStore) SaveValidatedMatchSquadProvisioningIntent(ctx context.Context, candidate ProvisionCandidate, proof ProvisionValidationProof, intent MatchSquadProvisioningIntent) (MatchSquadProvisioningOperation, error) {
	if intent.MatchID != candidate.MatchID || !validMatchSquadIntentFields(intent) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	var operation MatchSquadProvisioningOperation
	err := s.withValidatedProvisionCutTx(ctx, candidate, proof, func(tx pgx.Tx) error {
		var err error
		operation, err = saveMatchSquadProvisioningIntentTx(ctx, tx, intent, true)
		return err
	})
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	return operation, nil
}

func (s *MatchStore) AuthorizeMatchSquadProviderDispatch(ctx context.Context, candidate ProvisionCandidate, proof ProvisionValidationProof, operationID uuid.UUID, provider string) (MatchSquadDispatchTicket, error) {
	if operationID == uuid.Nil || (provider != "chat" && provider != "voice") {
		return MatchSquadDispatchTicket{}, ErrMatchSquadConflict
	}
	var ticket MatchSquadDispatchTicket
	err := s.withValidatedProvisionCutTx(ctx, candidate, proof, func(tx pgx.Tx) error {
		if mismatch, err := matchSquadCreationMismatchTx(ctx, tx, candidate.MatchID); err != nil {
			return err
		} else if mismatch {
			return ErrMatchSquadCreationMismatch
		}
		operation, err := scanMatchSquadProvisioningOperation(tx.QueryRow(ctx, `
		 SELECT match_id,operation_id,participant_manifest_sha256,participant_manifest_bytes,state,
		 chat_operation_id,chat_request_sha256,chat_request_bytes,chat_receipt_id,chat_receipt_bytes,chat_id,
		 voice_operation_id,voice_request_sha256,voice_request_bytes,voice_receipt_id,voice_receipt_bytes,voice_room_id
		 FROM matchmaking_match_squad_operations WHERE match_id=$1`, candidate.MatchID))
		if err != nil {
			return err
		}
		var protocol int16
		var disposition, dispatch string
		var firstID *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT dispatch_protocol_version,provision_disposition,
		 CASE WHEN $2='chat' THEN chat_dispatch_state ELSE voice_dispatch_state END,
		 CASE WHEN $2='chat' THEN chat_first_dispatch_id ELSE voice_first_dispatch_id END
		 FROM matchmaking_match_squad_operations WHERE match_id=$1`, candidate.MatchID, provider).
			Scan(&protocol, &disposition, &dispatch, &firstID); err != nil {
			return err
		}
		// Zero preserves the origin of an operation classified by021's
		// conservative historical backfill. It still needs this exact fresh
		// source/ban/candidate transaction before any LIVE retry.
		if operation.OperationID != operationID || (protocol != 0 && protocol != 1) || disposition != "LIVE" || operation.State != "provisioning" {
			return ErrMatchSquadPending
		}
		if dispatch == "RECEIPT_KNOWN" || (provider == "voice" && (operation.ChatReceiptID == nil || len(operation.VoiceRequestHash) != 32 || len(operation.VoiceRequestBytes) == 0)) {
			return ErrMatchSquadPending
		}
		if dispatch == "NOT_ISSUED" {
			id := uuid.New()
			firstID = &id
			query := `UPDATE matchmaking_match_squad_operations SET chat_dispatch_state='MAY_HAVE_CREATED',
			 chat_first_dispatch_id=$2,chat_first_dispatch_at=clock_timestamp(),provision_revision=provision_revision+1,
			 provision_last_progress_at=clock_timestamp(),updated_at=clock_timestamp() WHERE match_id=$1`
			if provider == "voice" {
				query = `UPDATE matchmaking_match_squad_operations SET voice_dispatch_state='MAY_HAVE_CREATED',
				 voice_first_dispatch_id=$2,voice_first_dispatch_at=clock_timestamp(),provision_revision=provision_revision+1,
				 provision_last_progress_at=clock_timestamp(),updated_at=clock_timestamp() WHERE match_id=$1`
			}
			if _, err := tx.Exec(ctx, query, candidate.MatchID, id); err != nil {
				return err
			}
		} else if dispatch != "MAY_HAVE_CREATED" {
			return ErrMatchSquadConflict
		} else {
			// Every successful authorization consumes the exact local cut. A
			// retry keeps its permanent first-dispatch binding but cannot reuse
			// the previous attempt's source proof when the marker already exists.
			if _, err := tx.Exec(ctx, `UPDATE matchmaking_match_squad_operations
			 SET provision_revision=provision_revision+1,updated_at=clock_timestamp() WHERE match_id=$1`, candidate.MatchID); err != nil {
				return err
			}
		}
		if firstID == nil || *firstID == uuid.Nil {
			return ErrMatchSquadConflict
		}
		ticket = MatchSquadDispatchTicket{Operation: operation, Provider: provider, FirstDispatchID: *firstID}
		return nil
	})
	if err != nil {
		return MatchSquadDispatchTicket{}, err
	}
	return ticket, nil
}

func (s *MatchStore) ActivateValidatedProvisionedMatch(ctx context.Context, candidate ProvisionCandidate, proof ProvisionValidationProof, operationID uuid.UUID) (Match, error) {
	if operationID == uuid.Nil {
		return Match{}, ErrMatchSquadConflict
	}
	var activated Match
	err := s.withValidatedProvisionCutTx(ctx, candidate, proof, func(tx pgx.Tx) error {
		if mismatch, err := matchSquadCreationMismatchTx(ctx, tx, candidate.MatchID); err != nil {
			return err
		} else if mismatch {
			return ErrMatchSquadCreationMismatch
		}
		var persistedID uuid.UUID
		var protocol int16
		var disposition, state, chat, voice string
		if err := tx.QueryRow(ctx, `SELECT operation_id,dispatch_protocol_version,provision_disposition,state,chat_dispatch_state,voice_dispatch_state
		 FROM matchmaking_match_squad_operations WHERE match_id=$1`, candidate.MatchID).
			Scan(&persistedID, &protocol, &disposition, &state, &chat, &voice); err != nil {
			return err
		}
		if persistedID != operationID || (protocol != 0 && protocol != 1) || disposition != "LIVE" || state != "provisioning" || chat != "RECEIPT_KNOWN" || voice != "RECEIPT_KNOWN" {
			return ErrMatchSquadPending
		}
		var err error
		activated, err = activateProvisionedMatchTx(ctx, tx, candidate.MatchID, true)
		return err
	})
	if err != nil {
		return Match{}, err
	}
	return activated, nil
}

func validMatchSquadIntentFields(intent MatchSquadProvisioningIntent) bool {
	return intent.MatchID != uuid.Nil && intent.OperationID != uuid.Nil && intent.ChatOperationID != uuid.Nil && intent.VoiceOperationID != uuid.Nil &&
		len(intent.ParticipantManifestBytes) > 0 && len(intent.ChatRequestBytes) > 0 &&
		bytes.Equal(sumSHA256(intent.ParticipantManifestBytes), intent.ParticipantManifestHash) &&
		bytes.Equal(sumSHA256(intent.ChatRequestBytes), intent.ChatRequestHash)
}

func (s *MatchStore) withValidatedProvisionCutTx(ctx context.Context, candidate ProvisionCandidate, proof ProvisionValidationProof, transition func(pgx.Tx) error) error {
	if s == nil || s.Pool == nil || candidate.MatchID == uuid.Nil || candidate.Hash() == ([32]byte{}) || proof == nil ||
		proof.Hash() != candidate.Hash() || !proof.Matches(candidate) {
		return ErrProvisionCandidateChanged
	}
	accounts := candidate.Accounts()
	parties := candidate.Parties()
	if len(accounts) == 0 || len(parties) == 0 {
		return ErrSourcePartyUnavailable
	}
	partyIDs := make([]uuid.UUID, 0, len(parties))
	for _, party := range parties {
		partyIDs = append(partyIDs, party.ID)
	}
	nativeBinding := copyNativeBinding(s.NativeAdmission)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := mmadmission.Acquire(ctx, tx, nativeBinding); err != nil {
		return err
	}
	// The admission gate above stays held through this transaction's COMMIT.
	guard, err := AcquirePlatformBanGuard(ctx, tx, accounts)
	if err != nil {
		return err
	}
	if err := lockMatchSquadReceiptMatchTx(ctx, tx, candidate.MatchID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM parties WHERE id=ANY($1::uuid[]) ORDER BY id FOR UPDATE`, partyIDs); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM search_sessions WHERE match_id=$1 OR party_id=ANY($2::uuid[]) ORDER BY id FOR UPDATE`, candidate.MatchID, partyIDs); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM match_proposals WHERE match_id=$1 ORDER BY id FOR UPDATE`, candidate.MatchID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT match_id FROM matchmaking_match_squad_operations WHERE match_id=$1 FOR UPDATE`, candidate.MatchID); err != nil {
		return err
	}
	if err := guard.Check(ctx); err != nil {
		return err
	}
	if err := VerifyProvisionCandidateTx(ctx, tx, candidate); err != nil {
		return err
	}
	if err := transition(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
