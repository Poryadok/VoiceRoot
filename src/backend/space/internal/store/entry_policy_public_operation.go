package store

import (
	"bytes"
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
)

type EntryPolicySetProposal struct {
	request   *spacev1.SetEntryPolicyRequest
	call      *lifecycleprincipal.VerifiedAdmissionCall
	operation JoinOperation
}

type VerifiedEntryPolicySet struct {
	proposal *EntryPolicySetProposal
	subject  *admissionauthority.VerifiedSubject
	settings *admissionauthority.VerifiedSettingsPermission
}

func (p *EntryPolicySetProposal) Seal(subject *admissionauthority.VerifiedSubject, settings *admissionauthority.VerifiedSettingsPermission) (*VerifiedEntryPolicySet, error) {
	d := &VerifiedEntryPolicySet{proposal: p, subject: subject, settings: settings}
	if err := d.check(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *VerifiedEntryPolicySet) check() error {
	if d == nil || d.proposal == nil || d.subject == nil || d.settings == nil {
		return ErrEntryPolicyUnavailable
	}
	expected, err := d.proposal.OwnerRequest()
	if err != nil {
		return err
	}
	subject, err := d.subject.Transcript(expected)
	if err != nil {
		return err
	}
	settings, err := d.settings.Transcript(expected)
	if err != nil {
		return err
	}
	if subject.ObservationSHA256 != settings.Subject.ObservationSHA256 {
		return ErrEntryPolicyUnavailable
	}
	return nil
}

func (s *SpaceStore) PrepareEntryPolicySet(ctx context.Context, request *spacev1.SetEntryPolicyRequest) (*EntryPolicySetProposal, error) {
	call, err := lifecycleprincipal.RequireEntryPolicySet(ctx, request)
	if err != nil {
		return nil, err
	}
	request = proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	account, err := canonicalJoinID(actor.AccountID)
	if err != nil {
		return nil, ErrEntryPolicyUnavailable
	}
	profile, err := canonicalJoinID(actor.ProfileID)
	if err != nil {
		return nil, ErrEntryPolicyUnavailable
	}
	in, _, err := entryPolicyPublicInput(request, account, profile)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrEntryPolicyUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockJoinPublicOperation(ctx, tx, in); err != nil {
		return nil, err
	}
	local := *s
	local.tx = tx
	saved, _, found, err := local.lookupEntryPolicyPublicOperation(ctx, in, true)
	if err != nil {
		return nil, err
	}
	if !found {
		// A legacy30 receipt alone lacks authenticated account/method/H
		// provenance. Never manufacture that provenance from today's caller.
		var legacy bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_entry_policy_operations WHERE actor_profile_id=$1 AND operation_id=$2)`, in.ActorProfileID, in.OperationID).Scan(&legacy); err != nil || legacy {
			return nil, ErrEntryPolicyUnavailable
		}
		_, err = tx.Exec(ctx, `INSERT INTO space_join_operations(actor_profile_id,operation_id,actor_account_id,space_id,request_method,request_sha256) VALUES($1,$2,$3,$4,$5,$6)`, in.ActorProfileID, in.OperationID, in.ActorAccountID, in.SpaceID, in.Method, in.RequestSHA256)
		if err != nil {
			return nil, err
		}
		saved, _, found, err = local.lookupEntryPolicyPublicOperation(ctx, in, true)
		if err != nil || !found {
			return nil, ErrEntryPolicyUnavailable
		}
	}
	if _, err = call.ActorFor(request); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &EntryPolicySetProposal{request: request, call: call, operation: *saved}, nil
}

func (s *SpaceStore) lookupEntryPolicyPublicOperation(ctx context.Context, in JoinOperationInput, lock bool) (*JoinOperation, *spacev1.SetEntryPolicyResponse, bool, error) {
	query := `SELECT ` + joinOperationColumns + `,basis_version,operation_origin,decision_basis,decision_basis_sha256,observed_basis_sha256,publication_operation_id,publication_state,start_selection_context,start_selection_sha256,legacy_selection_context,legacy_selection_sha256,outcome_code,outcome_proto FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`
	if lock {
		if s.tx == nil {
			return nil, nil, false, ErrEntryPolicyUnavailable
		}
		query += ` FOR UPDATE`
	}
	var saved JoinOperation
	var version, origin int16
	var basis, basisSHA, observed, start, startSHA, legacy, legacySHA, raw []byte
	var publication *uuid.UUID
	var publicationState *string
	var code *int32
	err := s.db().QueryRow(ctx, query, in.ActorProfileID, in.OperationID).Scan(&saved.Input.SpaceID, &saved.Input.ActorAccountID, &saved.Input.ActorProfileID, &saved.Input.OperationID, &saved.Input.Method, &saved.Input.RequestSHA256, &saved.AttemptID, &saved.State, &version, &origin, &basis, &basisSHA, &observed, &publication, &publicationState, &start, &startSHA, &legacy, &legacySHA, &code, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	if in.Method != spacev1.SpaceService_SetEntryPolicy_FullMethodName || !bytes.Equal(joinDigest("public-operation", saved.Input), joinDigest("public-operation", in)) {
		return nil, nil, true, ErrEntryPolicyConflict
	}
	if version != 1 || origin != 1 || saved.AttemptID != nil || basis != nil || basisSHA != nil || observed != nil || publication != nil || publicationState != nil || start != nil || startSHA != nil || legacy != nil || legacySHA != nil {
		return nil, nil, true, ErrEntryPolicyUnavailable
	}
	if saved.State == "RESERVED" && code == nil && raw == nil {
		return &saved, nil, true, nil
	}
	if saved.State != "COMPLETED" || code == nil || *code != 0 || raw == nil {
		return nil, nil, true, ErrEntryPolicyUnavailable
	}
	var ack spacev1.SetEntryPolicyResponse
	if proto.Unmarshal(raw, &ack) != nil || len(ack.ProtoReflect().GetUnknown()) != 0 || ack.OperationId != in.OperationID.String() || ack.EntryPolicyVersion == 0 || ack.EntryPolicyVersion > math.MaxInt64 {
		return nil, nil, true, ErrEntryPolicyUnavailable
	}
	return &saved, &ack, true, nil
}

func (p *EntryPolicySetProposal) ingress() error {
	if p == nil || p.call == nil || p.request == nil {
		return ErrEntryPolicyUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	if err != nil {
		return err
	}
	if actor.AccountID != p.operation.Input.ActorAccountID.String() || actor.ProfileID != p.operation.Input.ActorProfileID.String() || actor.RPC != spacev1.SpaceService_SetEntryPolicy_FullMethodName {
		return ErrEntryPolicyUnavailable
	}
	in, _, err := entryPolicyPublicInput(p.request, p.operation.Input.ActorAccountID, p.operation.Input.ActorProfileID)
	if err != nil || !bytes.Equal(joinDigest("public-operation", in), joinDigest("public-operation", p.operation.Input)) {
		return ErrEntryPolicyConflict
	}
	return nil
}

func (p *EntryPolicySetProposal) OwnerRequest() (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if err := p.ingress(); err != nil {
		return nil, err
	}
	if p.operation.State != "RESERVED" {
		return nil, ErrEntryPolicyUnavailable
	}
	actor, err := p.call.ActorFor(p.request)
	if err != nil || actor.SessionEpoch <= 0 {
		return nil, ErrEntryPolicyUnavailable
	}
	epoch := uint64(actor.SessionEpoch)
	in := p.operation.Input
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR, AccountId: in.ActorAccountID.String(), ProfileId: in.ActorProfileID.String(), SpaceId: in.SpaceID.String(), AdmissionBindingSha256: append([]byte(nil), in.RequestSHA256...), InteractiveSessionEpoch: &epoch}, nil
}

// ReplayEntryPolicySet returns only the saved safe acknowledgement. It never
// reads current policy, membership or settings authorization and never repairs.
func (s *SpaceStore) ReplayEntryPolicySet(ctx context.Context, p *EntryPolicySetProposal) (*spacev1.SetEntryPolicyResponse, bool, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, false, ErrEntryPolicyUnavailable
	}
	if err := p.ingress(); err != nil {
		return nil, false, err
	}
	_, ack, found, err := s.lookupEntryPolicyPublicOperation(ctx, p.operation.Input, false)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, ErrEntryPolicyUnavailable
	}
	if err = p.ingress(); err != nil {
		return nil, false, err
	}
	return ack, ack != nil, nil
}
