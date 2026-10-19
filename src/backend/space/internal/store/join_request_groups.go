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

var ErrJoinRequestAmbiguous = errors.New("join request requires an explicit attempt")

type JoinRequestGroup struct {
	ID, SpaceID, AccountID, ProfileID uuid.UUID
	PolicyVersion                     int64
}

type joinRequestGroupRow struct {
	JoinRequestGroup
	policySHA256 []byte
}

const joinRequestGroupColumns = `id,space_id,account_id,profile_id,policy_version,policy_sha256`

func scanJoinRequestGroup(row pgx.Row) (*joinRequestGroupRow, error) {
	var group joinRequestGroupRow
	err := row.Scan(&group.ID, &group.SpaceID, &group.AccountID, &group.ProfileID, &group.PolicyVersion, &group.policySHA256)
	return &group, err
}

func (g *joinRequestGroupRow) contains(a *joinAttemptPrivate) bool {
	return g != nil && a != nil && g.SpaceID == a.Input.SpaceID && g.AccountID == a.Input.AccountID &&
		g.ProfileID == a.Input.ProfileID && g.PolicyVersion == a.PolicyVersion && bytes.Equal(g.policySHA256, a.policyDigest)
}

// LookupJoinRequestGroupForAttempt reads an existing association. It neither
// attaches an attempt nor establishes approval, visibility, or admission rights.
func (s *SpaceStore) LookupJoinRequestGroupForAttempt(ctx context.Context, space, attempt uuid.UUID) (*JoinRequestGroup, error) {
	if ctx == nil || s == nil || space == uuid.Nil || attempt == uuid.Nil {
		return nil, ErrJoinAttemptUnavailable
	}
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{space}, func(local *SpaceStore) (*JoinRequestGroup, error) {
		if err := lockAndCheckOwnershipScope(ctx, local.tx, []uuid.UUID{space}); err != nil {
			return nil, err
		}
		var request uuid.UUID
		err := local.db().QueryRow(ctx, `SELECT request_id FROM space_join_request_attempts WHERE attempt_id=$1 AND space_id=$2`, attempt, space).Scan(&request)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJoinAttemptDenied
		}
		if err != nil {
			return nil, err
		}
		group, err := local.checkedJoinRequestGroup(ctx, space, request)
		if err != nil {
			return nil, err
		}
		return &group.JoinRequestGroup, nil
	})
}

// AttachJoinRequestAttempt is bookkeeping in the successful pending submission
// transaction. Q contains neither proof nor approval and cannot authorize A.
func (s *SpaceStore) AttachJoinRequestAttempt(ctx context.Context, attempt uuid.UUID) (*JoinRequestGroup, error) {
	if ctx == nil || s == nil || s.tx == nil || attempt == uuid.Nil {
		return nil, ErrJoinAttemptUnavailable
	}
	var before uuid.UUID
	if err := s.db().QueryRow(ctx, `SELECT space_id FROM space_join_attempts WHERE id=$1`, attempt).Scan(&before); err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if err := lockAndCheckOwnershipScope(ctx, s.tx, []uuid.UUID{before}); err != nil {
		return nil, err
	}
	a, err := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, attempt))
	if err != nil || a.Input.SpaceID != before {
		return nil, ErrJoinAttemptUnavailable
	}
	group, err := scanJoinRequestGroup(s.db().QueryRow(ctx, `SELECT g.id,g.space_id,g.account_id,g.profile_id,g.policy_version,g.policy_sha256 FROM space_join_request_groups g JOIN space_join_request_attempts c ON c.request_id=g.id WHERE c.attempt_id=$1`, attempt))
	if err == nil {
		if !group.contains(a) {
			return nil, ErrJoinAttemptUnavailable
		}
		return &group.JoinRequestGroup, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if a.State != "PENDING" && a.State != "READY" {
		return nil, ErrJoinAttemptDenied
	}
	policy, err := entrypolicy.Parse(a.policyRaw)
	if err != nil || !policy.RequireManualApproval || !bytes.Equal(joinDigest("policy", policy), a.policyDigest) {
		return nil, ErrJoinAttemptUnavailable
	}
	group, err = scanJoinRequestGroup(s.db().QueryRow(ctx, `INSERT INTO space_join_request_groups(id,space_id,account_id,profile_id,policy_version,policy_sha256) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(space_id,account_id,profile_id,policy_version,policy_sha256) DO NOTHING RETURNING `+joinRequestGroupColumns, uuid.New(), a.Input.SpaceID, a.Input.AccountID, a.Input.ProfileID, a.PolicyVersion, a.policyDigest))
	if errors.Is(err, pgx.ErrNoRows) {
		group, err = scanJoinRequestGroup(s.db().QueryRow(ctx, `SELECT `+joinRequestGroupColumns+` FROM space_join_request_groups WHERE space_id=$1 AND account_id=$2 AND profile_id=$3 AND policy_version=$4 AND policy_sha256=$5`, a.Input.SpaceID, a.Input.AccountID, a.Input.ProfileID, a.PolicyVersion, a.policyDigest))
	}
	if err != nil {
		return nil, err
	}
	_, err = s.db().Exec(ctx, `INSERT INTO space_join_request_attempts(attempt_id,request_id,space_id) VALUES($1,$2,$3)`, a.ID, group.ID, group.SpaceID)
	if err != nil {
		return nil, err
	}
	return &group.JoinRequestGroup, nil
}

