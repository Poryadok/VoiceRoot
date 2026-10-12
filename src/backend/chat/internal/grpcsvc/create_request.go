package grpcsvc

import (
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func parseCreateRequestID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return nil, status.Error(codes.InvalidArgument, "request_id must be a canonical UUID")
	}
	return &id, nil
}
