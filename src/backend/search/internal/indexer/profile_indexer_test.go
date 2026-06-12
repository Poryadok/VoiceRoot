package indexer

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"

	"voice/backend/search/internal/store"
)

type recordingProfileStore struct {
	upserts []store.ProfileDocument
}

func (r *recordingProfileStore) UpsertProfile(_ context.Context, doc store.ProfileDocument) error {
	r.upserts = append(r.upserts, doc)
	return nil
}

func (r *recordingProfileStore) DeleteProfile(context.Context, uuid.UUID) error {
	return nil
}

type stubProfileHydrator struct {
	accountID uuid.UUID
	username  string
	disc      string
	display   string
}

func (s *stubProfileHydrator) LoadProfile(_ context.Context, _ uuid.UUID) (uuid.UUID, string, string, string, error) {
	return s.accountID, s.username, s.disc, s.display, nil
}

func TestProfileIndexer_ProfileCreated_UpsertsDocument(t *testing.T) {
	t.Parallel()
	rec := &recordingProfileStore{}
	accountID := uuid.New()
	profileID := uuid.New()
	idx := &ProfileIndexer{
		Store: rec,
		Profiles: &stubProfileHydrator{
			accountID: accountID,
			username:  "alice",
			disc:      "0001",
			display:   "Alice",
		},
	}
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.UserStreamEvent_ProfileCreated{
			ProfileCreated: &eventsv1.ProfileCreated{
				ProfileId: profileID.String(),
				AccountId: accountID.String(),
			},
		},
	}
	require.NoError(t, idx.Handle(context.Background(), env))
	require.Len(t, rec.upserts, 1)
	require.Equal(t, profileID, rec.upserts[0].ProfileID)
	require.Equal(t, accountID, rec.upserts[0].AccountID)
	require.Equal(t, "alice", rec.upserts[0].Username)
}
