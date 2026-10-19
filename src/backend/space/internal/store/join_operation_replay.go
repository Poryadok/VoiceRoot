package store

import (
	"bytes"
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// JoinOperationReplay is saved technical outcome data, not fresh authority.
type JoinOperationReplay struct {
	Operation JoinOperation
	Code      *int32
	Proto     []byte
}

func (s *SpaceStore) LookupJoinOperationReplay(ctx context.Context, in JoinOperationInput) (*JoinOperationReplay, bool, error) {
	if s == nil || s.Pool == nil || !validJoinOperation(in) {
		return nil, false, ErrJoinAttemptUnavailable
	}
	var result JoinOperationReplay
	o := &result.Operation
	err := s.db().QueryRow(ctx, `SELECT `+joinOperationColumns+`,outcome_code,outcome_proto FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID).Scan(&o.Input.SpaceID, &o.Input.ActorAccountID, &o.Input.ActorProfileID, &o.Input.OperationID, &o.Input.Method, &o.Input.RequestSHA256, &o.AttemptID, &o.State, &result.Code, &result.Proto)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	if !bytes.Equal(joinDigest("public-operation", o.Input), joinDigest("public-operation", in)) {
		return nil, true, ErrJoinAttemptConflict
	}
	if (o.State == "COMPLETED") != (result.Code != nil && result.Proto != nil) || (result.Code == nil) != (result.Proto == nil) {
		return nil, true, ErrJoinAttemptUnavailable
	}
	return &result, true, nil
}
