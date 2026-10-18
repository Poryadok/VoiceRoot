package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	spacev1 "voice.app/voice/space/v1"
)

func TestMemberNickname_RawActorCannotReachPersistence(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString(), "x-voice-user-id", uuid.NewString()))
	_, err := (&SpaceGRPC{}).UpdateMemberNickname(ctx, &spacev1.UpdateMemberNicknameRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString()})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
