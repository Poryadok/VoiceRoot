package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/privacy"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/memberadmissionrole"
	"voice/backend/space/internal/store"
)

// These are the actual protected owner/Role clients, not success callbacks.
type LegacyAdmissionDependencies struct {
	Subjects  *admissionauthority.Client
	Reviewers *admissionauthority.ReviewerClient
	Roles     *memberadmissionrole.Client
}

func (s *SpaceGRPC) joinLegacyAdmission(ctx context.Context, request proto.Message) (*spacev1.SpaceMembership, error) {
	var call *lifecycleprincipal.VerifiedAdmissionCall
	var err error
	direct := false
	switch req := request.(type) {
	case *spacev1.JoinSpaceRequest:
		direct = true
		call, err = lifecycleprincipal.RequireAdmissionJoinSpace(ctx, req)
	case *spacev1.JoinByInviteRequest:
		call, err = lifecycleprincipal.RequireAdmissionJoinInvite(ctx, req)
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported admission request")
	}
	if err != nil {
		return nil, err
	}
	request = proto.Clone(request)
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "native admission persistence unavailable")
	}
	p, err := s.Store.PrepareLegacyJoinStart(ctx, request)
	if err != nil {
		return nil, mapLegacyAdmissionError(err)
	}
	if member, done, err := s.Store.ReplayLegacyJoinOutcome(ctx, p); done || err != nil {
		if err == nil && member == nil {
			return nil, mapLegacyAdmissionError(store.ErrJoinAttemptUnavailable)
		}
		return member, mapLegacyAdmissionError(err)
	}
	op, err := p.Reservation()
	if err != nil {
		return nil, mapLegacyAdmissionError(err)
	}
	if s.LegacyAdmission == nil || s.LegacyAdmission.Subjects == nil {
		return nil, status.Error(codes.Unavailable, "admission owner observation unavailable")
	}
	// Social reads are deliberately outside every native transaction/fence.
	// Their local PRE restriction binds the exact target and finite observation.
	started := time.Now()
	var target uuid.UUID
	if direct {
		row, e := s.Store.GetSpace(ctx, op.Input.SpaceID)
		if e != nil {
			return nil, mapLegacyAdmissionError(e)
		}
		if row == nil {
			return nil, status.Error(codes.NotFound, "space not found")
		}
		target = row.OwnerProfileID
	} else {
		inv, e := s.Store.GetInviteByCode(ctx, request.(*spacev1.JoinByInviteRequest).Code)
		if e != nil {
			return nil, mapLegacyAdmissionError(e)
		}
		if inv == nil || inv.SpaceID != op.Input.SpaceID {
			return nil, status.Error(codes.Unavailable, "selected invite unavailable")
		}
		target = inv.CreatorProfileID
		if e = s.ensureLegacyInvitePrivacy(ctx, uuid.MustParse(actor.ProfileID), target); e != nil {
			return nil, e
		}
	}
	if err = s.ensureJoinNotBlocked(ctx, uuid.MustParse(actor.AccountID), target); err != nil {
		return nil, err
	}
	guard := func(ctx context.Context, local *store.SpaceStore, native *store.MemberAdmissionIntent) error {
		return local.CheckScopedLegacyJoinTarget(ctx, native, target, started)
	}
	var basis []byte
	var savedP *uuid.UUID
	switch op.State {
	case "DECIDED":
		recovery, e := s.Store.RecoverLegacyJoinDecision(ctx, p)
		if e != nil {
			return nil, mapLegacyAdmissionError(e)
		}
		basis, savedP = recovery.DecisionBasisSHA256, recovery.PublicationOperationID
	case "RESERVED":
		if op.AttemptID != nil {
			selected, e := s.Store.ReadLegacyJoinSelectedAttempt(ctx, p)
			if e != nil {
				return nil, mapLegacyAdmissionError(e)
			}
			if selected.State == "PENDING" {
				resume, e := s.Store.PrepareLegacyJoinPendingResume(ctx, p)
				if e != nil {
					return nil, mapLegacyAdmissionError(e)
				}
				ownerRequest, e := resume.OwnerRequest()
				if e != nil {
					return nil, mapLegacyAdmissionError(e)
				}
				subject, e := s.LegacyAdmission.Subjects.Observe(ctx, ownerRequest)
				if e != nil {
					return nil, mapLegacyAdmissionError(e)
				}
				verified, e := resume.Seal(subject)
				if e != nil {
					return nil, mapLegacyAdmissionError(e)
				}
				record, e := s.Store.CompleteLegacyJoinPendingResume(ctx, verified)
				if e != nil {
					return nil, mapLegacyAdmissionError(e)
				}
				if record == nil || record.RequestID == nil || record.Attempt.State != "PENDING" {
					return nil, mapLegacyAdmissionError(store.ErrJoinAttemptUnavailable)
				}
				return nil, mapLegacyAdmissionError(store.ErrJoinAttemptPending)
			}
			if selected.State != "COLLECTING" {
				return nil, mapLegacyAdmissionError(store.ErrJoinAttemptUnavailable)
			}
		}
		if op.AttemptID == nil {
			ownerRequest, e := p.OwnerRequest()
			if e != nil {
				return nil, mapLegacyAdmissionError(e)
			}
			subject, e := s.LegacyAdmission.Subjects.Observe(ctx, ownerRequest)
			if e != nil {
				return nil, mapLegacyAdmissionError(e)
			}
			identity, e := subject.Check(ownerRequest)
			if e != nil {
				return nil, mapLegacyAdmissionError(e)
			}
			if direct && (actor.AccountType == "guest" || identity.AccountType == "guest") {
				return nil, status.Error(codes.PermissionDenied, "guests require an invite")
			}
			verified, e := p.Seal(subject)
			if e != nil {
				return nil, mapLegacyAdmissionError(e)
			}
			attempt, e := s.Store.CommitLegacyJoinStart(ctx, verified)
			if e != nil {
				return nil, mapLegacyAdmissionError(e)
			}
			if attempt.State == "PENDING" {
				record, e := s.Store.CompleteLegacyJoinPending(ctx, verified)
				if e != nil {
					return nil, mapLegacyAdmissionError(e)
				}
				if record.RequestID == nil || record.Attempt.State != "PENDING" {
					return nil, mapLegacyAdmissionError(store.ErrJoinAttemptUnavailable)
				}
				return nil, mapLegacyAdmissionError(store.ErrJoinAttemptPending)
			}
		}
		decision, e := s.Store.PrepareLegacyJoinDecision(ctx, p)
		if e != nil {
			return nil, mapLegacyAdmissionError(e)
		}
		ownerRequest, e := decision.OwnerRequest()
		if e != nil {
			return nil, mapLegacyAdmissionError(e)
		}
		subject, e := s.LegacyAdmission.Subjects.Observe(ctx, ownerRequest)
		if e != nil {
			return nil, mapLegacyAdmissionError(e)
		}
		identity, e := subject.Check(ownerRequest)
		if e != nil {
			return nil, mapLegacyAdmissionError(e)
		}
		if direct && (actor.AccountType == "guest" || identity.AccountType == "guest") {
			return nil, status.Error(codes.PermissionDenied, "guests require an invite")
		}
		verified, e := decision.Seal(subject)
		if e != nil {
			return nil, mapLegacyAdmissionError(e)
		}
		record, e := s.Store.CommitLegacyJoinDecision(ctx, verified)
		if e != nil {
			return nil, mapLegacyAdmissionError(e)
		}
		if record.Attempt.State == "PENDING" && record.RequestID != nil {
			return nil, mapLegacyAdmissionError(store.ErrJoinAttemptPending)
		}
		if record.Attempt.State != "READY" {
			return nil, mapLegacyAdmissionError(store.ErrJoinAttemptUnavailable)
		}
		basis = record.DecisionBasisSHA256
	default:
		return nil, mapLegacyAdmissionError(store.ErrJoinAttemptUnavailable)
	}
	if s.LegacyAdmission.Roles == nil {
		return nil, mapLegacyAdmissionError(store.ErrJoinAttemptUnavailable)
	}
	native, err := s.Store.PrepareJoinDecisionAdmission(ctx, basis)
	if err != nil {
		return nil, mapLegacyAdmissionError(err)
	}
	if savedP != nil && *savedP != native.Input.OperationID {
		return nil, mapLegacyAdmissionError(store.ErrJoinAttemptConflict)
	}
	post := func(ctx context.Context, local *store.SpaceStore, intent *store.MemberAdmissionIntent, _ *store.MembershipRow) error {
		return local.MarkLegacyJoinPublishedOutcome(ctx, p, intent)
	}
	_, err = s.Store.CompleteInteractiveMemberAdmissionWithPublicationCall(ctx, native.Input.OperationID, s.LegacyAdmission.Roles, s.LegacyAdmission.Subjects, s.LegacyAdmission.Reviewers, guard, post, call, request)
	if err != nil {
		return nil, mapLegacyAdmissionError(err)
	}
	member, done, err := s.Store.ReplayLegacyJoinOutcome(ctx, p)
	if err != nil {
		return nil, mapLegacyAdmissionError(err)
	}
	if !done || member == nil {
		return nil, mapLegacyAdmissionError(store.ErrJoinAttemptUnavailable)
	}
	return member, nil
}

