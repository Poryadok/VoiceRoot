package authctx

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

const (
	HeaderUserID    = "x-voice-user-id"
	HeaderProfileID = "x-voice-profile-id"
)

// ProfileID returns the caller's active profile UUID from incoming gRPC metadata.
func ProfileID(ctx context.Context) (uuid.UUID, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, errMissingProfile
	}
	vals := md.Get(HeaderProfileID)
	if len(vals) == 0 || vals[0] == "" {
		return uuid.Nil, errMissingProfile
	}
	id, err := uuid.Parse(vals[0])
	if err != nil {
		return uuid.Nil, errMissingProfile
	}
	return id, nil
}

var errMissingProfile = &profileError{}

// ErrMissingAccount reports absent account metadata.
func ErrMissingAccount() error { return errMissingAccount }

var errMissingAccount = &accountError{}

type accountError struct{}

func (e *accountError) Error() string { return "missing account" }

type profileError struct{}

func (e *profileError) Error() string { return "missing profile" }
