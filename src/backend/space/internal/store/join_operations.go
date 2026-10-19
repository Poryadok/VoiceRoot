package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/entrypolicy"
)

// JoinOperationInput is replay metadata supplied by the protected adapter after
// exact request authentication. Neither it nor a saved row grants admission.
type JoinOperationInput struct {
	SpaceID, ActorAccountID, ActorProfileID, OperationID uuid.UUID
	Method                                               string
	RequestSHA256                                        []byte
}

const (
	JoinOperationStart   = "/voice.space.v1.SpaceService/StartJoinAttempt"
	JoinOperationSubmit  = "/voice.space.v1.SpaceService/SubmitJoinAttempt"
	JoinOperationApprove = "/voice.space.v1.SpaceService/ApproveJoinRequest"
)

type JoinOperation struct {
	Input     JoinOperationInput
	AttemptID *uuid.UUID
	State     string
}

func validJoinOperation(in JoinOperationInput) bool {
	return in.SpaceID != uuid.Nil && in.ActorAccountID != uuid.Nil && in.ActorProfileID != uuid.Nil && in.OperationID != uuid.Nil && len(in.RequestSHA256) == 32 &&
		(in.Method == JoinOperationStart || in.Method == JoinOperationSubmit || in.Method == JoinOperationApprove)
}

const joinOperationColumns = `space_id,actor_account_id,actor_profile_id,operation_id,request_method,request_sha256,selected_attempt_id,state`

func scanJoinOperation(row pgx.Row) (*JoinOperation, error) {
	var op JoinOperation
	err := row.Scan(&op.Input.SpaceID, &op.Input.ActorAccountID, &op.Input.ActorProfileID, &op.Input.OperationID, &op.Input.Method, &op.Input.RequestSHA256, &op.AttemptID, &op.State)
	return &op, err
}

// ReserveJoinOperation fixes the public operation namespace before owner RPCs.
// Exact existing reservation reads do not refresh policy or choose a new A.
func (s *SpaceStore) ReserveJoinOperation(ctx context.Context, in JoinOperationInput, attempt *uuid.UUID) (*JoinOperation, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if !validJoinOperation(in) || (attempt != nil && *attempt == uuid.Nil) {
		return nil, entrypolicy.ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		cleanup, cancel := BoundedCleanupContext(ctx)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	key := joinDigest("public-operation-lock", struct{ Actor, Operation uuid.UUID }{in.ActorProfileID, in.OperationID})
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(binary.BigEndian.Uint32(key[:4])), int32(binary.BigEndian.Uint32(key[4:8]))); err != nil {
		return nil, err
	}
	saved, err := scanJoinOperation(tx.QueryRow(ctx, `SELECT `+joinOperationColumns+` FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID))
	if err == nil {
		if !bytes.Equal(joinDigest("public-operation", saved.Input), joinDigest("public-operation", in)) || (attempt != nil && (saved.AttemptID == nil || *saved.AttemptID != *attempt)) {
			return nil, ErrJoinAttemptConflict
		}
		return saved, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if in.Method != JoinOperationStart && attempt == nil {
		return nil, entrypolicy.ErrInvalid
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	if attempt != nil {
		var accountID, profileID, spaceID uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT space_id,account_id,profile_id FROM space_join_attempts WHERE id=$1`, *attempt).Scan(&spaceID, &accountID, &profileID); err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if spaceID != in.SpaceID || (in.Method != JoinOperationApprove && (accountID != in.ActorAccountID || profileID != in.ActorProfileID)) {
			return nil, ErrJoinAttemptDenied
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,selected_attempt_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, in.ActorProfileID, in.OperationID, in.ActorAccountID, in.SpaceID, in.Method, in.RequestSHA256, attempt)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &JoinOperation{Input: in, AttemptID: attempt, State: "RESERVED"}, nil
}

// SelectJoinOperationAttempt pins Start's server-selected A in the same Space
// transaction that creates/reuses it. It never authorizes or submits that A.
func (s *SpaceStore) SelectJoinOperationAttempt(ctx context.Context, in JoinOperationInput, attempt uuid.UUID) error {
	if s == nil || s.tx == nil || !validJoinOperation(in) || attempt == uuid.Nil {
		return ErrJoinAttemptUnavailable
	}
	if err := lockAndCheckOwnershipScope(ctx, s.tx, []uuid.UUID{in.SpaceID}); err != nil {
		return err
	}
	saved, err := scanJoinOperation(s.db().QueryRow(ctx, `SELECT `+joinOperationColumns+` FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 FOR UPDATE`, in.ActorProfileID, in.OperationID))
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	if !bytes.Equal(joinDigest("public-operation", saved.Input), joinDigest("public-operation", in)) {
		return ErrJoinAttemptConflict
	}
	if saved.AttemptID != nil {
		if *saved.AttemptID != attempt {
			return ErrJoinAttemptConflict
		}
		return nil
	}
	if saved.State != "RESERVED" {
		return ErrJoinAttemptUnavailable
	}
	var accountID, profileID, spaceID uuid.UUID
	if err = s.db().QueryRow(ctx, `SELECT space_id,account_id,profile_id FROM space_join_attempts WHERE id=$1`, attempt).Scan(&spaceID, &accountID, &profileID); err != nil {
		return ErrJoinAttemptUnavailable
	}
	if spaceID != in.SpaceID || (in.Method != JoinOperationApprove && (accountID != in.ActorAccountID || profileID != in.ActorProfileID)) {
		return ErrJoinAttemptDenied
	}
	result, err := s.db().Exec(ctx, `UPDATE space_join_operations SET selected_attempt_id=$3 WHERE actor_profile_id=$1 AND operation_id=$2 AND selected_attempt_id IS NULL AND state='RESERVED'`, in.ActorProfileID, in.OperationID, attempt)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrJoinAttemptUnavailable
	}
	return nil
}
