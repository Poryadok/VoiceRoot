package correlation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"google.golang.org/grpc/metadata"
)

const (
	// RequestIDHeader is the HTTP header name for request correlation.
	RequestIDHeader = "X-Request-Id"
	// GRPCMetadataKey is the gRPC metadata key for request correlation.
	GRPCMetadataKey = "x-request-id"
)

// GenerateRequestID returns a random hex correlation id.
func GenerateRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(b[:])
}

// FromGRPC reads x-request-id from incoming gRPC metadata.
func FromGRPC(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get(GRPCMetadataKey)
	if len(vals) == 0 {
		return ""
	}
	return strings.TrimSpace(vals[0])
}

// WithGRPC attaches x-request-id to outgoing gRPC metadata for tests and clients.
func WithGRPC(ctx context.Context, id string) context.Context {
	id = strings.TrimSpace(id)
	if id == "" {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs(GRPCMetadataKey, id))
}

// OutgoingGRPC attaches x-request-id to outgoing metadata when present in ctx or id arg.
func OutgoingGRPC(ctx context.Context, id string) context.Context {
	id = strings.TrimSpace(id)
	if id == "" {
		id = FromGRPC(ctx)
	}
	if id == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, GRPCMetadataKey, id)
}