func (s *SpaceGRPC) ensureLegacyInvitePrivacy(ctx context.Context, recipient, inviter uuid.UUID) error {
	if s == nil || s.Privacy == nil {
		return status.Error(codes.Unavailable, "invite privacy observation unavailable")
	}
	audience, err := s.Privacy.AllowChatSpaceInvitesAudience(ctx, recipient)
	if err != nil {
		return mapLegacyAdmissionError(err)
	}
	// c143-c145: the viewer is the inviter, not the applicant. None of the
	// applicant capabilities or raw metadata establish the inviter's type.
	// These existing predicates are independent of that unavailable fact.
	if recipient != inviter && !audience.IsEveryoneShortcut() && !audience.IsNobody() {
		return status.Error(codes.Unavailable, "invite actor classification unavailable")
	}
	if err = privacy.CheckAllowed(privacy.Matcher{}, ctx, recipient, inviter, audience, false); err != nil {
		if errors.Is(err, privacy.ErrDenied) {
			return status.Error(codes.PermissionDenied, "invite blocked by recipient privacy settings")
		}
		return mapLegacyAdmissionError(err)
	}
	return nil
}

func mapLegacyAdmissionError(err error) error {
	if err == nil || status.Code(err) != codes.Unknown {
		return err
	}
	switch {
	case errors.Is(err, store.ErrJoinAttemptUnavailable), errors.Is(err, store.ErrEffectiveEpisodeUnavailable), errors.Is(err, store.ErrMemberAuthorityUnavailable), errors.Is(err, admissionauthority.ErrUnavailable), errors.Is(err, admissionauthority.ErrBinding):
		return status.Error(codes.Unavailable, "native admission unavailable")
	case errors.Is(err, store.ErrLegacyJoinInputRequired), errors.Is(err, store.ErrJoinAttemptPending), errors.Is(err, store.ErrMemberAdmissionStale):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, store.ErrJoinAttemptConflict), errors.Is(err, store.ErrMemberAdmissionConflict):
		return status.Error(codes.AlreadyExists, "admission operation conflicts")
	case errors.Is(err, store.ErrJoinAttemptDenied), errors.Is(err, admissionauthority.ErrIneligible):
		return status.Error(codes.PermissionDenied, "admission denied")
	case errors.Is(err, entrypolicy.ErrInvalid):
		return status.Error(codes.InvalidArgument, "invalid admission input")
	default:
		return mapInviteStoreErr(err)
	}
}
