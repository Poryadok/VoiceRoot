package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

// JoinSubmissionRecovery is historical coordinator input, never membership or
// current authority. Only the saved exact P may be resumed; no proof transfer.
type JoinSubmissionRecovery struct {
	AttemptID              uuid.UUID
	DecisionKind           uint8
	DecisionBasisSHA256    []byte
	PublicationOperationID *uuid.UUID
}

// RecoverJoinSubmissionDecision authenticates the exact original actor/body.
// It returns immutable historical coordinator data without refreshing owner
// observations, policy, invite, membership or counters. Admission still requires
// the actual saved native outcome or its own fresh continuation protocol.
func (s *SpaceStore) RecoverJoinSubmissionDecision(ctx context.Context, request *spacev1.SubmitJoinAttemptRequest) (*JoinSubmissionRecovery, error) {
	call, err := lifecycleprincipal.RequireAdmissionSubmit(ctx, request)
	if err != nil {
		return nil, err
	}
	request = proto.Clone(request).(*spacev1.SubmitJoinAttemptRequest)
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	space, err := canonicalJoinID(request.SpaceId)
	if err != nil {
		return nil, err
	}
	operation, err := canonicalJoinID(request.OperationId)
	if err != nil {
		return nil, err
	}
	attempt, err := canonicalJoinID(request.AttemptId)
	if err != nil {
		return nil, err
	}
	account, err := canonicalJoinID(actor.AccountID)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	profile, err := canonicalJoinID(actor.ProfileID)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	sha := sha256.Sum256(raw)
	op := JoinOperationInput{SpaceID: space, ActorAccountID: account, ActorProfileID: profile, OperationID: operation, Method: JoinOperationSubmit, RequestSHA256: sha[:]}
	recovered, err := s.loadJoinSubmissionRecovery(ctx, op)
	if err != nil {
		return nil, err
	}
	if recovered.AttemptID != attempt {
		return nil, ErrJoinAttemptConflict
	}
	if _, err = call.ActorFor(request); err != nil {
		return nil, err
	}
	return recovered, nil
}

func (s *SpaceStore) loadJoinSubmissionRecovery(ctx context.Context, op JoinOperationInput) (*JoinSubmissionRecovery, error) {
	if s == nil || s.Pool == nil || s.tx != nil || !validJoinOperation(op) || op.Method != JoinOperationSubmit {
		return nil, ErrJoinAttemptUnavailable
	}
	// Historical replay has no current Space/admission gate and acquires no
	// writer fence. One read-only snapshot prevents a partially purged chain.
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	local := *s
	local.tx = tx
	var saved JoinOperation
	var sha []byte
	err = tx.QueryRow(ctx, `SELECT `+joinOperationColumns+`,decision_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&saved.Input.SpaceID, &saved.Input.ActorAccountID, &saved.Input.ActorProfileID, &saved.Input.OperationID, &saved.Input.Method, &saved.Input.RequestSHA256, &saved.AttemptID, &saved.State, &sha)
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if !bytes.Equal(joinDigest("public-operation", saved.Input), joinDigest("public-operation", op)) {
		return nil, ErrJoinAttemptConflict
	}
	if saved.State == "RESERVED" || saved.AttemptID == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	history, err := local.loadJoinDecision(ctx, sha, false)
	if err != nil {
		return nil, err
	}
	c := history.basis.Context
	if c.Kind != JoinDecisionSubmitPending && c.Kind != JoinDecisionSubmitReady {
		return nil, ErrJoinAttemptUnavailable
	}
	a, err := scanJoinAttempt(tx.QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, c.AttemptID))
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if a.Input.SpaceID != op.SpaceID || a.Input.AccountID != op.ActorAccountID || a.Input.ProfileID != op.ActorProfileID || !bytes.Equal(a.binding, c.AttemptBindingSHA256) || !bytes.Equal(a.submission, c.SubmissionSHA256) {
		return nil, ErrJoinAttemptUnavailable
	}
	if c.Kind == JoinDecisionSubmitPending && history.publication != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if c.Kind == JoinDecisionSubmitReady {
		if !bytes.Equal(a.EvidenceSHA256, c.ReadyEvidenceSHA256) || (history.publication == nil && a.publication != nil) {
			return nil, ErrJoinAttemptUnavailable
		}
		if history.publication != nil && (a.publication == nil || *a.publication != *history.publication || history.publicationState == nil || (*history.publicationState != "PREPARING" && *history.publicationState != "PUBLISHED") || ((*history.publicationState == "PUBLISHED") != (a.State == "PUBLISHED"))) {
			return nil, ErrJoinAttemptUnavailable
		}
	}
	result := &JoinSubmissionRecovery{AttemptID: c.AttemptID, DecisionKind: c.Kind, DecisionBasisSHA256: slices.Clone(sha)}
	if history.publication != nil {
		p := *history.publication
		result.PublicationOperationID = &p
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}
