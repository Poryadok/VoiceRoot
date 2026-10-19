package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/memberadmissionrole"
	"voice/backend/space/internal/store"
)

type AdmissionDependencies struct {
	Subjects  *admissionauthority.Client
	Reviewers *admissionauthority.ReviewerClient
	Roles     *memberadmissionrole.Client
}

// The passed base retains all Composite/Audit behavior. Only the additive
// admission methods implemented here are overridden; legacy joins stay with
// their native dispatcher. Missing owner transports never use ordinary RPCs.
type admissionServer struct {
	spacev1.SpaceServiceServer
	store *store.SpaceStore
	deps  AdmissionDependencies
}

func NewAdmissionServer(base spacev1.SpaceServiceServer, st *store.SpaceStore, deps AdmissionDependencies) spacev1.SpaceServiceServer {
	if base == nil {
		base = &spacev1.UnimplementedSpaceServiceServer{}
	}
	return &admissionServer{SpaceServiceServer: base, store: st, deps: deps}
}

func admissionUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, status.Error(codes.InvalidArgument, "canonical UUID required")
	}
	return id, nil
}

func admissionReadSubject(request proto.Message, actor principal.Principal, space uuid.UUID, role authv1.SpaceAdmissionSubjectRole) (*authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid admission request")
	}
	account, err := admissionUUID(actor.AccountID)
	if err != nil {
		return nil, err
	}
	profile, err := admissionUUID(actor.ProfileID)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	frame := []byte("voice.space.join.read-context.v1\x00")
	frame = binary.BigEndian.AppendUint32(frame, uint32(len(actor.RPC)))
	frame = append(frame, actor.RPC...)
	frame = append(frame, account[:]...)
	frame = append(frame, profile[:]...)
	frame = append(frame, space[:]...)
	frame = append(frame, digest[:]...)
	binding := sha256.Sum256(frame)
	epoch := uint64(actor.SessionEpoch)
	return &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ, SubjectRole: role, AccountId: account.String(), ProfileId: profile.String(), SpaceId: space.String(), AdmissionBindingSha256: binding[:], InteractiveSessionEpoch: &epoch}, nil
}

func (s *admissionServer) GetJoinAttempt(ctx context.Context, original *spacev1.GetJoinAttemptRequest) (*spacev1.GetJoinAttemptResponse, error) {
	if original == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	request := proto.Clone(original).(*spacev1.GetJoinAttemptRequest)
	call, err := lifecycleprincipal.RequireAdmissionAttemptRead(ctx, request)
	if err != nil {
		return nil, err
	}
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	space, err := admissionUUID(request.SpaceId)
	if err != nil {
		return nil, err
	}
	attempt, err := admissionUUID(request.AttemptId)
	if err != nil {
		return nil, err
	}
	if s == nil || s.store == nil || s.deps.Subjects == nil {
		return nil, status.Error(codes.Unavailable, "admission owner transport unavailable")
	}
	expected, err := admissionReadSubject(request, actor, space, authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT)
	if err != nil {
		return nil, err
	}
	subject, err := s.deps.Subjects.Observe(ctx, expected)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	guard := func() error {
		if _, err := call.ActorFor(request); err != nil {
			return err
		}
		_, err := subject.Check(expected)
		return err
	}
	view, err := s.store.ReadOwnJoinAttempt(ctx, space, attempt, uuid.MustParse(actor.AccountID), uuid.MustParse(actor.ProfileID), guard)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	var member *store.MembershipRow
	if view.State == "PUBLISHED" {
		if view.PublicationID == nil {
			return nil, status.Error(codes.Unavailable, "native admission outcome unavailable")
		}
		intent, err := s.store.LookupMemberAdmission(ctx, *view.PublicationID)
		if err != nil {
			return nil, mapAdmissionError(err)
		}
		if intent == nil || intent.State != "PUBLISHED" || intent.Input.OperationID != *view.PublicationID || intent.Input.SpaceID != space || intent.Input.AccountID.String() != actor.AccountID || intent.Input.ProfileID.String() != actor.ProfileID || intent.Input.JoinAttempt == nil || intent.Input.JoinAttempt.AttemptID != view.ID {
			return nil, status.Error(codes.Unavailable, "native admission binding unavailable")
		}
		var saved bool
		member, saved, err = s.store.ReadMemberAdmissionOutcome(ctx, *view.PublicationID)
		if err != nil {
			return nil, mapAdmissionError(err)
		}
		if !saved || member == nil || member.SpaceID != space || member.ProfileID.String() != actor.ProfileID {
			return nil, status.Error(codes.Unavailable, "native admission outcome unavailable")
		}
	}
	projected, err := projectJoinAttemptRead(view, member)
	if err != nil {
		return nil, err
	}
	if err = guard(); err != nil {
		return nil, mapAdmissionError(err)
	}
	return &spacev1.GetJoinAttemptResponse{Attempt: projected}, nil
}

