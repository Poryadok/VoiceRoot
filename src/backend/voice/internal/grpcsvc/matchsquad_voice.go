package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	callsv1 "voice.app/voice/calls/v1"
)

type MatchSquadVoiceController interface {
	Create(context.Context, *callsv1.CreateMatchSquadRoomRequest) ([]byte, error)
	Teardown(context.Context, *callsv1.TeardownMatchSquadRoomRequest) ([]byte, error)
	Compact(context.Context, *callsv1.CompactMatchSquadRoomRequest) ([]byte, error)
}

type MatchSquadVoiceGRPC struct {
	callsv1.UnimplementedMatchSquadVoiceServiceServer
	Controller MatchSquadVoiceController
}

func (s *MatchSquadVoiceGRPC) CreateMatchSquadRoom(ctx context.Context, req *callsv1.CreateMatchSquadRoomRequest) (*callsv1.CreateMatchSquadRoomResponse, error) {
	if s == nil || s.Controller == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad provider unavailable")
	}
	encoded, err := s.Controller.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	receipt := new(callsv1.MatchSquadRoomReceipt)
	if err := proto.Unmarshal(encoded, receipt); err != nil {
		return nil, status.Error(codes.Internal, "stored Voice MatchSquad receipt is invalid")
	}
	return &callsv1.CreateMatchSquadRoomResponse{Receipt: receipt}, nil
}

func (s *MatchSquadVoiceGRPC) TeardownMatchSquadRoom(ctx context.Context, req *callsv1.TeardownMatchSquadRoomRequest) (*callsv1.TeardownMatchSquadRoomResponse, error) {
	if s == nil || s.Controller == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad provider unavailable")
	}
	encoded, err := s.Controller.Teardown(ctx, req)
	if err != nil {
		return nil, err
	}
	receipt := new(callsv1.MatchSquadRoomTeardownReceipt)
	if err := proto.Unmarshal(encoded, receipt); err != nil {
		return nil, status.Error(codes.Internal, "stored Voice MatchSquad teardown receipt is invalid")
	}
	return &callsv1.TeardownMatchSquadRoomResponse{Receipt: receipt}, nil
}

func (s *MatchSquadVoiceGRPC) CompactMatchSquadRoom(ctx context.Context, req *callsv1.CompactMatchSquadRoomRequest) (*callsv1.CompactMatchSquadRoomResponse, error) {
	if s == nil || s.Controller == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad provider unavailable")
	}
	encoded, err := s.Controller.Compact(ctx, req)
	if err != nil {
		return nil, err
	}
	receipt := new(callsv1.MatchSquadRoomCompactionReceipt)
	if err := proto.Unmarshal(encoded, receipt); err != nil {
		return nil, status.Error(codes.Internal, "stored Voice MatchSquad compaction receipt is invalid")
	}
	return &callsv1.CompactMatchSquadRoomResponse{Receipt: receipt}, nil
}
