package authctx

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ServiceIdentity is set only after a server-side S2S credential verifier has
// authenticated the caller. It must never be derived from forwarded metadata.
type ServiceIdentity string

const ServiceIdentityVoice ServiceIdentity = "voice"

type serviceIdentityContextKey struct{}

func WithVerifiedServiceIdentity(ctx context.Context, identity ServiceIdentity) context.Context {
	return context.WithValue(ctx, serviceIdentityContextKey{}, identity)
}

func VerifiedServiceIdentity(ctx context.Context) (ServiceIdentity, bool) {
	identity, ok := ctx.Value(serviceIdentityContextKey{}).(ServiceIdentity)
	return identity, ok && identity != ""
}

// VerifiedServiceIdentityUnaryInterceptor verifies the resolver's dedicated
// bearer credential and exposes only a server-side identity to the handler.
// An empty configured token fails closed.
func VerifiedServiceIdentityUnaryInterceptor(expectedVoiceToken string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != "/voice.space.v1.SpaceService/ResolveVoiceRoomAccess" {
			return handler(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		want := "Bearer " + strings.TrimSpace(expectedVoiceToken)
		values := md.Get("authorization")
		if strings.TrimSpace(expectedVoiceToken) == "" || len(values) != 1 || values[0] != want {
			return nil, status.Error(codes.Unauthenticated, "verified Voice service identity required")
		}
		return handler(WithVerifiedServiceIdentity(ctx, ServiceIdentityVoice), req)
	}
}

// Metadata keys aligned with Gateway downstream headers (see gateway applyClaims).
const (
	HeaderUserID    = "x-voice-user-id"    // JWT claim user_id == account_id
	HeaderProfileID = "x-voice-profile-id" // active profile_id
)

// AccountID returns the caller's account UUID from incoming gRPC metadata, if present and valid.
func AccountID(ctx context.Context) (uuid.UUID, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	vals := md.Get(HeaderUserID)
	if len(vals) == 0 || vals[0] == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(vals[0])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// ProfileID returns the caller's active profile UUID from incoming gRPC metadata, if present and valid.
func ProfileID(ctx context.Context) (uuid.UUID, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	vals := md.Get(HeaderProfileID)
	if len(vals) == 0 || vals[0] == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(vals[0])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
