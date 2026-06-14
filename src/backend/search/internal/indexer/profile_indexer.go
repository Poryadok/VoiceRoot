package indexer

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	eventsv1 "voice.app/voice/events/v1"

	"voice/backend/search/internal/store"
)

// ProfileStore indexes profile discovery documents.
type ProfileStore interface {
	UpsertProfile(ctx context.Context, doc store.ProfileDocument) error
	DeleteProfile(ctx context.Context, profileID uuid.UUID) error
}

// ProfileHydrator loads profile fields for indexing.
type ProfileHydrator interface {
	LoadProfile(ctx context.Context, profileID uuid.UUID) (accountID uuid.UUID, username, discriminator, displayName, verificationType string, err error)
}

// ProfileIndexer handles user.events profile payloads.
type ProfileIndexer struct {
	Store     ProfileStore
	Profiles  ProfileHydrator
}

// Handle processes profile_created and profile_updated events.
func (idx *ProfileIndexer) Handle(ctx context.Context, env *eventsv1.UserStreamEvent) error {
	if idx == nil || env == nil {
		return nil
	}
	if created := env.GetProfileCreated(); created != nil {
		return idx.upsert(ctx, created.GetProfileId())
	}
	return nil
}

func (idx *ProfileIndexer) upsert(ctx context.Context, profileRaw string) error {
	if idx.Store == nil || idx.Profiles == nil {
		return fmt.Errorf("profile indexer not configured")
	}
	profileID, err := uuid.Parse(profileRaw)
	if err != nil {
		return fmt.Errorf("invalid profile_id: %w", err)
	}
	accountID, username, discriminator, displayName, verificationType, err := idx.Profiles.LoadProfile(ctx, profileID)
	if err != nil {
		return err
	}
	return idx.Store.UpsertProfile(ctx, store.ProfileDocument{
		ProfileID:        profileID,
		AccountID:        accountID,
		Username:         username,
		Discriminator:    discriminator,
		DisplayName:      displayName,
		VerificationType: verificationType,
	})
}