// ResolveJoinRequestApproval fixes Q and exact A atomically with the existing
// public operation reservation BEFORE remote observations. It is not approval.
func (s *SpaceStore) ResolveJoinRequestApproval(ctx context.Context, op JoinOperationInput, requestID uuid.UUID, explicitA *uuid.UUID) (*JoinOperation, error) {
	return s.resolveJoinRequestApproval(ctx, op, requestID, explicitA, false)
}

// Observation may select a claimed or published original winner. It does not
// approve, publish, or make the observational operation a new decision owner.
func (s *SpaceStore) resolveJoinRequestApprovalObservation(ctx context.Context, op JoinOperationInput, requestID uuid.UUID, explicitA *uuid.UUID) (*JoinOperation, error) {
	return s.resolveJoinRequestApproval(ctx, op, requestID, explicitA, true)
}

func (s *SpaceStore) resolveJoinRequestApproval(ctx context.Context, op JoinOperationInput, requestID uuid.UUID, explicitA *uuid.UUID, observation bool) (*JoinOperation, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if !validJoinOperation(op) || op.Method != JoinOperationApprove || requestID == uuid.Nil || (explicitA != nil && *explicitA == uuid.Nil) {
		return nil, entrypolicy.ErrInvalid
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	// Same namespace/order as69 ReserveJoinOperation; never lock it from a
	// preexisting Space-scoped transaction.
	key := joinDigest("public-operation-lock", struct{ Actor, Operation uuid.UUID }{op.ActorProfileID, op.OperationID})
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(binary.BigEndian.Uint32(key[:4])), int32(binary.BigEndian.Uint32(key[4:8]))); err != nil {
		return nil, err
	}
	saved, err := scanJoinOperation(tx.QueryRow(ctx, `SELECT `+joinOperationColumns+` FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID))
	scoped := *s
	scoped.tx = tx
	if err == nil {
		if !sameJoinRequestOperation(saved.Input, op) {
			return nil, ErrJoinAttemptConflict
		}
		selection, err := scoped.loadJoinRequestSelection(ctx, op)
		if err != nil || saved.AttemptID == nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if !selection.matches(op, requestID, *saved.AttemptID) || !sameOptionalJoinAttempt(selection.explicitA, explicitA) {
			return nil, ErrJoinAttemptConflict
		}
		// Historical selection stays pinned even when another child appears or
		// A later becomes ineligible. The actual approval must use fresh guards.
		return saved, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{op.SpaceID}); err != nil {
		return nil, err
	}
	group, err := scoped.checkedJoinRequestGroup(ctx, op.SpaceID, requestID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT a.id FROM space_join_request_attempts c JOIN space_join_attempts a ON a.id=c.attempt_id WHERE c.request_id=$1 AND (a.state IN ('PENDING','READY') OR ($3 AND a.state IN ('CLAIMED','PUBLISHED'))) AND ($2::uuid IS NULL OR a.id=$2) ORDER BY a.id LIMIT 2`, group.ID, explicitA, observation)
	if err != nil {
		return nil, err
	}
	var candidates []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, ErrJoinAttemptDenied
	}
	if len(candidates) != 1 {
		return nil, ErrJoinRequestAmbiguous
	}
	attempt := candidates[0]
	_, err = tx.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,selected_attempt_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, op.ActorProfileID, op.OperationID, op.ActorAccountID, op.SpaceID, op.Method, op.RequestSHA256, attempt)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_join_request_selections(actor_profile_id,operation_id,actor_account_id,space_id,request_id,selected_attempt_id,explicit_attempt_id,request_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, op.ActorProfileID, op.OperationID, op.ActorAccountID, op.SpaceID, requestID, attempt, explicitA, op.RequestSHA256)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &JoinOperation{Input: op, AttemptID: &attempt, State: "RESERVED"}, nil
}

