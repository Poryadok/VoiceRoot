package store

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LatchMatchSquadAbandonmentTx shares the caller's already locked abandonment
// transaction. It never performs provider IO or commits independently.
func LatchMatchSquadAbandonmentTx(ctx context.Context, tx pgx.Tx, matchID uuid.UUID, reason string, evidence [32]byte) error {
	if tx == nil || matchID == uuid.Nil || reason == "" || evidence == ([32]byte{}) {
		return ErrMatchSquadConflict
	}
	// Match first serializes this latch with intent creation and activation.
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM matches WHERE id=$1 FOR UPDATE`, matchID).Scan(&status); err != nil {
		return err
	}
	if status != "abandoned" {
		return ErrMatchSquadConflict
	}
	var disposition, state, chat, voice string
	err := tx.QueryRow(ctx, `SELECT COALESCE(to_jsonb(o)->>'provision_disposition',''),state,
		COALESCE(to_jsonb(o)->>'chat_dispatch_state',''),COALESCE(to_jsonb(o)->>'voice_dispatch_state','')
		FROM matchmaking_match_squad_operations o WHERE match_id=$1 FOR UPDATE`, matchID).Scan(&disposition, &state, &chat, &voice)
	if errors.Is(err, pgx.ErrNoRows) {
		// No operation exists; the abandoned match itself rejects future intent.
		return nil
	}
	if err != nil {
		return err
	}
	if disposition == "" || chat == "" || voice == "" {
		// Existing unmarked intent cannot safely classify possible effects.
		return ErrSourcePartyUnavailable
	}
	required := []string{}
	var chatOp, voiceOp *uuid.UUID
	if chat != "NOT_ISSUED" {
		required = append(required, "chat")
		id := uuid.New()
		chatOp = &id
	}
	if voice != "NOT_ISSUED" {
		required = append(required, "voice")
		id := uuid.New()
		voiceOp = &id
	}
	if disposition == "ABANDONED" {
		if len(required) == 0 {
			if state != "closed" {
				return ErrMatchSquadConflict
			}
			return nil
		}
		var completeBinding bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM matchmaking_match_squad_teardowns a
			WHERE a.match_id=$1 AND a.purpose='PROVISION_COMPENSATION' AND a.required_providers=$2
			AND NOT EXISTS (SELECT 1 FROM unnest(a.required_providers) AS required(provider)
			  WHERE NOT EXISTS (SELECT 1 FROM matchmaking_match_squad_teardown_participants p
			    WHERE p.aggregate_id=a.aggregate_id AND p.provider=required.provider)))`, matchID, required).Scan(&completeBinding); err != nil {
			return err
		}
		if !completeBinding || (state != "compensating" && state != "closed") {
			return ErrMatchSquadConflict
		}
		return nil
	}
	if state != "provisioning" {
		return ErrMatchSquadConflict
	}
	_, err = tx.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET
		provision_disposition='ABANDONED',provision_revision=provision_revision+1,
		provision_abandoned_at=clock_timestamp(),provision_abandon_reason=$2,
		provision_abandon_evidence_sha256=$3,
		state=CASE WHEN cardinality($4::TEXT[])=0 THEN 'closed' ELSE 'compensating' END,
		provision_last_progress_at=clock_timestamp(),updated_at=clock_timestamp() WHERE match_id=$1`, matchID, reason, evidence[:], required)
	if err != nil {
		return err
	}
	if len(required) == 0 {
		// The durable disposition/dispatch evidence is sufficient. There was no
		// possible provider effect, so do not fabricate a teardown aggregate.
		return nil
	}
	aggregateID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO matchmaking_match_squad_teardowns
		(aggregate_id,match_id,state,purpose,required_providers,chat_teardown_operation_id,voice_teardown_operation_id,aggregate_completed_at)
		VALUES ($1,$2,CASE WHEN cardinality($3::TEXT[])=0 THEN 'complete' ELSE 'pending' END,
		'PROVISION_COMPENSATION',$3,$4,$5,CASE WHEN cardinality($3::TEXT[])=0 THEN clock_timestamp() END)`,
		aggregateID, matchID, required, chatOp, voiceOp)
	if err != nil {
		return err
	}
	for _, provider := range required {
		operationID := chatOp
		if provider == "voice" {
			operationID = voiceOp
		}
		if _, err := tx.Exec(ctx, `INSERT INTO matchmaking_match_squad_teardown_participants
			(aggregate_id,provider,operation_id,state) VALUES ($1,$2,$3,'WAITING_FOR_CREATION')`,
			aggregateID, provider, operationID); err != nil {
			return err
		}
	}
	if chat == "RECEIPT_KNOWN" || voice == "RECEIPT_KNOWN" {
		operation, err := scanMatchSquadProvisioningOperation(tx.QueryRow(ctx, `
		 SELECT match_id,operation_id,participant_manifest_sha256,participant_manifest_bytes,state,
		 chat_operation_id,chat_request_sha256,chat_request_bytes,chat_receipt_id,chat_receipt_bytes,chat_id,
		 voice_operation_id,voice_request_sha256,voice_request_bytes,voice_receipt_id,voice_receipt_bytes,voice_room_id
		 FROM matchmaking_match_squad_operations WHERE match_id=$1`, matchID))
		if err != nil {
			return err
		}
		for _, provider := range required {
			if (provider == "chat" && chat == "RECEIPT_KNOWN") || (provider == "voice" && voice == "RECEIPT_KNOWN") {
				if err := materializeMatchSquadCompensationTx(ctx, tx, operation, provider); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func latchMatchAbandonmentCauseTx(ctx context.Context, tx pgx.Tx, matchID uuid.UUID, reason string) error {
	evidence := sha256.Sum256([]byte("match-abandonment-v1:" + matchID.String() + ":" + reason))
	return LatchMatchSquadAbandonmentTx(ctx, tx, matchID, reason, evidence)
}
