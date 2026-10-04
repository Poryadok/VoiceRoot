package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
)

// MatchSquadMemberController is the Voice-owned, delegated-user adapter for
// the three MatchSquad membership operations. It is deliberately separate
// from VoiceGRPC so forwarded profile metadata cannot authorize these calls.
type MatchSquadMemberController interface {
	Join(context.Context, *callsv1.JoinMatchSquadRoomRequest) (*callsv1.JoinMatchSquadRoomResponse, error)
	GetJoinToken(context.Context, *callsv1.GetMatchSquadJoinTokenRequest) (*callsv1.GetMatchSquadJoinTokenResponse, error)
	Leave(context.Context, *callsv1.LeaveMatchSquadRoomRequest) (*callsv1.LeaveMatchSquadRoomResponse, error)
}

type MatchSquadMemberGRPC struct {
	callsv1.UnimplementedMatchSquadMemberServiceServer
	Controller MatchSquadMemberController
}

func (s *MatchSquadMemberGRPC) JoinMatchSquadRoom(ctx context.Context, req *callsv1.JoinMatchSquadRoomRequest) (*callsv1.JoinMatchSquadRoomResponse, error) {
	if s == nil || s.Controller == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad member service unavailable")
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad join request")
	}
	return s.Controller.Join(ctx, req)
}

func (s *MatchSquadMemberGRPC) GetMatchSquadJoinToken(ctx context.Context, req *callsv1.GetMatchSquadJoinTokenRequest) (*callsv1.GetMatchSquadJoinTokenResponse, error) {
	if s == nil || s.Controller == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad member service unavailable")
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad token request")
	}
	return s.Controller.GetJoinToken(ctx, req)
}

func (s *MatchSquadMemberGRPC) LeaveMatchSquadRoom(ctx context.Context, req *callsv1.LeaveMatchSquadRoomRequest) (*callsv1.LeaveMatchSquadRoomResponse, error) {
	if s == nil || s.Controller == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad member service unavailable")
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad leave request")
	}
	return s.Controller.Leave(ctx, req)
}