func projectJoinAttemptRead(v *store.JoinAttemptRead, member *store.MembershipRow) (*spacev1.JoinAttemptView, error) {
	if v == nil {
		return nil, status.Error(codes.Unavailable, "admission snapshot unavailable")
	}
	state := spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_UNSPECIFIED
	switch v.State {
	case "COLLECTING":
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_COLLECTING
	case "PENDING":
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING
	case "READY", "CLAIMED":
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PROCESSING
	case "FAILED":
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_FAILED
	case "EXPIRED":
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_EXPIRED
	case "INVALIDATED":
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_INVALIDATED
	case "PUBLISHED":
		if member == nil || member.SpaceID != v.SpaceID {
			return nil, status.Error(codes.Unavailable, "native admission outcome required")
		}
		state = spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED
	default:
		return nil, status.Error(codes.Unavailable, "native admission outcome required")
	}
	r := &spacev1.JoinAdmissionResult{State: state}
	if member != nil {
		r.SpaceMembership = membershipRowToProto(member)
	}
	if v.RequestID != nil {
		r.RequestId = v.RequestID.String()
	} else if v.State == "PENDING" {
		return nil, status.Error(codes.Unavailable, "request association unavailable")
	}
	requirements := &spacev1.JoinRequirements{RequirePhone: v.Requirements.Phone, RequireCaptcha: v.Requirements.Captcha, RequireManualApproval: v.Requirements.Manual}
	for _, q := range v.Requirements.Questions {
		requirements.Questions = append(requirements.Questions, &spacev1.JoinQuestion{QuestionId: q.ID, Prompt: q.Prompt})
	}
	view := &spacev1.JoinAttemptView{AttemptId: v.ID.String(), SpaceId: v.SpaceID.String(), PolicyVersion: v.PolicyVersion, ExpiresAt: timestamppb.New(v.CollectionDeadline), Requirements: requirements, Result: r}
	for _, a := range v.Answers {
		view.SubmittedAnswers = append(view.SubmittedAnswers, &spacev1.JoinAnswer{QuestionId: a.QuestionID.String(), Value: a.Value})
	}
	return view, nil
}

func mapAdmissionError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) != codes.Unknown {
		return err
	}
	switch {
	case errors.Is(err, store.ErrJoinReadNotFound):
		return status.Error(codes.NotFound, "join request not found")
	case errors.Is(err, admissionauthority.ErrIneligible), errors.Is(err, store.ErrJoinAttemptDenied), errors.Is(err, store.ErrGuestsNotAllowed), errors.Is(err, store.ErrAccountBanned):
		return status.Error(codes.PermissionDenied, "admission denied")
	case errors.Is(err, store.ErrJoinAttemptConflict):
		return status.Error(codes.AlreadyExists, "admission operation conflict")
	case errors.Is(err, admissionauthority.ErrBinding), errors.Is(err, admissionauthority.ErrUnavailable), errors.Is(err, store.ErrJoinAttemptUnavailable):
		return status.Error(codes.Unavailable, "admission authority unavailable")
	default:
		return status.Error(codes.Internal, "admission operation failed")
	}
}
