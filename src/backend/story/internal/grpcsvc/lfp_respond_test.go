package grpcsvc_test

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	grpcsvc "voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/store"
	"voice/backend/pkg/integrationtest"

	storyv1 "voice.app/voice/story/v1"
)

type lfpRecordingPublisher struct {
	events []string
}

func (r *lfpRecordingPublisher) PublishStoryCreated(context.Context, string, string, string, string, []string) error {
	return nil
}
func (r *lfpRecordingPublisher) PublishStoryViewed(context.Context, string, string) error { return nil }
func (r *lfpRecordingPublisher) PublishStoryReacted(context.Context, string, string, string) error {
	return nil
}
func (r *lfpRecordingPublisher) PublishStoryExpired(context.Context, string) error { return nil }
func (r *lfpRecordingPublisher) PublishStoryHighlightCreated(context.Context, string, string) error {
	return nil
}
func (r *lfpRecordingPublisher) PublishStoryLfpCreated(context.Context, string, string, string) error {
	r.events = append(r.events, "StoryLfpCreated")
	return nil
}
func (r *lfpRecordingPublisher) PublishStoryLfpResponse(context.Context, string, string, string, string) error {
	r.events = append(r.events, "StoryLfpResponse")
	return nil
}
func (r *lfpRecordingPublisher) Close() error { return nil }

func TestRespondToLfpStory_publishesJoinResponse(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storylfp", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)

	st := &store.StoryStore{Pool: pool}
	svc := grpcsvc.NewStoryGRPC(st)
	rec := &lfpRecordingPublisher{}
	svc.Events = rec

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	storyv1.RegisterStoryServiceServer(s, svc)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() { s.Stop() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := storyv1.NewStoryServiceClient(conn)

	author := uuid.New()
	viewer := uuid.New()
	create, err := client.CreateLookingForParty(withProfile(ctx, uuid.New(), author), &storyv1.CreateLookingForPartyRequest{
		CriteriaJson: `{"game_id":"dota-2","mode":"5v5","visibility":"everyone"}`,
	})
	require.NoError(t, err)
	require.Contains(t, rec.events, "StoryLfpCreated")

	_, err = client.RespondToLfpStory(withProfile(ctx, uuid.New(), viewer), &storyv1.RespondToLfpStoryRequest{
		StoryId:      create.GetStory().GetId(),
		ResponseType: "JOIN",
	})
	require.NoError(t, err)
	require.Contains(t, rec.events, "StoryLfpResponse")
}

func TestRespondToLfpStory_rejectsOwnStory(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	create, err := client.CreateLookingForParty(withProfile(context.Background(), uuid.New(), author), &storyv1.CreateLookingForPartyRequest{
		CriteriaJson: `{"game_id":"dota-2","visibility":"everyone"}`,
	})
	require.NoError(t, err)

	_, err = client.RespondToLfpStory(withProfile(context.Background(), uuid.New(), author), &storyv1.RespondToLfpStoryRequest{
		StoryId:      create.GetStory().GetId(),
		ResponseType: "INVITE",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
