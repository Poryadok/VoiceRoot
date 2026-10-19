package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"
	"voice/backend/space/internal/entrypolicy"
)

var (
	ErrJoinAttemptDenied      = errors.New("join attempt no longer eligible")
	ErrJoinAttemptUnavailable = errors.New("join attempt authority unavailable")
	ErrJoinAttemptConflict    = errors.New("join attempt immutable binding conflict")
	ErrJoinAttemptPending     = errors.New("join attempt pending review")
)

// JoinAttemptInput is internal decision data, never an authenticated principal.
// Adapters must derive it from an exact verified applicant context.
type JoinAttemptInput struct {
	SpaceID, AccountID, ProfileID uuid.UUID
	InviteID                      *uuid.UUID
	Guest                         bool
}
type JoinAttemptReference struct {
	AttemptID      uuid.UUID
	EvidenceSHA256 []byte
}

// MarkJoinAttemptPublished is local bookkeeping, never membership authority.
// Only the first native publication transaction calls it; public status must
// use the native durable outcome, and completed native replay never calls it.
func (s *SpaceStore) MarkJoinAttemptPublished(ctx context.Context, ref JoinAttemptReference, publication uuid.UUID) error {
	if s == nil || s.tx == nil || ref.AttemptID == uuid.Nil || publication == uuid.Nil || len(ref.EvidenceSHA256) != 32 {
		return ErrJoinAttemptUnavailable
	}
	var spaceID uuid.UUID
	if err := s.db().QueryRow(ctx, `SELECT space_id FROM space_join_attempts WHERE id=$1`, ref.AttemptID).Scan(&spaceID); err != nil {
		return ErrJoinAttemptUnavailable
	}
	if err := lockAndCheckOwnershipScope(ctx, s.tx, []uuid.UUID{spaceID}); err != nil {
		return err
	}
	result, err := s.db().Exec(ctx, `UPDATE space_join_attempts SET state='PUBLISHED' WHERE id=$1 AND state='CLAIMED' AND publication_operation_id=$2 AND evidence_sha256=$3`, ref.AttemptID, publication, ref.EvidenceSHA256)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrJoinAttemptUnavailable
	}
	return nil
}

type JoinAttemptApprovalInput struct {
	SpaceID, AttemptID, ActorAccountID, ActorProfileID, OperationID uuid.UUID
}

// InvalidateJoinAttemptsForAccount participates only in the full account-ban
// transaction. It never treats a caller assertion as evidence of a ban.
func (s *SpaceStore) InvalidateJoinAttemptsForAccount(ctx context.Context, spaceID, accountID uuid.UUID) error {
	if s == nil || s.tx == nil || spaceID == uuid.Nil || accountID == uuid.Nil {
		return ErrJoinAttemptUnavailable
	}
	if err := lockAndCheckOwnershipScope(ctx, s.tx, []uuid.UUID{spaceID}); err != nil {
		return err
	}
	var banned bool
	if err := s.db().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_bans WHERE space_id=$1 AND account_id=$2)`, spaceID, accountID).Scan(&banned); err != nil {
		return err
	}
	if !banned {
		return ErrJoinAttemptUnavailable
	}
	// The native consumer must first prove exact P/A retirement and its durable
	// generation floor. A native state label without that binding is insufficient.
	// Until that bridge is configured, preserve the whole transaction on CLAIMED.
	var claimed bool
	if err := s.db().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_join_attempts WHERE space_id=$1 AND account_id=$2 AND state='CLAIMED')`, spaceID, accountID).Scan(&claimed); err != nil {
		return err
	}
	if claimed {
		return ErrJoinAttemptUnavailable
	}
	_, err := s.db().Exec(ctx, `UPDATE space_join_attempts SET state='INVALIDATED' WHERE space_id=$1 AND account_id=$2 AND state IN ('COLLECTING','PENDING','READY')`, spaceID, accountID)
	return err
}

type JoinAttemptApprovalOperation struct {
	Input   JoinAttemptApprovalInput
	State   string
	Outcome *JoinAttempt
}

