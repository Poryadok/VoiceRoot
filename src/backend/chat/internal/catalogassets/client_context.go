package catalogassets

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
)

func bindServiceCall(ctx context.Context, issuer *principal.Issuer, audience, method string, req proto.Message, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "catalog context required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, status.FromContextError(err).Err()
	}
	if issuer == nil {
		return nil, nil, status.Error(codes.Unavailable, "catalog service signer unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, nil, status.Error(codes.InvalidArgument, "invalid catalog service request")
	}
	requestID := uuid.NewString()
	token, err := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, nil, status.Error(codes.Unavailable, "catalog service signer unavailable")
	}
	call, cancel := context.WithTimeout(ctx, timeout)
	return metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), cancel, nil
}
