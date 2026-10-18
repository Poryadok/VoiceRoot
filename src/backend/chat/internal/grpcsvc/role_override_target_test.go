package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestValidateRoleOverrideTargetRejectsUnverifiedAndWrongCaller(t *testing.T) {
	s := &ChatGRPC{}
	req := &chatv1.ValidateRoleOverrideTargetRequest{SpaceId: uuid.NewString(), ChatId: uuid.NewString()}
	_, err := s.ValidateRoleOverrideTarget(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "chat", RPC: chatv1.ChatService_ValidateRoleOverrideTarget_FullMethodName, RequestHash: hash, RequestID: uuid.NewString()})
	_, err = s.ValidateRoleOverrideTarget(ctx, req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

type overrideTargetDM struct {
	DMStore
	call func(context.Context, uuid.UUID, uuid.UUID) error
}

func (f overrideTargetDM) ValidateRoleOverrideTarget(ctx context.Context, s, c uuid.UUID) error {
	return f.call(ctx, s, c)
}

func TestValidateRoleOverrideTargetCallsOnlyOwnerStore(t *testing.T) {
	req := &chatv1.ValidateRoleOverrideTargetRequest{SpaceId: uuid.NewString(), ChatId: uuid.NewString()}
	verified := func() context.Context {
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "role", Subject: "service:role", Audience: "chat", RPC: chatv1.ChatService_ValidateRoleOverrideTarget_FullMethodName, RequestHash: hash, RequestID: uuid.NewString()})
	}
	calls := 0
	var storeErr error
	// Every non-target DM interface method would panic through the nil embedding.
	svc := &ChatGRPC{DM: overrideTargetDM{call: func(_ context.Context, s, c uuid.UUID) error {
		calls++
		require.Equal(t, req.SpaceId, s.String())
		require.Equal(t, req.ChatId, c.String())
		return storeErr
	}}}
	_, err := svc.ValidateRoleOverrideTarget(verified(), req)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	for _, test := range []struct {
		err  error
		code codes.Code
	}{{pgx.ErrNoRows, codes.NotFound}, {errors.New("state unavailable"), codes.Unavailable}} {
		storeErr = test.err
		_, err = svc.ValidateRoleOverrideTarget(verified(), req)
		require.Equal(t, test.code, status.Code(err))
	}
	ctx := verified()
	req.ChatId = uuid.NewString()
	before := calls
	_, err = svc.ValidateRoleOverrideTarget(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, before, calls)
}

func TestValidateRoleOverrideTargetRejectsInvalidIDsBeforeStore(t *testing.T) {
	for _, raw := range []string{"", uuid.Nil.String(), "bad-id", "123E4567-E89B-12D3-A456-426614174000"} {
		req := &chatv1.ValidateRoleOverrideTargetRequest{SpaceId: raw, ChatId: uuid.NewString()}
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "role", Subject: "service:role", Audience: "chat", RPC: chatv1.ChatService_ValidateRoleOverrideTarget_FullMethodName, RequestHash: hash, RequestID: uuid.NewString()})
		_, err = (&ChatGRPC{}).ValidateRoleOverrideTarget(ctx, req)
		require.Equal(t, codes.InvalidArgument, status.Code(err), raw)
	}
}
