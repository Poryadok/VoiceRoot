package storyconsume

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/matchmaking/internal/store"
)

func TestStoryLfpJetStreamSubjectsMatchPublisher(t *testing.T) {
	t.Parallel()
	require.Equal(t, "story.lfp_created", subjectStoryLfpCreated)
	require.Equal(t, "story.lfp_response", subjectStoryLfpResponse)
	require.Equal(t, "matchmaking_story_lfp_v2", defaultDurable)
}

func TestApplyStoryEvent_InvalidIDIsPermanent(t *testing.T) {
	err := ApplyStoryEvent(&store.LfpStore{}, &eventsv1.StoryStreamEvent{Payload: &eventsv1.StoryStreamEvent_StoryLfpCreated{StoryLfpCreated: &eventsv1.StoryLfpCreated{StoryId: "not-a-uuid", AuthorProfileId: uuid.NewString()}}})
	require.Error(t, err)
	require.True(t, errors.Is(err, errInvalidStoryEvent))
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

func TestApplyStoryEvent_LfpCreatedReplayPreservesInactiveListing(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	lfp := &store.LfpStore{Pool: pool}
	storyID, authorID := uuid.New(), uuid.New()
	criteria := `{"game_id":"` + uuid.NewString() + `","region":"eu"}`
	event := &eventsv1.StoryStreamEvent{
		EventId: uuid.NewString(), OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.StoryStreamEvent_StoryLfpCreated{
			StoryLfpCreated: &eventsv1.StoryLfpCreated{
				StoryId: storyID.String(), AuthorProfileId: authorID.String(), CriteriaJson: criteria,
			},
		},
	}
	require.NoError(t, ApplyStoryEvent(lfp, event))
	created, err := lfp.GetListing(ctx, storyID)
	require.NoError(t, err)
	require.Nil(t, created.InactiveAt)
	require.NoError(t, ApplyStoryEvent(lfp, event))
	activeReplay, err := lfp.GetListing(ctx, storyID)
	require.NoError(t, err)
	require.Equal(t, created, activeReplay)

	// Party acceptance deactivates this listing; transport replay is not a new publication.
	require.NoError(t, lfp.MarkListingInactive(ctx, storyID))
	inactive, err := lfp.GetListing(ctx, storyID)
	require.NoError(t, err)
	require.NotNil(t, inactive.InactiveAt)
	require.NoError(t, ApplyStoryEvent(lfp, event))
	replayed, err := lfp.GetListing(ctx, storyID)
	require.NoError(t, err)
	require.Equal(t, inactive, replayed, "the same creation event must preserve deactivation")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM lfp_listings WHERE story_id=$1`, storyID).Scan(&count))
	require.Equal(t, 1, count)

	// A distinct Story publication still creates its own active listing.
	fresh := proto.Clone(event).(*eventsv1.StoryStreamEvent)
	fresh.EventId = uuid.NewString()
	freshID := uuid.New()
	fresh.GetStoryLfpCreated().StoryId = freshID.String()
	require.NoError(t, ApplyStoryEvent(lfp, fresh))
	listing, err := lfp.GetListing(ctx, freshID)
	require.NoError(t, err)
	require.Nil(t, listing.InactiveAt)
	require.Equal(t, authorID, listing.AuthorProfileID)
	require.JSONEq(t, criteria, listing.CriteriaJSON)
}
