package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/privacy"
	"voice/backend/story/internal/store"
)

type testStoryPrivacy struct {
	audience privacy.Audience
	err      error
}

func (p testStoryPrivacy) ShowStoriesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return p.audience, p.err
}

func TestCanViewStoryFailsClosedWhenCurrentPrivacyIsUnavailable(t *testing.T) {
	author, viewer := uuid.New(), uuid.New()

	for _, tc := range []struct {
		name    string
		checker StoryPrivacyChecker
	}{
		{name: "missing checker"},
		{name: "lookup error", checker: testStoryPrivacy{err: errors.New("user unavailable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &StoryGRPC{Privacy: tc.checker}
			for _, visibility := range []string{"everyone", "friends"} {
				row := &store.StoryRow{AuthorProfileID: author, Visibility: visibility, ExpiresAt: time.Now().Add(time.Hour)}
				require.False(t, svc.canViewActiveStory(context.Background(), viewer, row), visibility)
				// Author archive access does not depend on the User privacy service.
				require.True(t, svc.canViewActiveStory(context.Background(), author, row))
			}
		})
	}
}

func TestCapCreateStoryVisibilityFailsBeforeReturningPermissivePolicy(t *testing.T) {
	profileID := uuid.New()
	for _, tc := range []struct {
		name    string
		checker StoryPrivacyChecker
	}{
		{name: "missing checker"},
		{name: "lookup error", checker: testStoryPrivacy{err: errors.New("user unavailable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &StoryGRPC{Privacy: tc.checker}
			_, _, err := svc.capCreateStoryVisibility(context.Background(), profileID, "everyone", nil)
			require.Equal(t, codes.Unavailable, status.Code(err))
		})
	}
}

func TestStoryPrivacyFloorFailsWhenUserPolicyIsUnavailable(t *testing.T) {
	svc := &StoryGRPC{Privacy: testStoryPrivacy{err: errors.New("user unavailable")}}
	_, err := svc.storyPrivacyFloor(context.Background(), uuid.New())
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestCanViewStoryAppliesSuccessfullyLoadedNobodyFloor(t *testing.T) {
	svc := &StoryGRPC{Privacy: testStoryPrivacy{audience: privacy.Nobody()}}
	row := &store.StoryRow{AuthorProfileID: uuid.New(), Visibility: "everyone", ExpiresAt: time.Now().Add(time.Hour)}
	require.False(t, svc.canViewActiveStory(context.Background(), uuid.New(), row))
}

func TestCanViewStoryUsesSuccessfullyLoadedEveryoneFloor(t *testing.T) {
	svc := &StoryGRPC{Privacy: testStoryPrivacy{audience: privacy.EveryoneWithGuests()}}
	row := &store.StoryRow{AuthorProfileID: uuid.New(), Visibility: "everyone", ExpiresAt: time.Now().Add(time.Hour)}
	require.True(t, svc.canViewActiveStory(context.Background(), uuid.New(), row))
}