// ReserveJoinAttemptApproval must precede remote observations. This is a
// technical selection journal, not reviewer authorization or approval.
func (s *SpaceStore) ReserveJoinAttemptApproval(ctx context.Context, in JoinAttemptApprovalInput) (*JoinAttemptApprovalOperation, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if in.SpaceID == uuid.Nil || in.AttemptID == uuid.Nil || in.ActorAccountID == uuid.Nil || in.ActorProfileID == uuid.Nil || in.OperationID == uuid.Nil {
		return nil, entrypolicy.ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	key := joinDigest("approval-operation", struct{ Actor, Operation uuid.UUID }{in.ActorProfileID, in.OperationID})
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(binary.BigEndian.Uint32(key[:4])), int32(binary.BigEndian.Uint32(key[4:8]))); err != nil {
		return nil, err
	}
	request := joinDigest("approval-request", in)
	var prior, outcome []byte
	var state string
	err = tx.QueryRow(ctx, `SELECT request_sha256,state,outcome FROM space_join_approval_operations WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID).Scan(&prior, &state, &outcome)
	if err == nil {
		if !bytes.Equal(prior, request) {
			return nil, ErrJoinAttemptConflict
		}
		operation := &JoinAttemptApprovalOperation{Input: in, State: state}
		if state == "COMPLETED" {
			var saved JoinAttempt
			if json.Unmarshal(outcome, &saved) != nil || saved.ID != in.AttemptID || saved.Input.SpaceID != in.SpaceID || saved.State != "READY" || len(saved.EvidenceSHA256) != 32 {
				return nil, ErrJoinAttemptUnavailable
			}
			operation.Outcome = &saved
		} else if state != "RESERVED" || outcome != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		return operation, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	a, err := scanJoinAttempt(tx.QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, in.AttemptID))
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if a.Input.SpaceID != in.SpaceID || (a.State != "PENDING" && a.State != "READY") {
		return nil, ErrJoinAttemptDenied
	}
	p, err := entrypolicy.Parse(a.policyRaw)
	if err != nil || !p.RequireManualApproval {
		return nil, ErrJoinAttemptDenied
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_join_approval_operations(actor_profile_id,operation_id,attempt_id,request_sha256,state) VALUES($1,$2,$3,$4,'RESERVED')`, in.ActorProfileID, in.OperationID, in.AttemptID, request)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &JoinAttemptApprovalOperation{Input: in, State: "RESERVED"}, nil
}

// JoinAttemptApprovalGuard uses only local pre-fetched verified owner decisions.
// It must recheck their strict monotonic freshness budget; no RPC under locks.
type JoinAttemptApprovalGuard func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error

// joinAttemptDecisionHook persists only local decision data in the same TX.
// It is separate from the final pure authority guard; no RPC belongs here.
// Only the private verified decision adapter may supply a production hook.
type joinAttemptDecisionHook func(context.Context, *SpaceStore, *joinAttemptPrivate) error

// ApproveJoinAttempt records a review decision, not successful admission. The
// coordinator still owes native Ensure/first publication and fresh owner guards.
func (s *SpaceStore) ApproveJoinAttempt(ctx context.Context, in JoinAttemptApprovalInput, guard JoinAttemptApprovalGuard) (*JoinAttempt, error) {
	return s.approveJoinAttempt(ctx, in, guard, nil)
}

