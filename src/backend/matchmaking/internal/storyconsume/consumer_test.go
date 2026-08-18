package storyconsume

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/store"
	eventsv1 "voice.app/voice/events/v1"
)

func TestStoryLfpJetStreamSubjectMatchesPublisher(t *testing.T) {
	t.Parallel()
	require.Equal(t, "story.>", jsSubjectStoryLfp)
	require.Equal(t, "matchmaking_story_lfp_v2", defaultDurable)
}

func TestApplyStoryEvent_LfpCreatedAndJoinResponse(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	lfp := &store.LfpStore{Pool: pool}

	storyID := uuid.New()
	authorID := uuid.New()
	responderID := uuid.New()
	gameID := uuid.New()
	criteria := `{"game_id":"` + gameID.String() + `","mode":"5v5 Ranked","region":"eu","self":{"role":"Carry","rank":"Herald"}}`

	err := ApplyStoryEvent(lfp, &eventsv1.StoryStreamEvent{
		Payload: &eventsv1.StoryStreamEvent_StoryLfpCreated{
			StoryLfpCreated: &eventsv1.StoryLfpCreated{
				StoryId:         storyID.String(),
				AuthorProfileId: authorID.String(),
				CriteriaJson:    criteria,
			},
		},
	})
	require.NoError(t, err)

	listing, err := lfp.GetListing(ctx, storyID)
	require.NoError(t, err)
	require.JSONEq(t, criteria, listing.CriteriaJSON)

	err = ApplyStoryEvent(lfp, &eventsv1.StoryStreamEvent{
		Payload: &eventsv1.StoryStreamEvent_StoryLfpResponse{
			StoryLfpResponse: &eventsv1.StoryLfpResponse{
				StoryId:            storyID.String(),
				AuthorProfileId:    authorID.String(),
				ResponderProfileId: responderID.String(),
				ResponseType:       "JOIN",
			},
		},
	})
	require.NoError(t, err)

	req, err := lfp.GetPendingRequest(ctx, storyID, responderID, store.LfpResponseJoin)
	require.NoError(t, err)
	require.Equal(t, store.LfpRequestPending, req.Status)
	require.Equal(t, authorID, req.AuthorProfileID)
}
