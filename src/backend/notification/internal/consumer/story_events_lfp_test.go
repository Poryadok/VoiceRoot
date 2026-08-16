package consumer

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

func TestHandleStoryLfpResponse_JoinRoutesToAuthor(t *testing.T) {
	authorID := uuid.NewString()
	responderID := uuid.NewString()
	handler := &StoryEventHandler{Router: delivery.DecideRouting}
	decisions := handler.HandleStoryLfpResponse(context.Background(), &eventsv1.StoryLfpResponse{
		StoryId:            uuid.NewString(),
		AuthorProfileId:    authorID,
		ResponderProfileId: responderID,
		ResponseType:       "JOIN",
	})
	require.Contains(t, decisions, authorID)
	require.True(t, decisions[authorID].InApp)
	require.NotContains(t, decisions, responderID)
}

func TestHandleStoryLfpResponse_InviteRoutesToAuthor(t *testing.T) {
	authorID := uuid.NewString()
	responderID := uuid.NewString()
	handler := &StoryEventHandler{Router: delivery.DecideRouting}
	decisions := handler.HandleStoryLfpResponse(context.Background(), &eventsv1.StoryLfpResponse{
		StoryId:            uuid.NewString(),
		AuthorProfileId:    authorID,
		ResponderProfileId: responderID,
		ResponseType:       "INVITE",
	})
	require.Contains(t, decisions, authorID)
	require.True(t, decisions[authorID].Push || decisions[authorID].InApp)
}