type joinRequestSelection struct {
	space, account, request, attempt uuid.UUID
	explicitA                        *uuid.UUID
	requestSHA256                    []byte
}

func (s *SpaceStore) loadJoinRequestSelection(ctx context.Context, op JoinOperationInput) (*joinRequestSelection, error) {
	var selected joinRequestSelection
	err := s.db().QueryRow(ctx, `SELECT space_id,actor_account_id,request_id,selected_attempt_id,explicit_attempt_id,request_sha256 FROM space_join_request_selections WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&selected.space, &selected.account, &selected.request, &selected.attempt, &selected.explicitA, &selected.requestSHA256)
	return &selected, err
}

func (s *joinRequestSelection) matches(op JoinOperationInput, request, attempt uuid.UUID) bool {
	return s.space == op.SpaceID && s.account == op.ActorAccountID && s.request == request && s.attempt == attempt && bytes.Equal(s.requestSHA256, op.RequestSHA256)
}

func sameOptionalJoinAttempt(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func sameJoinRequestOperation(a, b JoinOperationInput) bool {
	return bytes.Equal(joinDigest("public-operation", a), joinDigest("public-operation", b))
}

// checkedJoinRequestGroup rejects inconsistent/missing children instead of
// filtering them out and silently selecting a different otherwise valid A.
func (s *SpaceStore) checkedJoinRequestGroup(ctx context.Context, space, request uuid.UUID) (*joinRequestGroupRow, error) {
	group, err := scanJoinRequestGroup(s.db().QueryRow(ctx, `SELECT `+joinRequestGroupColumns+` FROM space_join_request_groups WHERE id=$1 AND space_id=$2`, request, space))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJoinAttemptDenied
	}
	if err != nil {
		return nil, err
	}
	var inconsistent bool
	err = s.db().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_join_request_attempts c LEFT JOIN space_join_attempts a ON a.id=c.attempt_id WHERE c.request_id=$1 AND (a.id IS NULL OR c.space_id IS DISTINCT FROM $2 OR a.space_id IS DISTINCT FROM $2 OR a.account_id IS DISTINCT FROM $3 OR a.profile_id IS DISTINCT FROM $4 OR a.policy_version IS DISTINCT FROM $5 OR a.policy_sha256 IS DISTINCT FROM $6))`, request, space, group.AccountID, group.ProfileID, group.PolicyVersion, group.policySHA256).Scan(&inconsistent)
	if err != nil {
		return nil, err
	}
	if inconsistent {
		return nil, ErrJoinAttemptUnavailable
	}
	return group, nil
}

// VerifyJoinRequestApprovalSelection is a local final-hook check, not a current
// owner allow. It never takes an actor/operation lock after the Space fence.
func (s *SpaceStore) VerifyJoinRequestApprovalSelection(ctx context.Context, op JoinOperationInput, requestID, attempt uuid.UUID) error {
	if ctx == nil || s == nil || s.tx == nil || !validJoinOperation(op) || op.Method != JoinOperationApprove || requestID == uuid.Nil || attempt == uuid.Nil {
		return ErrJoinAttemptUnavailable
	}
	if err := lockAndCheckOwnershipScope(ctx, s.tx, []uuid.UUID{op.SpaceID}); err != nil {
		return err
	}
	saved, err := scanJoinOperation(s.db().QueryRow(ctx, `SELECT `+joinOperationColumns+` FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID))
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	if !sameJoinRequestOperation(saved.Input, op) || saved.AttemptID == nil || *saved.AttemptID != attempt {
		return ErrJoinAttemptConflict
	}
	selection, err := s.loadJoinRequestSelection(ctx, op)
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	if !selection.matches(op, requestID, attempt) {
		return ErrJoinAttemptConflict
	}
	if _, err = s.checkedJoinRequestGroup(ctx, op.SpaceID, requestID); err != nil {
		return err
	}
	var attached bool
	if err = s.db().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_join_request_attempts WHERE request_id=$1 AND attempt_id=$2 AND space_id=$3)`, requestID, attempt, op.SpaceID).Scan(&attached); err != nil {
		return err
	}
	if !attached {
		return ErrJoinAttemptUnavailable
	}
	return nil
}