func (s *SpaceStore) approveJoinAttempt(ctx context.Context, in JoinAttemptApprovalInput, guard JoinAttemptApprovalGuard, hook joinAttemptDecisionHook) (*JoinAttempt, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if in.SpaceID == uuid.Nil || in.AttemptID == uuid.Nil || in.ActorAccountID == uuid.Nil || in.ActorProfileID == uuid.Nil || in.OperationID == uuid.Nil {
		return nil, entrypolicy.ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	// Lock actor/operation before the Space key. Same operation can never select
	// another attempt/Space while a remote authorization is in flight.
	key := joinDigest("approval-operation", struct{ Actor, Operation uuid.UUID }{in.ActorProfileID, in.OperationID})
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(binary.BigEndian.Uint32(key[:4])), int32(binary.BigEndian.Uint32(key[4:8]))); err != nil {
		return nil, err
	}
	request := joinDigest("approval-request", in)
	var prior, outcome []byte
	var operationState string
	err = tx.QueryRow(ctx, `SELECT request_sha256,state,outcome FROM space_join_approval_operations WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID).Scan(&prior, &operationState, &outcome)
	if err == nil {
		if !bytes.Equal(prior, request) {
			return nil, ErrJoinAttemptConflict
		}
		if operationState == "COMPLETED" {
			var saved JoinAttempt
			if json.Unmarshal(outcome, &saved) != nil || saved.ID != in.AttemptID || saved.Input.SpaceID != in.SpaceID || saved.State != "READY" || len(saved.EvidenceSHA256) != 32 {
				return nil, ErrJoinAttemptUnavailable
			}
			return &saved, tx.Commit(ctx)
		}
		if operationState != "RESERVED" || outcome != nil {
			return nil, ErrJoinAttemptUnavailable
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJoinAttemptUnavailable
	}
	if err != nil {
		return nil, err
	}
	if guard == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	scoped := *s
	scoped.tx = tx
	a, err := scanJoinAttempt(tx.QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, in.AttemptID))
	if err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if a.Input.SpaceID != in.SpaceID {
		return nil, ErrJoinAttemptDenied
	}
	if a.State != "PENDING" && a.State != "READY" {
		return nil, ErrJoinAttemptDenied
	}
	if err = scoped.localJoinAttemptGates(ctx, a.Input); err != nil {
		return nil, err
	}
	p, err := scoped.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return nil, err
	}
	if p.Version != a.PolicyVersion || !bytes.Equal(joinDigest("policy", p.Policy), a.policyDigest) {
		return nil, ErrJoinAttemptDenied
	}
	if !p.Policy.RequireManualApproval || p.Policy.RequirePhone || p.Policy.RequireCaptcha {
		return nil, ErrJoinAttemptUnavailable
	}
	if err = a.validateAutomaticEvidence(p.Policy); err != nil {
		return nil, err
	}
	if member, err := scoped.IsSpaceMember(ctx, in.SpaceID, in.ActorProfileID); err != nil {
		return nil, err
	} else if !member {
		return nil, ErrJoinAttemptDenied
	}
	// Every potentially waiting local query precedes this pure local recheck of
	// exact verified subject/Role bindings and strict earliest-call <5s budget.
	if err = guard(ctx, &scoped, in); err != nil {
		return nil, err
	}
	newDecision := a.State == "PENDING"
	if newDecision {
		a.approvedAccount = &in.ActorAccountID
		a.approvedProfile = &in.ActorProfileID
		a.approvalOperation = &in.OperationID
		a.State = "READY"
		a.EvidenceSHA256 = a.evidence()
		_, err = tx.Exec(ctx, `UPDATE space_join_attempts SET state='READY',approved_account_id=$2,approved_profile_id=$3,approval_operation_id=$4,evidence_sha256=$5 WHERE id=$1`, a.ID, in.ActorAccountID, in.ActorProfileID, in.OperationID, a.EvidenceSHA256)
		if err != nil {
			return nil, err
		}
	} else if a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil || !bytes.Equal(a.evidence(), a.EvidenceSHA256) {
		return nil, ErrJoinAttemptUnavailable
	}
	saved, _ := json.Marshal(a.JoinAttempt)
	tag, err := tx.Exec(ctx, `UPDATE space_join_approval_operations SET state='COMPLETED',evidence_sha256=$3,outcome=$4 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='RESERVED' AND request_sha256=$5`, in.ActorProfileID, in.OperationID, a.EvidenceSHA256, saved, request)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, ErrJoinAttemptConflict
	}
	if newDecision && hook != nil {
		if err = hook(ctx, &scoped, a); err != nil {
			return nil, err
		}
	}
	// Recheck after journal writes too: database waits must not extend authority.
	if err = guard(ctx, &scoped, in); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &a.JoinAttempt, nil
}

type JoinAttempt struct {
	ID                 uuid.UUID
	Input              JoinAttemptInput
	PolicyVersion      int64
	State              string
	CollectionDeadline time.Time
	EvidenceSHA256     []byte
}
type joinAttemptPrivate struct {
	JoinAttempt
	policyRaw, policyDigest, binding, submission, answers            []byte
	approvedAccount, approvedProfile, approvalOperation, publication *uuid.UUID
}

const joinAttemptColumns = `id,space_id,account_id,profile_id,invite_id,guest,policy_version,state,collection_deadline,evidence_sha256,policy_snapshot,policy_sha256,binding_sha256,submission_sha256,submitted_answers,approved_account_id,approved_profile_id,approval_operation_id,publication_operation_id`

func scanJoinAttempt(row pgx.Row) (*joinAttemptPrivate, error) {
	a := &joinAttemptPrivate{}
	err := row.Scan(&a.ID, &a.Input.SpaceID, &a.Input.AccountID, &a.Input.ProfileID, &a.Input.InviteID, &a.Input.Guest, &a.PolicyVersion, &a.State, &a.CollectionDeadline, &a.EvidenceSHA256, &a.policyRaw, &a.policyDigest, &a.binding, &a.submission, &a.answers, &a.approvedAccount, &a.approvedProfile, &a.approvalOperation, &a.publication)
	a.CollectionDeadline = a.CollectionDeadline.UTC()
	return a, err
}

func (a *joinAttemptPrivate) validateAutomaticEvidence(p entrypolicy.Policy) error {
	if !bytes.Equal(joinDigest("policy", p), a.policyDigest) || !bytes.Equal(joinBinding(a.Input, a.PolicyVersion, a.policyDigest), a.binding) {
		return ErrJoinAttemptUnavailable
	}
	storedPolicy, err := entrypolicy.Parse(a.policyRaw)
	if err != nil || !bytes.Equal(joinDigest("policy", storedPolicy), a.policyDigest) {
		return ErrJoinAttemptUnavailable
	}
	var answers []entrypolicy.Answer
	if json.Unmarshal(a.answers, &answers) != nil {
		return ErrJoinAttemptUnavailable
	}
	raw, err := canonicalJoinAnswers(p, answers)
	if err != nil || !bytes.Equal(joinDigest("submission", json.RawMessage(raw)), a.submission) {
		return ErrJoinAttemptUnavailable
	}
	return nil
}
func joinDigest(domain string, value any) []byte {
	raw, _ := json.Marshal(value)
	digest := sha256.Sum256(append([]byte("voice.space.join."+domain+".v1\x00"), raw...))
	return digest[:]
}
func validJoinInput(in JoinAttemptInput) bool {
	return in.SpaceID != uuid.Nil && in.AccountID != uuid.Nil && in.ProfileID != uuid.Nil && (in.InviteID == nil || *in.InviteID != uuid.Nil)
}
func joinBinding(in JoinAttemptInput, version int64, policy []byte) []byte {
	return joinDigest("binding", struct {
		Input   JoinAttemptInput
		Version int64
		Policy  []byte
	}{in, version, policy})
}
func (a *joinAttemptPrivate) evidence() []byte {
	return joinDigest("evidence", struct {
		ID                                                  uuid.UUID
		Binding, Policy, Submission                         []byte
		ApproverAccount, ApproverProfile, ApprovalOperation *uuid.UUID
	}{a.ID, a.binding, a.policyDigest, a.submission, a.approvedAccount, a.approvedProfile, a.approvalOperation})
}

// This helper is local-only and requires the common Space transaction fence.
func (s *SpaceStore) localJoinAttemptGates(ctx context.Context, in JoinAttemptInput) error {
	if banned, err := s.IsAccountBanned(ctx, in.SpaceID, in.AccountID); err != nil {
		return err
	} else if banned {
		return ErrAccountBanned
	}
	if member, err := s.IsSpaceMember(ctx, in.SpaceID, in.ProfileID); err != nil {
		return err
	} else if member {
		return ErrAlreadySpaceMember
	}
	var visibility string
	var guests bool
	var count int32
	if err := s.db().QueryRow(ctx, `SELECT visibility,allow_guests,member_count FROM spaces WHERE id=$1`, in.SpaceID).Scan(&visibility, &guests, &count); err != nil {
		return err
	}
	cap, err := memberCapTx(ctx, s.tx, in.SpaceID)
	if err != nil {
		return err
	}
	if count >= cap {
		return ErrMemberCapReached
	}
	if in.Guest && (in.InviteID == nil || !guests) {
		return ErrGuestsNotAllowed
	}
	if in.InviteID == nil {
		if visibility != "public" {
			return ErrJoinAttemptDenied
		}
		return nil
	}
	inv, err := s.GetInviteByID(ctx, *in.InviteID)
	if err != nil {
		return err
	}
	if inv == nil || inv.SpaceID != in.SpaceID {
		return ErrInviteNotFound
	}
	var now time.Time
	if err = s.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return err
	}
	return s.validateInviteForJoin(inv, now)
}

func (s *SpaceStore) StartJoinAttempt(ctx context.Context, in JoinAttemptInput) (*JoinAttempt, error) {
	if !validJoinInput(in) {
		return nil, entrypolicy.ErrInvalid
	}
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{in.SpaceID}, func(scoped *SpaceStore) (*JoinAttempt, error) {
		if err := scoped.localJoinAttemptGates(ctx, in); err != nil {
			return nil, err
		}
		p, err := scoped.GetEntryPolicy(ctx, in.SpaceID)
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(p.Policy)
		policyHash := joinDigest("policy", p.Policy)
		binding := joinBinding(in, p.Version, policyHash)
		var now time.Time
		if err = scoped.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return nil, err
		}
		a, err := scanJoinAttempt(scoped.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE binding_sha256=$1 AND state IN ('COLLECTING','PENDING','READY')`, binding))
		if err == nil {
			if a.State != "COLLECTING" || now.Before(a.CollectionDeadline) {
				return &a.JoinAttempt, nil
			}
			if _, err = scoped.db().Exec(ctx, `UPDATE space_join_attempts SET state='EXPIRED' WHERE id=$1`, a.ID); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		a, err = scanJoinAttempt(scoped.db().QueryRow(ctx, `INSERT INTO space_join_attempts(id,space_id,account_id,profile_id,invite_id,guest,binding_sha256,policy_version,policy_sha256,policy_snapshot,state,created_at,collection_deadline) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'COLLECTING',$11::timestamptz,$11::timestamptz+interval '600 seconds') RETURNING `+joinAttemptColumns, uuid.New(), in.SpaceID, in.AccountID, in.ProfileID, in.InviteID, in.Guest, binding, p.Version, policyHash, raw, now))
		if err != nil {
			return nil, err
		}
		return &a.JoinAttempt, nil
	})
}

