package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/entrypolicy"
)

type JoinLegacySelection struct {
	Operation JoinOperation
	Context   JoinLegacySelectionContext
}

func (s *SpaceStore) ReserveLegacyJoinSelection(ctx context.Context, in JoinOperationInput, origin uint8, invite *uuid.UUID) (*JoinLegacySelection, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if !validLegacyJoinOperation(in) || (origin != JoinOperationClientKey && origin != JoinOperationServerNoKey) || ((in.Method == spacev1.SpaceService_JoinByInvite_FullMethodName) != (invite != nil)) || (invite != nil && *invite == uuid.Nil) {
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
	local := *s
	local.tx = tx
	saved, err := local.loadLegacyJoinSelection(ctx, in, origin)
	if err == nil {
		if !sameOptionalJoinAttempt(saved.Context.InviteID, invite) {
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
	policy, err := local.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return nil, err
	}
	c := JoinLegacySelectionContext{Operation: in, Origin: origin, PolicyVersion: policy.Version, PolicySHA256: joinDigest("policy", policy.Policy)}
	if invite != nil {
		inv, err := local.GetInviteByID(ctx, *invite)
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
		if err = local.validateInviteForJoin(inv, now); err != nil {
			return nil, err
		}
		id := *invite
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
	_, err = tx.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256,basis_version,operation_origin,legacy_selection_context,legacy_selection_sha256) VALUES($1,$2,$3,$4,$5,$6,2,$7,$8,$9)`, in.ActorProfileID, in.OperationID, in.ActorAccountID, in.SpaceID, in.Method, in.RequestSHA256, origin, raw, sha[:])
	if err != nil {
		return nil, err
	}
	saved, err = local.loadLegacyJoinSelection(ctx, in, origin)
	if err != nil {
		return nil, err
	}
	return saved, tx.Commit(ctx)
}

func (s *SpaceStore) loadLegacyJoinSelection(ctx context.Context, in JoinOperationInput, origin uint8) (*JoinLegacySelection, error) {
	var saved JoinLegacySelection
	var raw, digest []byte
	var version, savedOrigin int16
	o := &saved.Operation
	err := s.db().QueryRow(ctx, `SELECT `+joinOperationColumns+`,basis_version,operation_origin,legacy_selection_context,legacy_selection_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID).Scan(&o.Input.SpaceID, &o.Input.ActorAccountID, &o.Input.ActorProfileID, &o.Input.OperationID, &o.Input.Method, &o.Input.RequestSHA256, &o.AttemptID, &o.State, &version, &savedOrigin, &raw, &digest)
	if err != nil {
		return nil, err
	}
	if version != 2 || savedOrigin != int16(origin) || !bytes.Equal(joinDigest("public-operation", o.Input), joinDigest("public-operation", in)) {
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
	if err != nil || !bytes.Equal(sha[:], digest) || saved.Context.Origin != origin || !bytes.Equal(joinDigest("public-operation", saved.Context.Operation), joinDigest("public-operation", in)) {
		return nil, ErrJoinAttemptUnavailable
	}
	if o.State != "RESERVED" && o.State != "DECIDED" && o.State != "COMPLETED" {
		return nil, ErrJoinAttemptUnavailable
	}
	return &saved, nil
}
