package chatcreation

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// Store errors are classified by the concrete adapter; authentication and
// external admission failures never manufacture a stored terminal outcome.
type OutcomeStore interface {
	GetChatCreationOutcome(context.Context, uuid.UUID, uuid.UUID) (*spacev1.SpaceChatCreationOutcome, error)
	CommitChatCreation(context.Context, *spacev1.SpaceChatCreationBinding, codes.Code) (*spacev1.SpaceChatCreationOutcome, error)
}

type Admission interface {
	CheckChatCreation(context.Context, *spacev1.SpaceChatCreationBinding) error
	CheckOutcomeDisclosure(context.Context, principal.Principal, *spacev1.SpaceChatCreationBinding) error
}

type Server struct {
	spacev1.UnimplementedSpaceServiceServer
	Store        OutcomeStore
	Admission    Admission
	UnknownError error
	MapError     func(error) error
}

func (s *Server) mapError(err error) error {
	if s.MapError != nil {
		return s.MapError(err)
	}
	return status.Error(codes.Unavailable, "composite persistence unavailable")
}

func verifiedChat(ctx context.Context) bool {
	p, ok := principal.FromContext(ctx)
	return ok && p.Kind == "service" && p.Issuer == "chat" && p.Subject == "service:chat" && p.Audience == "space"
}

func (s *Server) CommitSpaceChatCreation(ctx context.Context, req *spacev1.CommitSpaceChatCreationRequest) (*spacev1.CommitSpaceChatCreationResponse, error) {
	if !verifiedChat(ctx) {
		return nil, status.Error(codes.Unauthenticated, "verified Chat service required")
	}
	if req == nil || HasUnknown(req) || ValidateBinding(req.GetBinding()) != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid composite binding")
	}
	if s == nil || s.Store == nil || s.UnknownError == nil {
		return nil, status.Error(codes.Unavailable, "composite persistence unavailable")
	}
	binding := proto.Clone(req.Binding).(*spacev1.SpaceChatCreationBinding)
	existing, err := s.Store.GetChatCreationOutcome(ctx, uuid.MustParse(binding.SpaceId), uuid.MustParse(binding.OperationId))
	if err == nil {
		if ValidateOutcome(binding, existing) != nil {
			return nil, status.Error(codes.AlreadyExists, "composite binding conflict")
		}
		return &spacev1.CommitSpaceChatCreationResponse{Outcome: existing}, nil
	}
	if !errors.Is(err, s.UnknownError) {
		return nil, s.mapError(err)
	}
	if s.Admission == nil {
		return nil, status.Error(codes.Unavailable, "composite admission unavailable")
	}
	admission := codes.OK
	if err = s.Admission.CheckChatCreation(ctx, binding); err != nil {
		admission = status.Code(err)
		if !TerminalFailure(admission) {
			return nil, err
		}
	}
	out, err := s.Store.CommitChatCreation(ctx, binding, admission)
	if err != nil {
		return nil, s.mapError(err)
	}
	if ValidateOutcome(binding, out) != nil {
		return nil, status.Error(codes.Unavailable, "invalid composite persisted outcome")
	}
	return &spacev1.CommitSpaceChatCreationResponse{Outcome: out}, nil
}

func (s *Server) GetSpaceChatCreationOutcome(ctx context.Context, req *spacev1.GetSpaceChatCreationOutcomeRequest) (*spacev1.GetSpaceChatCreationOutcomeResponse, error) {
	p, ok := principal.FromContext(ctx)
	gateway := ok && p.Kind == "delegated_user" && p.Issuer == "gateway" && p.Audience == "space"
	if !verifiedChat(ctx) && !gateway {
		return nil, status.Error(codes.Unauthenticated, "verified composite principal required")
	}
	if req == nil || HasUnknown(req) || !CanonicalID(req.SpaceId) || !CanonicalID(req.OperationId) {
		return nil, status.Error(codes.InvalidArgument, "invalid composite lookup")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "composite persistence unavailable")
	}
	out, err := s.Store.GetChatCreationOutcome(ctx, uuid.MustParse(req.SpaceId), uuid.MustParse(req.OperationId))
	if err != nil {
		return nil, s.mapError(err)
	}
	if out == nil || ValidateOutcome(out.GetBinding(), out) != nil || out.GetBinding().GetSpaceId() != req.SpaceId || out.GetBinding().GetOperationId() != req.OperationId {
		return nil, status.Error(codes.Unavailable, "invalid composite persisted outcome")
	}
	if gateway {
		if s.Admission == nil {
			return nil, status.Error(codes.Unavailable, "composite disclosure unavailable")
		}
		if err = s.Admission.CheckOutcomeDisclosure(ctx, p, out.Binding); err != nil {
			return nil, err
		}
	}
	return &spacev1.GetSpaceChatCreationOutcomeResponse{Outcome: out}, nil
}
