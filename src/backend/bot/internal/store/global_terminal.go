package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

type BotGlobalTerminalEffect struct {
	Participant, State         string
	RequestBytes, ReceiptBytes []byte
}

func globalDeletion(claim *BotDeletion) bool {
	return claim != nil && claim.OperationID != uuid.Nil && claim.BotID != uuid.Nil && (claim.Purpose == "" || claim.Purpose == "global_delete")
}

func (s *BotStore) GlobalTerminalEffects(ctx context.Context, claim *BotDeletion) ([]BotGlobalTerminalEffect, error) {
	if !globalDeletion(claim) {
		return nil, ErrInstallationConflict
	}
	rows, err := s.Pool.Query(ctx, `SELECT e.participant,e.state,e.request_bytes,e.request_sha256,e.receipt_bytes,e.receipt_sha256 FROM bot_deletion_terminal_effects e JOIN bot_deletion_operations o USING(operation_id) WHERE o.operation_id=$1 AND o.bot_id=$2 ORDER BY e.participant`, claim.OperationID, claim.BotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BotGlobalTerminalEffect{}
	for rows.Next() {
		var e BotGlobalTerminalEffect
		var requestSHA, receiptSHA []byte
		if err = rows.Scan(&e.Participant, &e.State, &e.RequestBytes, &requestSHA, &e.ReceiptBytes, &receiptSHA); err != nil {
			return nil, err
		}
		if (e.State == "COMPLETE") != (e.ReceiptBytes != nil) || (e.ReceiptBytes != nil && e.RequestBytes == nil) {
			return nil, ErrInstallationConflict
		}
		for _, pair := range [][2][]byte{{e.RequestBytes, requestSHA}, {e.ReceiptBytes, receiptSHA}} {
			digest := sha256.Sum256(pair[0])
			if (pair[0] == nil) != (pair[1] == nil) || (pair[0] != nil && !bytes.Equal(digest[:], pair[1])) {
				return nil, ErrInstallationConflict
			}
		}
		out = append(out, e)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(out) != 2 || out[0].Participant != "role" || out[1].Participant != "space" {
		return nil, ErrInstallationConflict
	}
	return out, nil
}

func validateGlobalSpaceRequest(claim *BotDeletion, body []byte) error {
	if !globalDeletion(claim) || len(body) == 0 || len(body) > 1024 {
		return ErrInstallationConflict
	}
	q := new(spacev1.RegisterBotGlobalTerminalRequest)
	if proto.Unmarshal(body, q) != nil || len(q.ProtoReflect().GetUnknown()) != 0 || q.ProtocolVersion != 1 || q.BotId != claim.BotID.String() || q.DecisionId != claim.OperationID.String() {
		return ErrInstallationConflict
	}
	canonical, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil || !bytes.Equal(canonical, body) {
		return ErrInstallationConflict
	}
	return nil
}

func (s *BotStore) globalTerminalClaimTx(ctx context.Context, claim *BotDeletion) (pgx.Tx, error) {
	if !globalDeletion(claim) {
		return nil, ErrInstallationConflict
	}
	tx, err := s.deletionClaimTx(ctx, claim)
	if err != nil {
		return nil, err
	}
	var decision, digest []byte
	var bot uuid.UUID
	var status string
	// This is a read under the already-held root claim, not a Bot lock below
	// root. Legitimate Bot writers cannot restore live state past this root.
	err = tx.QueryRow(ctx, `SELECT o.bot_id,o.decision_bytes,o.decision_sha256,b.status FROM bot_deletion_operations o JOIN bots b ON b.id=o.bot_id WHERE o.operation_id=$1`, claim.OperationID).Scan(&bot, &decision, &digest, &status)
	actual := sha256.Sum256(decision)
	if err != nil || status != "disabled" || bot != claim.BotID || !bytes.Equal(decision, claim.DecisionBytes) || !bytes.Equal(digest, claim.DecisionSHA256) || !bytes.Equal(actual[:], digest) {
		_ = tx.Rollback(context.Background())
		if err != nil {
			return nil, err
		}
		return nil, ErrInstallationConflict
	}
	return tx, nil
}

func (s *BotStore) SealBotGlobalSpaceRequest(ctx context.Context, claim *BotDeletion, body []byte) error {
	body = bytes.Clone(body)
	if err := validateGlobalSpaceRequest(claim, body); err != nil {
		return err
	}
	tx, err := s.globalTerminalClaimTx(ctx, claim)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var saved, savedSHA []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,request_sha256 FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND participant='space' FOR UPDATE`, claim.OperationID).Scan(&saved, &savedSHA)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInstallationConflict
	}
	if err != nil {
		return err
	}
	if saved != nil {
		digest := sha256.Sum256(saved)
		if !bytes.Equal(saved, body) || !bytes.Equal(digest[:], savedSHA) {
			return ErrInstallationConflict
		}
		if err = s.lockLifecycleClaim(ctx, tx, claim); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if savedSHA != nil {
		return ErrInstallationConflict
	}
	if err = s.lockLifecycleClaim(ctx, tx, claim); err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	_, err = tx.Exec(ctx, `UPDATE bot_deletion_terminal_effects SET request_bytes=$2,request_sha256=$3 WHERE operation_id=$1 AND participant='space'`, claim.OperationID, body, digest[:])
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *BotStore) SaveBotGlobalSpaceReceipt(ctx context.Context, claim *BotDeletion, body, receipt []byte) error {
	body, receipt = bytes.Clone(body), bytes.Clone(receipt)
	if err := validateGlobalSpaceRequest(claim, body); err != nil {
		return err
	}
	if len(receipt) == 0 || len(receipt) > 4096 {
		return ErrInstallationConflict
	}
	r := new(spacev1.RegisterBotGlobalTerminalResponse)
	if proto.Unmarshal(receipt, r) != nil || len(r.ProtoReflect().GetUnknown()) != 0 || r.Receipt == nil {
		return ErrInstallationConflict
	}
	proof := r.Receipt
	id, idErr := uuid.Parse(proof.ReceiptId)
	canonical, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	if err != nil || !bytes.Equal(canonical, receipt) || len(proof.ProtoReflect().GetUnknown()) != 0 || proof.ProtocolVersion != 1 || proof.BotId != claim.BotID.String() || proof.DecisionId != claim.OperationID.String() || idErr != nil || id == uuid.Nil || id.String() != proof.ReceiptId || proof.RegisteredAt == nil || proof.RegisteredAt.CheckValid() != nil || proof.RegisteredAt.Seconds <= 0 || len(proof.RegisteredAt.ProtoReflect().GetUnknown()) != 0 || proof.Outcome != spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED {
		return ErrInstallationConflict
	}
	tx, err := s.globalTerminalClaimTx(ctx, claim)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var savedRequest, savedRequestSHA, savedReceipt, savedReceiptSHA []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,request_sha256,receipt_bytes,receipt_sha256 FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND participant='space' FOR UPDATE`, claim.OperationID).Scan(&savedRequest, &savedRequestSHA, &savedReceipt, &savedReceiptSHA)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInstallationConflict
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(savedRequest, body) || savedRequest == nil || (savedReceipt != nil && !bytes.Equal(savedReceipt, receipt)) {
		return ErrInstallationConflict
	}
	requestDigest, receiptDigest := sha256.Sum256(savedRequest), sha256.Sum256(savedReceipt)
	if !bytes.Equal(requestDigest[:], savedRequestSHA) || (savedReceipt == nil) != (savedReceiptSHA == nil) || (savedReceipt != nil && !bytes.Equal(receiptDigest[:], savedReceiptSHA)) {
		return ErrInstallationConflict
	}
	if err = s.lockLifecycleClaim(ctx, tx, claim); err != nil {
		return err
	}
	if savedReceipt == nil {
		digest := sha256.Sum256(receipt)
		_, err = tx.Exec(ctx, `UPDATE bot_deletion_terminal_effects SET receipt_bytes=$2,receipt_sha256=$3,state='COMPLETE' WHERE operation_id=$1 AND participant='space'`, claim.OperationID, receipt, digest[:])
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
