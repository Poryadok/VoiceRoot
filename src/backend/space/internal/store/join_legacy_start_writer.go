package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

type JoinLegacyStartProposal struct {
	request   proto.Message
	call      *lifecycleprincipal.VerifiedAdmissionCall
	selection *JoinLegacySelection
}
type VerifiedLegacyJoinStart struct {
	proposal *JoinLegacyStartProposal
	subject  *admissionauthority.VerifiedSubject
}

// PrepareLegacyJoinStart retains the real legacy request/method. A missing
// public key stays absent in that signed request; only the private journal gets
// a new UUID. This reservation happens before its first owner observation.
func (s *SpaceStore) PrepareLegacyJoinStart(ctx context.Context, request proto.Message) (*JoinLegacyStartProposal, error) {
	var call *lifecycleprincipal.VerifiedAdmissionCall
	var err error
	var space uuid.UUID
	var invite *uuid.UUID
	var publicKey *string
	switch r := request.(type) {
	case *spacev1.JoinSpaceRequest:
		if r == nil {
			return nil, entrypolicy.ErrInvalid
		}
		call, err = lifecycleprincipal.RequireAdmissionJoinSpace(ctx, r)
	case *spacev1.JoinByInviteRequest:
		if r == nil {
			return nil, entrypolicy.ErrInvalid
		}
		call, err = lifecycleprincipal.RequireAdmissionJoinInvite(ctx, r)
	default:
		return nil, entrypolicy.ErrInvalid
	}
	if err != nil {
		return nil, err
	}
	request = proto.Clone(request)
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	switch r := request.(type) {
	case *spacev1.JoinSpaceRequest:
		space, err = canonicalJoinID(r.SpaceId)
		publicKey = r.OperationId
	case *spacev1.JoinByInviteRequest:
		var inv *InviteRow
		inv, err = s.GetInviteByCode(ctx, r.Code)
		if err == nil && inv == nil {
			err = ErrInviteNotFound
		}
		if err == nil {
			space = inv.SpaceID
			id := inv.ID
			invite = &id
		}
		publicKey = r.OperationId
	}
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
	operation, origin := uuid.New(), JoinOperationServerNoKey
	if publicKey != nil {
		operation, err = canonicalJoinID(*publicKey)
		if err != nil {
			return nil, err
		}
		origin = JoinOperationClientKey
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	sha := sha256.Sum256(raw)
	in := JoinOperationInput{SpaceID: space, ActorAccountID: account, ActorProfileID: profile, OperationID: operation, Method: actor.RPC, RequestSHA256: sha[:]}
	selection, err := s.ReserveLegacyJoinSelection(ctx, in, origin, invite)
	if err != nil {
		return nil, err
	}
	if _, err = call.ActorFor(request); err != nil {
		return nil, err
	}
	return &JoinLegacyStartProposal{request: request, call: call, selection: selection}, nil
}

func (p *JoinLegacyStartProposal) OwnerRequest() (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if p == nil || p.call == nil || p.selection == nil || p.selection.Operation.State != "RESERVED" || p.selection.Operation.AttemptID != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	if err != nil {
		return nil, err
	}
	c := p.selection.Context
	sha, err := c.SHA256()
	if err != nil {
		return nil, err
	}
	if actor.AccountID != c.Operation.ActorAccountID.String() || actor.ProfileID != c.Operation.ActorProfileID.String() || actor.RPC != c.Operation.Method {
		return nil, ErrJoinAttemptUnavailable
	}
	epoch := uint64(actor.SessionEpoch)
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: actor.AccountID, ProfileId: actor.ProfileID, SpaceId: c.Operation.SpaceID.String(), AdmissionBindingSha256: sha[:], InteractiveSessionEpoch: &epoch}, nil
}
func (p *JoinLegacyStartProposal) Seal(subject *admissionauthority.VerifiedSubject) (*VerifiedLegacyJoinStart, error) {
	expected, err := p.OwnerRequest()
	if err != nil {
		return nil, err
	}
	if _, err = subject.Check(expected); err != nil {
		return nil, err
	}
	return &VerifiedLegacyJoinStart{proposal: p, subject: subject}, nil
}
func (d *VerifiedLegacyJoinStart) check() (admissionauthority.SubjectIdentity, error) {
	if d == nil || d.proposal == nil || d.subject == nil {
		return admissionauthority.SubjectIdentity{}, ErrJoinAttemptUnavailable
	}
	expected, err := d.proposal.OwnerRequest()
	if err != nil {
		return admissionauthority.SubjectIdentity{}, err
	}
	return d.subject.Check(expected)
}

// CommitLegacyJoinStart creates only a private collecting A/pin. A subsequent
// real Context2-bound SUBMIT owner observation and decision transaction are
// mandatory; this result cannot be returned as admitted or pending to a client.
func (s *SpaceStore) CommitLegacyJoinStart(ctx context.Context, d *VerifiedLegacyJoinStart) (*JoinAttempt, error) {
	if ctx == nil || s == nil || s.Pool == nil || s.tx != nil || d == nil || d.proposal == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	p := d.proposal
	if _, err := d.check(); err != nil {
		return nil, err
	}
	c := p.selection.Context
	in := c.Operation
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
	saved, err := local.loadLegacyJoinSelection(ctx, in, c.Origin)
	if err != nil {
		return nil, err
	}
	actual, err := saved.Context.SHA256()
	if err != nil {
		return nil, err
	}
	expected, err := c.SHA256()
	if err != nil || actual != expected {
		return nil, ErrJoinAttemptConflict
	}
	if saved.Operation.State != "RESERVED" || saved.Operation.AttemptID != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	identity, err := d.check()
	if err != nil {
		return nil, err
	}
	policy, err := local.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return nil, err
	}
	if policy.Version != c.PolicyVersion || !bytes.Equal(joinDigest("policy", policy.Policy), c.PolicySHA256) {
		return nil, ErrJoinAttemptDenied
	}
	applicant := JoinAttemptInput{SpaceID: in.SpaceID, AccountID: in.ActorAccountID, ProfileID: in.ActorProfileID, InviteID: c.InviteID, Guest: identity.AccountType == "guest"}
	a, err := local.StartJoinAttempt(ctx, applicant)
	if err != nil {
		return nil, err
	}
	if a.PolicyVersion != c.PolicyVersion {
		return nil, ErrJoinAttemptUnavailable
	}
	tag, err := tx.Exec(ctx, `UPDATE space_join_operations SET selected_attempt_id=$3 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='RESERVED' AND selected_attempt_id IS NULL AND basis_version=2 AND operation_origin=$4`, in.ActorProfileID, in.OperationID, a.ID, c.Origin)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, ErrJoinAttemptConflict
	}
	if a.State == "COLLECTING" {
		var now time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return nil, err
		}
		if !now.Before(a.CollectionDeadline) {
			return nil, ErrJoinAttemptDenied
		}
	}
	if _, err = d.check(); err != nil {
		return nil, err
	}
	return a, tx.Commit(ctx)
}
