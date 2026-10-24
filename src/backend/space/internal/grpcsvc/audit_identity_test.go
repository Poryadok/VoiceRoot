package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// boundAuditFixture is only for ledger/ACL tests. Network admission is covered
// separately by the real Gateway/mTLS/JWKS/Redis/PostgreSQL route fixture.
func boundAuditFixture(ctx context.Context, account, profile uuid.UUID, request *spacev1.GetAuditLogRequest) context.Context {
	hash, err := principal.RequestHash(request)
	if err != nil {
		panic(err)
	}
	return principal.WithVerified(ctx, principal.Principal{Kind: "delegated_user", Issuer: "gateway",
		Subject: account.String(), AccountID: account.String(), ProfileID: profile.String(),
		SessionEpoch: 7, Audience: "space", RPC: spacev1.SpaceService_GetAuditLog_FullMethodName,
		RequestID: uuid.NewString(), RequestHash: hash})
}

func TestAuditIdentityBeforeAnyStoreOrRoleAccess(t *testing.T) {
	request := &spacev1.GetAuditLogRequest{SpaceId: uuid.NewString()}
	raw := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString()))
	for _, ctx := range []context.Context{context.Background(), raw} {
		response, err := (*SpaceGRPC)(nil).GetAuditLog(ctx, request)
		require.Nil(t, response)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	ctx := boundAuditFixture(context.Background(), uuid.New(), uuid.New(), request)
	response, err := (*SpaceGRPC)(nil).GetAuditLog(ctx, proto.Clone(request).(*spacev1.GetAuditLogRequest))
	require.Nil(t, response)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "valid identity reaches persistence boundary")
	request.SpaceId = uuid.NewString()
	_, err = (*SpaceGRPC)(nil).GetAuditLog(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "request-bound identity cannot authorize another query")
}
