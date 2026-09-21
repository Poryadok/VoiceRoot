package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/socialprincipal"
	"voice/backend/user/internal/searchprojection"

	userv1 "voice.app/voice/user/v1"
)

// SearchProjectionGRPC exposes only User's authoritative bootstrap/replay API
// on the Search TLS principal listener.
type SearchProjectionGRPC struct {
	userv1.UnimplementedUserServiceServer
	User      *UserGRPC
	CursorKey []byte
}

func (s *SearchProjectionGRPC) BeginSearchProfileSnapshot(ctx context.Context, req *userv1.BeginSearchProfileSnapshotRequest) (*userv1.BeginSearchProfileSnapshotResponse, error) {
	if err := socialprincipal.RequireSocialForMethod(ctx, "search", userv1.UserService_BeginSearchProfileSnapshot_FullMethodName, req); err != nil {
		return nil, err
	}
	high, err := s.User.Profiles.SearchProjectionCheckpoint(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.BeginSearchProfileSnapshotResponse{HighWatermark: high}, nil
}
func (s *SearchProjectionGRPC) ListSearchProfileSnapshot(ctx context.Context, req *userv1.ListSearchProfileSnapshotRequest) (*userv1.ListSearchProfileSnapshotResponse, error) {
	if err := socialprincipal.RequireSocialForMethod(ctx, "search", userv1.UserService_ListSearchProfileSnapshot_FullMethodName, req); err != nil {
		return nil, err
	}
	var after *uuid.UUID
	if raw := strings.TrimSpace(req.GetCursor()); raw != "" {
		value, err := searchprojection.DecodeCursor(s.CursorKey, raw)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid snapshot cursor")
		}
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid snapshot cursor")
		}
		after = &id
	}
	events, next, err := s.User.Profiles.ListSearchProjectionSnapshot(ctx, after, int(req.GetPageSize()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	response := &userv1.ListSearchProfileSnapshotResponse{Events: events}
	if next != nil {
		cursor, err := searchprojection.EncodeCursor(s.CursorKey, next.String())
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		response.NextCursor = cursor
	}
	return response, nil
}
func (s *SearchProjectionGRPC) ListSearchProfileJournal(ctx context.Context, req *userv1.ListSearchProfileJournalRequest) (*userv1.ListSearchProfileJournalResponse, error) {
	if err := socialprincipal.RequireSocialForMethod(ctx, "search", userv1.UserService_ListSearchProfileJournal_FullMethodName, req); err != nil {
		return nil, err
	}
	events, high, err := s.User.Profiles.ListSearchProjectionJournal(ctx, req.GetAfterOffset(), int(req.GetPageSize()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.ListSearchProfileJournalResponse{Events: events, HighWatermark: high}, nil
}
func (s *SearchProjectionGRPC) GetSearchProfileCheckpoint(ctx context.Context, req *userv1.GetSearchProfileCheckpointRequest) (*userv1.GetSearchProfileCheckpointResponse, error) {
	if err := socialprincipal.RequireSocialForMethod(ctx, "search", userv1.UserService_GetSearchProfileCheckpoint_FullMethodName, req); err != nil {
		return nil, err
	}
	high, err := s.User.Profiles.SearchProjectionCheckpoint(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.GetSearchProfileCheckpointResponse{HighWatermark: high}, nil
}
func RegisterSearchProjectionServer(server grpc.ServiceRegistrar, user *UserGRPC, cursorKey []byte) {
	desc := userv1.UserService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range userv1.UserService_ServiceDesc.Methods {
		switch method.MethodName {
		case "BeginSearchProfileSnapshot", "ListSearchProfileSnapshot", "ListSearchProfileJournal", "GetSearchProfileCheckpoint":
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, &SearchProjectionGRPC{User: user, CursorKey: cursorKey})
}
