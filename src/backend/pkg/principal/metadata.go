package principal

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/metadata"
)

const (
	authorizationMetadataKey = "authorization"
	requestIDMetadataKey     = "x-request-id"
)

// ErrInvalidPrincipalMetadata intentionally gives no caller-controlled detail.
var ErrInvalidPrincipalMetadata = errors.New("invalid principal metadata")

// TransportMetadata contains the only metadata accepted by a protected RPC.
type TransportMetadata struct {
	BearerToken string
	RequestID   string
}

// IncomingMetadata accepts only one signed principal bearer and one request id.
// It rejects raw identity metadata at strict protected-RPC boundaries.
func IncomingMetadata(ctx context.Context) (TransportMetadata, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || hasRawIdentityMetadata(md) {
		return TransportMetadata{}, ErrInvalidPrincipalMetadata
	}
	authorization, requestIDs := md.Get(authorizationMetadataKey), md.Get(requestIDMetadataKey)
	if len(authorization) != 1 || len(requestIDs) != 1 {
		return TransportMetadata{}, ErrInvalidPrincipalMetadata
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authorization[0], prefix) {
		return TransportMetadata{}, ErrInvalidPrincipalMetadata
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization[0], prefix))
	requestID := strings.TrimSpace(requestIDs[0])
	if token == "" || requestID == "" || strings.ContainsAny(token, " \t\r\n") {
		return TransportMetadata{}, ErrInvalidPrincipalMetadata
	}
	return TransportMetadata{BearerToken: token, RequestID: requestID}, nil
}

func hasRawIdentityMetadata(md metadata.MD) bool {
	for _, key := range []string{"x-voice-profile-id", "x-voice-account-id", "x-voice-user-id", "x-voice-internal-caller"} {
		if len(md.Get(key)) != 0 {
			return true
		}
	}
	return false
}
