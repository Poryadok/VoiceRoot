package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/reportprincipal"
	"voice/backend/space/internal/store"
)

type ReportReadStore interface {
	ReadProtectedSpaceReportReadContext(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*store.SpaceReportReadContext, error)
}
type ReportReadGRPC struct {
	spacev1.UnimplementedSpaceServiceServer
	Store ReportReadStore
}

func (s *ReportReadGRPC) ResolveSpaceReportReadContext(ctx context.Context, req *spacev1.ResolveSpaceReportReadContextRequest) (*spacev1.ResolveSpaceReportReadContextResponse, error) {
	if !reportprincipal.Producer(ctx, req) {
		return nil, status.Error(codes.Unauthenticated, "verified Messaging report reader required")
	}
	unavailable := status.Error(codes.Unavailable, "Space report context unavailable")
	if s == nil || s.Store == nil {
		return nil, unavailable
	}
	space, profile, chat := uuid.MustParse(req.SpaceId), uuid.MustParse(req.ViewerProfileId), uuid.MustParse(req.Chat.Id)
	facts, err := s.Store.ReadProtectedSpaceReportReadContext(ctx, space, profile, chat)
	response := &spacev1.ResolveSpaceReportReadContextResponse{ProtocolVersion: 1, OperationId: req.OperationId, SpaceId: req.SpaceId, ViewerProfileId: req.ViewerProfileId, Chat: proto.Clone(req.Chat).(*chatv1.ChatRef)}
	if errors.Is(err, store.ErrSpaceReportReadDenied) {
		return response, nil
	}
	if err != nil || facts == nil || !facts.Eligible || facts.SpaceID != space || facts.ProfileID != profile || facts.ChatID != chat || facts.TreeNodeID == uuid.Nil || facts.EpisodeID == uuid.Nil || facts.SpaceRevision == 0 || facts.EpisodeGeneration <= 0 || facts.LedgerRevision <= 0 || facts.SourceBinding == nil || facts.ValidityRemaining < time.Millisecond || facts.ValidityRemaining > 5*time.Second || facts.SampledAt.IsZero() {
		return nil, unavailable
	}
	sampled := timestamppb.New(facts.SampledAt)
	if sampled.CheckValid() != nil {
		return nil, unavailable
	}
	if facts.HistoryKnown {
		since := timestamppb.New(facts.HistoryVisibleSince)
		if facts.HistoryVisibleSince.IsZero() || since.CheckValid() != nil || facts.HistoryVisibleSince.After(facts.SampledAt) {
			return nil, unavailable
		}
		response.HistoryVisibleSince = since
	} else if !facts.HistoryVisibleSince.IsZero() {
		return nil, unavailable
	}
	response.Eligible = true
	response.SpaceAuthorityRevision = facts.SpaceRevision
	response.LifecycleGeneration = facts.LifecycleGeneration
	response.TreeAttachmentIncarnation = facts.TreeNodeID.String()
	response.MembershipBinding = proto.Clone(facts.SourceBinding).(*rolev1.MembershipBinding)
	response.HistoryKnown = facts.HistoryKnown
	response.EffectiveEpisodeId = facts.EpisodeID.String()
	response.EffectiveEpisodeGeneration = uint64(facts.EpisodeGeneration)
	response.LedgerRevision = uint64(facts.LedgerRevision)
	response.EvaluatedAt = sampled
	response.ValidityRemainingMs = uint32(facts.ValidityRemaining / time.Millisecond)
	return response, nil
}

// RegisterReportReadServer exposes exactly this method on the private transport.
func RegisterReportReadServer(server grpc.ServiceRegistrar, service *ReportReadGRPC) {
	desc := spacev1.SpaceService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, m := range spacev1.SpaceService_ServiceDesc.Methods {
		if m.MethodName == "ResolveSpaceReportReadContext" {
			desc.Methods = append(desc.Methods, m)
		}
	}
	server.RegisterService(&desc, service)
}