func canonicalJoinAnswers(p entrypolicy.Policy, answers []entrypolicy.Answer) ([]byte, error) {
	err := p.CheckAnswers(answers)
	if err != nil && !errors.Is(err, entrypolicy.ErrDenied) {
		return nil, err
	}
	copyAnswers := append([]entrypolicy.Answer{}, answers...)
	for i := range copyAnswers {
		copyAnswers[i].Value = strings.Trim(norm.NFC.String(copyAnswers[i].Value), "\t\n\r ")
	}
	slices.SortFunc(copyAnswers, func(a, b entrypolicy.Answer) int { return bytes.Compare(a.QuestionID[:], b.QuestionID[:]) })
	raw, _ := json.Marshal(copyAnswers)
	return raw, err
}

func (s *SpaceStore) SubmitJoinAttempt(ctx context.Context, in JoinAttemptInput, id uuid.UUID, answers []entrypolicy.Answer) (*JoinAttempt, error) {
	return s.submitJoinAttempt(ctx, in, id, answers, nil, nil)
}

func (s *SpaceStore) submitJoinAttempt(ctx context.Context, in JoinAttemptInput, id uuid.UUID, answers []entrypolicy.Answer, hook joinAttemptDecisionHook, guard func() error) (*JoinAttempt, error) {
	if !validJoinInput(in) || id == uuid.Nil {
		return nil, entrypolicy.ErrInvalid
	}
	if hook != nil && guard == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	result, err := withOwnershipScopeValue(s, ctx, []uuid.UUID{in.SpaceID}, func(scoped *SpaceStore) (*JoinAttempt, error) {
		a, err := scanJoinAttempt(scoped.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		if !bytes.Equal(joinDigest("input", in), joinDigest("input", a.Input)) {
			return nil, ErrJoinAttemptDenied
		}
		p, err := entrypolicy.Parse(a.policyRaw)
		if err != nil {
			return nil, ErrJoinAttemptUnavailable
		}
		raw, answerErr := canonicalJoinAnswers(p, answers)
		if answerErr != nil && !errors.Is(answerErr, entrypolicy.ErrDenied) {
			return nil, answerErr
		}
		submission := joinDigest("submission", json.RawMessage(raw))
		if a.submission != nil {
			if !bytes.Equal(submission, a.submission) {
				return nil, ErrJoinAttemptConflict
			}
			return &a.JoinAttempt, nil
		}
		if a.State != "COLLECTING" {
			return nil, ErrJoinAttemptDenied
		}
		var now time.Time
		if err = scoped.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return nil, err
		}
		if !now.Before(a.CollectionDeadline) {
			_, err = scoped.db().Exec(ctx, `UPDATE space_join_attempts SET state='EXPIRED' WHERE id=$1`, id)
			a.State = "EXPIRED"
			return &a.JoinAttempt, err
		}
		if err = scoped.localJoinAttemptGates(ctx, in); err != nil {
			return nil, err
		}
		current, err := scoped.GetEntryPolicy(ctx, in.SpaceID)
		if err != nil {
			return nil, err
		}
		if current.Version != a.PolicyVersion || !bytes.Equal(joinDigest("policy", current.Policy), a.policyDigest) {
			return nil, ErrJoinAttemptDenied
		}
		if p.RequirePhone {
			if in.Guest {
				return nil, ErrJoinAttemptDenied
			}
			return nil, ErrJoinAttemptUnavailable
		}
		if p.RequireCaptcha {
			return nil, ErrJoinAttemptUnavailable
		}
		if guard != nil {
			if err = guard(); err != nil {
				return nil, err
			}
		}
		a.submission = submission
		switch {
		case answerErr != nil:
			a.State = "FAILED"
		case p.RequireManualApproval:
			a.State = "PENDING"
		default:
			a.State = "READY"
			a.EvidenceSHA256 = a.evidence()
		}
		_, err = scoped.db().Exec(ctx, `UPDATE space_join_attempts SET state=$2,submission_sha256=$3,submitted_answers=$4,evidence_sha256=$5 WHERE id=$1`, id, a.State, submission, raw, a.EvidenceSHA256)
		if err != nil {
			return nil, err
		}
		if hook != nil && (a.State == "PENDING" || a.State == "READY") {
			if err = hook(ctx, scoped, a); err != nil {
				return nil, err
			}
		}
		// The writer may have waited after the first clock sample (including in
		// database triggers). A newly collected proof must still be within its
		// window after those waits; this does not expire historical pending proof.
		var withinCollection bool
		if err = scoped.db().QueryRow(ctx, `SELECT clock_timestamp()<collection_deadline FROM space_join_attempts WHERE id=$1`, id).Scan(&withinCollection); err != nil {
			return nil, err
		}
		if !withinCollection {
			if hook != nil {
				// The outer transaction rolls back both newly collected proof and
				// its decision basis; no partial historical decision survives.
				return nil, ErrJoinAttemptDenied
			}
			_, err = scoped.db().Exec(ctx, `UPDATE space_join_attempts SET state='EXPIRED',evidence_sha256=NULL WHERE id=$1`, id)
			a.State = "EXPIRED"
			a.EvidenceSHA256 = nil
		}
		if guard != nil {
			if err = guard(); err != nil {
				return nil, err
			}
		}
		return &a.JoinAttempt, err
	})
	if err == nil && result != nil {
		if result.State == "FAILED" {
			return result, entrypolicy.ErrDenied
		}
		if result.State == "EXPIRED" || result.State == "INVALIDATED" {
			return result, ErrJoinAttemptDenied
		}
	}
	return result, err
}

