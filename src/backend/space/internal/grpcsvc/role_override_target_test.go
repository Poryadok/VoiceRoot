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
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestValidateRoleOverrideTargetRejectsUnverifiedAndWrongCaller(t *testing.T) {
	s := &SpaceGRPC{}
	req := &spacev1.ValidateRoleOverrideTargetRequest{SpaceId: uuid.NewString()}
	_, err := s.ValidateRoleOverrideTarget(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "space", RPC: spacev1.SpaceService_ValidateRoleOverrideTarget_FullMethodName, RequestHash: hash, RequestID: uuid.NewString()})
	_, err = s.ValidateRoleOverrideTarget(ctx, req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

type overrideTargetStoreFunc func(context.Context, uuid.UUID, *uuid.UUID) error

func (f overrideTargetStoreFunc) ValidateRoleOverrideTarget(ctx context.Context, s uuid.UUID, r *uuid.UUID) error {
	return f(ctx, s, r)
}

func TestValidateRoleOverrideTargetOptionalPresenceAndStoreErrors(t *testing.T) {
	req := &spacev1.ValidateRoleOverrideTargetRequest{SpaceId: uuid.NewString()}
	verified := func(req *spacev1.ValidateRoleOverrideTargetRequest) context.Context {
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "role", Subject: "service:role", Audience: "space", RPC: spacev1.SpaceService_ValidateRoleOverrideTarget_FullMethodName, RequestHash: hash, RequestID: uuid.NewString()})
	}
	calls := 0
	var storeErr error
	svc := &SpaceGRPC{roleOverrideTargets: overrideTargetStoreFunc(func(_ context.Context, spaceID uuid.UUID, roomID *uuid.UUID) error {
		calls++
		require.Equal(t, req.SpaceId, spaceID.String())
		if req.VoiceRoomId == nil {
			require.Nil(t, roomID)
		} else {
			require.Equal(t, *req.VoiceRoomId, roomID.String())
		}
		return storeErr
	})}
	_, err := svc.ValidateRoleOverrideTarget(verified(req), req)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	empty := ""
	req.VoiceRoomId = &empty
	_, err = svc.ValidateRoleOverrideTarget(verified(req), req)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Equal(t, 1, calls)
	room := uuid.NewString()
	req.VoiceRoomId = &room
	_, err = svc.ValidateRoleOverrideTarget(verified(req), req)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	for _, test := range []struct {
		err  error
		code codes.Code
	}{{pgx.ErrNoRows, codes.NotFound}, {errors.New("state unavailable"), codes.Unavailable}} {
		storeErr = test.err
		_, err = svc.ValidateRoleOverrideTarget(verified(req), req)
		require.Equal(t, test.code, status.Code(err))
	}
	// A credential for absent optional room does not authorize present room.
	req.VoiceRoomId = nil
	ctx := verified(req)
	req.VoiceRoomId = &room
	before := calls
	_, err = svc.ValidateRoleOverrideTarget(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, before, calls)
}

func TestValidateRoleOverrideTargetRejectsInvalidIDsBeforeStore(t *testing.T) {
	for _, raw := range []string{"", uuid.Nil.String(), "bad-id", "123E4567-E89B-12D3-A456-426614174000"} {
		req := &spacev1.ValidateRoleOverrideTargetRequest{SpaceId: raw}
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "role", Subject: "service:role", Audience: "space", RPC: spacev1.SpaceService_ValidateRoleOverrideTarget_FullMethodName, RequestHash: hash, RequestID: uuid.NewString()})
		_, err = (&SpaceGRPC{}).ValidateRoleOverrideTarget(ctx, req)
		require.Equal(t, codes.InvalidArgument, status.Code(err), raw)
	}
}
