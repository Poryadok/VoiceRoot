package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

type allowAllSpaceQueue struct{}

func (allowAllSpaceQueue) EnsureMemberAndMMEnabled(context.Context, uuid.UUID) error {
	return nil
}

type denySpaceQueue struct {
	code codes.Code
	msg  string
}

func (d denySpaceQueue) EnsureMemberAndMMEnabled(context.Context, uuid.UUID) error {
	return status.Error(d.code, d.msg)
}

func TestStartSpaceQueue_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	pool := startDB(t, context.Background())
	srv := searchTestServer(t, pool)
	srv.SpaceQueue = allowAllSpaceQueue{}
	profileID := uuid.New()
	ctx := ctxWithProfile(profileID)
	spaceID := uuid.New().String()

	gameID := dotaGameID(t, srv, ctx)
	resp, err := srv.StartSpaceQueue(ctx, &matchmakingv1.StartSpaceQueueRequest{
		SpaceId:      spaceID,
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.NoError(t, err)
	require.Equal(t, "searching", resp.GetSearchSession().GetStatus())
	require.Equal(t, spaceID, resp.GetSearchSession().GetSpaceId())
	require.Equal(t, profileID.String(), resp.GetSearchSession().GetProfileId())

	_, err = srv.CancelSearch(ctx, &matchmakingv1.CancelSearchRequest{
		SessionId: resp.GetSearchSession().GetId(),
	})
	require.NoError(t, err)
}

func TestStartSpaceQueue_NotMemberDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	pool := startDB(t, context.Background())
	srv := searchTestServer(t, pool)
	srv.SpaceQueue = denySpaceQueue{code: codes.PermissionDenied, msg: "not a space member"}
	ctx := ctxWithProfile(uuid.New())
	gameID := dotaGameID(t, srv, ctx)

	_, err := srv.StartSpaceQueue(ctx, &matchmakingv1.StartSpaceQueueRequest{
		SpaceId:      uuid.New().String(),
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
