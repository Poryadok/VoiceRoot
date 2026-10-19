package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/entrypolicy"
)

// JoinStartSelection is immutable pre-owner replay data. OutcomeProto contains
// only the safe completed START projection, never policy answers or authority.
type JoinStartSelection struct {
	Operation    JoinOperation
	Context      JoinStartSelectionContext
	OutcomeCode  *int32
	OutcomeProto []byte
}

// LookupJoinStartRequest uses the authenticated actor and exact request before
// resolving an invite code that may have changed since the original call.
func (s *SpaceStore) LookupJoinStartRequest(ctx context.Context, account, profile, operation uuid.UUID, requestSHA []byte) (*JoinStartSelection, bool, error) {
	if s == nil || s.Pool == nil || account == uuid.Nil || profile == uuid.Nil || operation == uuid.Nil || !validJoinDecisionDigest(requestSHA) {
		return nil, false, ErrJoinAttemptUnavailable
	}
	var space uuid.UUID
	err := s.db().QueryRow(ctx, `SELECT space_id FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, operation).Scan(&space)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	return s.LookupJoinStartSelection(ctx, JoinOperationInput{SpaceID: space, ActorAccountID: account, ActorProfileID: profile, OperationID: operation, Method: JoinOperationStart, RequestSHA256: requestSHA})
}

func lockJoinPublicOperation(ctx context.Context, tx pgx.Tx, in JoinOperationInput) error {
	key := joinDigest("public-operation-lock", struct{ Actor, Operation uuid.UUID }{in.ActorProfileID, in.OperationID})
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(binary.BigEndian.Uint32(key[:4])), int32(binary.BigEndian.Uint32(key[4:8])))
	return err
}

func (s *SpaceStore) loadJoinStartSelection(ctx context.Context, in JoinOperationInput) (*JoinStartSelection, error) {
	var saved JoinStartSelection
	var raw, digest []byte
	o := &saved.Operation
	err := s.db().QueryRow(ctx, `SELECT `+joinOperationColumns+`,start_selection_context,start_selection_sha256,outcome_code,outcome_proto FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID).Scan(&o.Input.SpaceID, &o.Input.ActorAccountID, &o.Input.ActorProfileID, &o.Input.OperationID, &o.Input.Method, &o.Input.RequestSHA256, &o.AttemptID, &o.State, &raw, &digest, &saved.OutcomeCode, &saved.OutcomeProto)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(joinDigest("public-operation", o.Input), joinDigest("public-operation", in)) {
		return nil, ErrJoinAttemptConflict
	}
	if len(raw) == 0 || len(raw) > 4096 || !validJoinDecisionDigest(digest) {
		return nil, ErrJoinAttemptUnavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&saved.Context) != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, ErrJoinAttemptUnavailable
	}
	sha, err := saved.Context.SHA256()
	if err != nil || !bytes.Equal(sha[:], digest) || !bytes.Equal(joinDigest("public-operation", saved.Context.Operation), joinDigest("public-operation", in)) {
		return nil, ErrJoinAttemptUnavailable
	}
	if o.State == "COMPLETED" {
		if o.AttemptID == nil || saved.OutcomeCode == nil || saved.OutcomeProto == nil {
			return nil, ErrJoinAttemptUnavailable
		}
	} else if o.State != "RESERVED" || saved.OutcomeCode != nil || saved.OutcomeProto != nil || o.AttemptID != nil {
		// The initial flow pins A and the safe outcome atomically. Unknown old
		// partial states cannot be repaired with today's caller or policy.
		return nil, ErrJoinAttemptUnavailable
	}
	return &saved, nil
}

// LookupJoinStartSelection distinguishes an absent operation from unavailable
// provenance. Call it before target/policy reads or any new owner observation.
func (s *SpaceStore) LookupJoinStartSelection(ctx context.Context, in JoinOperationInput) (*JoinStartSelection, bool, error) {
	if s == nil || s.Pool == nil || !validJoinOperation(in) || in.Method != JoinOperationStart {
		return nil, false, ErrJoinAttemptUnavailable
	}
	saved, err := s.loadJoinStartSelection(ctx, in)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	return saved, true, nil
}

// ReserveJoinStartSelection atomically snapshots exact policy/InviteID and the
// actor/op namespace before owner I/O. Existing reservations never reselect.
// This function does not create A, infer Guest, or grant admission.
func (s *SpaceStore) ReserveJoinStartSelection(ctx context.Context, in JoinOperationInput, invite *uuid.UUID) (*JoinStartSelection, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if !validJoinOperation(in) || in.Method != JoinOperationStart || (invite != nil && *invite == uuid.Nil) {
		return nil, entrypolicy.ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockJoinPublicOperation(ctx, tx, in); err != nil {
		return nil, err
	}
	scoped := *s
	scoped.tx = tx
	saved, err := scoped.loadJoinStartSelection(ctx, in)
	if err == nil {
		if (saved.Context.InviteID == nil) != (invite == nil) || (invite != nil && *saved.Context.InviteID != *invite) {
			return nil, ErrJoinAttemptConflict
		}
		return saved, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	policy, err := scoped.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return nil, err
	}
	c := JoinStartSelectionContext{Operation: in, Kind: 1, PolicyVersion: policy.Version, PolicySHA256: joinDigest("policy", policy.Policy)}
	if invite != nil {
		inv, err := scoped.GetInviteByID(ctx, *invite)
		if err != nil {
			return nil, err
		}
		if inv == nil || inv.SpaceID != in.SpaceID {
			return nil, ErrInviteNotFound
		}
		var now time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return nil, err
		}
		if err = scoped.validateInviteForJoin(inv, now); err != nil {
			return nil, err
		}
		id := *invite
		c.Kind = 2
		c.InviteID = &id
	} else {
		var visibility string
		if err = tx.QueryRow(ctx, `SELECT visibility FROM spaces WHERE id=$1`, in.SpaceID).Scan(&visibility); err != nil {
			return nil, err
		}
		if visibility != "public" {
			return nil, ErrJoinAttemptDenied
		}
	}
	sha, err := c.SHA256()
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,start_selection_context,start_selection_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, in.ActorProfileID, in.OperationID, in.ActorAccountID, in.SpaceID, in.Method, in.RequestSHA256, raw, sha[:])
	if err != nil {
		return nil, err
	}
	saved, err = scoped.loadJoinStartSelection(ctx, in)
	if err != nil {
		return nil, err
	}
	return saved, tx.Commit(ctx)
}
