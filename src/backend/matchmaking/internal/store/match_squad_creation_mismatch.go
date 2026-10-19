package store

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrMatchSquadCreationMismatch = errors.New("match squad creation requires operator reconciliation")

// RecordMatchSquadCreationMismatch retains the first exact contradictory response
// observed by the protected coordinator, bound to its immutable frozen request.
// A transport error or terminal status without response bytes is not evidence.
// The original operation may already be closed: recording a late contradiction
// never reopens it or changes its sealed provider obligation set.
func (s *MatchStore) RecordMatchSquadCreationMismatch(ctx context.Context, matchID, operationID uuid.UUID, provider string, requestHash, response []byte) error {
	if s == nil || s.Pool == nil || matchID == uuid.Nil || operationID == uuid.Nil || len(requestHash) != 32 || len(response) == 0 || (provider != "chat" && provider != "voice") {
		return ErrMatchSquadConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockMatchSquadReceiptMatchTx(ctx, tx, matchID); err != nil {
		return err
	}
	var actualID uuid.UUID
	var frozen, prior []byte
	err = tx.QueryRow(ctx, `SELECT operation_id,CASE WHEN $2='chat' THEN chat_request_sha256 ELSE voice_request_sha256 END,
 CASE WHEN $2='chat' THEN chat_creation_mismatch_bytes ELSE voice_creation_mismatch_bytes END
 FROM matchmaking_match_squad_operations WHERE match_id=$1 FOR UPDATE`, matchID, provider).Scan(&actualID, &frozen, &prior)
	if err != nil {
		return err
	}
	if actualID != operationID || !bytes.Equal(frozen, requestHash) {
		return ErrMatchSquadConflict
	}
	if len(prior) > 0 {
		if !bytes.Equal(prior, response) {
			return ErrMatchSquadCreationMismatch
		}
		return tx.Commit(ctx)
	}
	query := `UPDATE matchmaking_match_squad_operations SET chat_creation_mismatch_bytes=$2,chat_creation_mismatch_sha256=$3,chat_creation_mismatch_at=clock_timestamp(),provision_revision=provision_revision+1 WHERE match_id=$1`
	if provider == "voice" {
		query = `UPDATE matchmaking_match_squad_operations SET voice_creation_mismatch_bytes=$2,voice_creation_mismatch_sha256=$3,voice_creation_mismatch_at=clock_timestamp(),provision_revision=provision_revision+1 WHERE match_id=$1`
	}
	if _, err = tx.Exec(ctx, query, matchID, response, sumSHA256(response)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CheckMatchSquadCreationEvidence fails closed if the new evidence schema is
// absent. Cleanup receipt persistence uses the compatible transaction helper.
func (s *MatchStore) CheckMatchSquadCreationEvidence(ctx context.Context, matchID uuid.UUID) error {
	if s == nil || s.Pool == nil || matchID == uuid.Nil {
		return ErrSourcePartyUnavailable
	}
	var version string
	var mismatch bool
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(to_jsonb(o)->>'creation_evidence_version',''),
 (to_jsonb(o)->>'chat_creation_mismatch_bytes' IS NOT NULL OR to_jsonb(o)->>'voice_creation_mismatch_bytes' IS NOT NULL)
 FROM matchmaking_match_squad_operations o WHERE match_id=$1`, matchID).Scan(&version, &mismatch)
	if err != nil {
		return err
	}
	if version != "1" {
		return ErrSourcePartyUnavailable
	}
	if mismatch {
		return ErrMatchSquadCreationMismatch
	}
	return nil
}

func matchSquadCreationMismatchTx(ctx context.Context, tx pgx.Tx, matchID uuid.UUID) (bool, error) {
	var mismatch bool
	err := tx.QueryRow(ctx, `SELECT (to_jsonb(o)->>'chat_creation_mismatch_bytes' IS NOT NULL OR to_jsonb(o)->>'voice_creation_mismatch_bytes' IS NOT NULL)
 FROM matchmaking_match_squad_operations o WHERE match_id=$1`, matchID).Scan(&mismatch)
	return mismatch, err
}
