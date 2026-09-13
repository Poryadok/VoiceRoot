package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcsvc "voice/backend/story/internal/grpcsvc"

	storyv1 "voice.app/voice/story/v1"
)

func int32Ptr(value int32) *int32 { return &value }

type storyMediaCheckerFake struct {
	metadata grpcsvc.StoryMediaMetadata
	err      error
}

func (f storyMediaCheckerFake) GetStoryMediaMetadata(context.Context, uuid.UUID) (grpcsvc.StoryMediaMetadata, error) {
	return f.metadata, f.err
}

func TestCreateStory_mediaValidationFailsClosedWhenFileUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()
	profile := uuid.New()
	mediaID := uuid.NewString()
	_, err := client.CreateStory(withProfile(context.Background(), uuid.New(), profile), &storyv1.CreateStoryRequest{Type: "photo", MediaFileId: &mediaID})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestCreateStory_mediaValidationRejectsForeignAndUnavailableFile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profile := uuid.New()
	mediaID := uuid.NewString()
	foreign := storyMediaCheckerFake{metadata: grpcsvc.StoryMediaMetadata{UploaderProfileID: uuid.New(), Status: "ready", ScanResult: "clean", FileType: "image"}}
	client, _, cleanup := startStoryGRPCWithFiles(t, foreign)
	defer cleanup()
	_, err := client.CreateStory(withProfile(context.Background(), uuid.New(), profile), &storyv1.CreateStoryRequest{Type: "photo", MediaFileId: &mediaID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	unavailableClient, _, unavailableCleanup := startStoryGRPCWithFiles(t, storyMediaCheckerFake{err: status.Error(codes.Unavailable, "file unavailable")})
	defer unavailableCleanup()
	_, err = unavailableClient.CreateStory(withProfile(context.Background(), uuid.New(), profile), &storyv1.CreateStoryRequest{Type: "photo", MediaFileId: &mediaID})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestCreateStory_mediaValidationEnforcesImageAndVideoMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profile := uuid.New()
	mediaID := uuid.NewString()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	validImage := storyMediaCheckerFake{metadata: grpcsvc.StoryMediaMetadata{UploaderProfileID: profile, Status: "ready", ScanResult: "clean", FileType: "image"}}
	client, _, cleanup := startStoryGRPCWithFiles(t, validImage)
	defer cleanup()
	_, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{Type: "photo", MediaFileId: &mediaID})
	require.NoError(t, err)
	wrongImageClient, _, wrongImageCleanup := startStoryGRPCWithFiles(t, storyMediaCheckerFake{metadata: grpcsvc.StoryMediaMetadata{UploaderProfileID: profile, Status: "ready", ScanResult: "clean", FileType: "video"}})
	defer wrongImageCleanup()
	_, err = wrongImageClient.CreateStory(ctx, &storyv1.CreateStoryRequest{Type: "photo", MediaFileId: &mediaID})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	longVideoClient, _, longVideoCleanup := startStoryGRPCWithFiles(t, storyMediaCheckerFake{metadata: grpcsvc.StoryMediaMetadata{UploaderProfileID: profile, Status: "ready", ScanResult: "clean", FileType: "video", DurationSeconds: int32Ptr(61)}})
	defer longVideoCleanup()
	_, err = longVideoClient.CreateStory(ctx, &storyv1.CreateStoryRequest{Type: "video", MediaFileId: &mediaID})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateStory_mediaValidationRejectsChatScopedAndUnreadyFile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profile := uuid.New()
	mediaID := uuid.NewString()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	for _, tc := range []struct {
		name string
		meta grpcsvc.StoryMediaMetadata
	}{
		{name: "chat context", meta: grpcsvc.StoryMediaMetadata{UploaderProfileID: profile, Status: "ready", ScanResult: "clean", FileType: "image", HasChatContext: true}},
		{name: "not ready", meta: grpcsvc.StoryMediaMetadata{UploaderProfileID: profile, Status: "processing", ScanResult: "pending", FileType: "image"}},
		{name: "infected", meta: grpcsvc.StoryMediaMetadata{UploaderProfileID: profile, Status: "failed", ScanResult: "infected", FileType: "image"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, _, cleanup := startStoryGRPCWithFiles(t, storyMediaCheckerFake{metadata: tc.meta})
			defer cleanup()
			_, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{Type: "photo", MediaFileId: &mediaID})
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
		})
	}
}

func TestCreateLookingForParty_mediaValidationRejectsForeignFile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profile := uuid.New()
	mediaID := uuid.NewString()
	client, _, cleanup := startStoryGRPCWithFiles(t, storyMediaCheckerFake{metadata: grpcsvc.StoryMediaMetadata{UploaderProfileID: uuid.New(), Status: "ready", ScanResult: "clean", FileType: "image"}})
	defer cleanup()
	_, err := client.CreateLookingForParty(withProfile(context.Background(), uuid.New(), profile), &storyv1.CreateLookingForPartyRequest{CriteriaJson: `{"visibility":"everyone"}`, MediaFileId: &mediaID})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