// VerifyJoinAttemptAdmission is for the native core's fenced transaction only.
// Historical evidence never replaces the mandatory fresh final-owner guard.
func (s *SpaceStore) VerifyJoinAttemptAdmission(ctx context.Context, in JoinAttemptInput, ref JoinAttemptReference, version int64, publication uuid.UUID) error {
	if s == nil || s.tx == nil || !validJoinInput(in) || ref.AttemptID == uuid.Nil || len(ref.EvidenceSHA256) != 32 || publication == uuid.Nil {
		return ErrJoinAttemptUnavailable
	}
	a, err := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1 FOR UPDATE`, ref.AttemptID))
	if err != nil {
		return ErrJoinAttemptUnavailable
	}
	if !bytes.Equal(joinDigest("input", in), joinDigest("input", a.Input)) {
		return ErrJoinAttemptDenied
	}
	if a.State != "READY" && a.State != "CLAIMED" {
		return ErrJoinAttemptDenied
	}
	if !bytes.Equal(ref.EvidenceSHA256, a.EvidenceSHA256) || !bytes.Equal(a.evidence(), a.EvidenceSHA256) {
		return ErrJoinAttemptUnavailable
	}
	p, err := s.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return err
	}
	if version != a.PolicyVersion || p.Version != version || !bytes.Equal(joinDigest("policy", p.Policy), a.policyDigest) {
		return ErrJoinAttemptDenied
	}
	if p.Policy.RequirePhone || p.Policy.RequireCaptcha {
		return ErrJoinAttemptUnavailable
	}
	if err = a.validateAutomaticEvidence(p.Policy); err != nil {
		return err
	}
	if p.Policy.RequireManualApproval && (a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil) {
		return ErrJoinAttemptUnavailable
	}
	if a.publication != nil && *a.publication != publication {
		return ErrJoinAttemptConflict
	}
	_, err = s.db().Exec(ctx, `UPDATE space_join_attempts SET state='CLAIMED',publication_operation_id=$2 WHERE id=$1`, a.ID, publication)
	return err
}
