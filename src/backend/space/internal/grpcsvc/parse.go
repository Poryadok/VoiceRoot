package grpcsvc

import (
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func parseUUIDField(name, value string) (uuid.UUID, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s is required", name)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s", name)
	}
	return id, nil
}
