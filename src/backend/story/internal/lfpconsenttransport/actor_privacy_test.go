package lfpconsenttransport

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/privacy"
	"voice/backend/story/internal/lfpconsentstore"
)

type actorFloorFunc func(context.Context, uuid.UUID) (privacy.Audience, error)

func (f actorFloorFunc) ShowStoriesAudience(c context.Context, id uuid.UUID) (privacy.Audience, error) {
	return f(c, id)
}

type actorSocial struct{ err error }

func (s actorSocial) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.err == nil, s.err
}
func (s actorSocial) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, s.err
}

func TestActorPrivacyUsesOriginalGuestAndFreshFloor(t *testing.T) {
	for _, kind := range []uint64{lfpconsent.Guest, lfpconsent.Regular} {
		journal, envelope := actorJournalFixture(t, kind)
		_, bound, e := journal.ReadActorConsentContext(context.Background(), envelope.Consent().ConsentID, envelope.QualifiedSHA256(), envelope.Origin())
		require.NoError(t, e)
		for _, tt := range []struct {
			name            string
			floor           privacy.Audience
			outage, missing bool
			denied          bool
		}{
			{name: "explicit everyone includes guest", floor: privacy.EveryoneWithGuests()},
			{name: "friends only", floor: privacy.FriendsOnly(), denied: kind == lfpconsent.Guest},
			{name: "nobody", floor: privacy.Nobody(), denied: true},
			{name: "floor outage", outage: true},
			{name: "missing social authority", floor: privacy.FriendsOnly(), missing: true, denied: kind == lfpconsent.Guest},
		} {
			t.Run(tt.name, func(t *testing.T) {
				adapter := &ActorPrivacy{Matcher: privacy.Matcher{Social: actorSocial{}}, Floor: actorFloorFunc(func(_ context.Context, id uuid.UUID) (privacy.Audience, error) {
					require.Equal(t, envelope.Consent().AuthorProfileID, id)
					if tt.outage {
						return privacy.Audience{}, errors.New("offline")
					}
					return tt.floor, nil
				})}
				if tt.missing {
					adapter.Matcher.Social = nil
				}
				e := adapter.CheckActorResponder(context.Background(), bound, envelope.Actor())
				switch {
				case tt.outage || tt.missing && kind == lfpconsent.Regular:
					require.ErrorIs(t, e, lfpconsentstore.ErrUnavailable)
				case tt.denied:
					require.ErrorIs(t, e, ErrResponderIneligible)
				default:
					require.NoError(t, e)
				}
			})
		}
	}
}

func TestActorAudienceRejectsUnknownInsteadOfFallback(t *testing.T) {
	for _, raw := range []string{"null", "[]", "{bad}", `{"include_guests":true,"unknown":true}`, `{"space_ids":["x"]}`} {
		_, e := strictActorAudience("everyone", raw)
		require.Error(t, e, raw)
	}
	_, e := strictActorAudience("unknown", "")
	require.Error(t, e)
	guest, e := strictActorAudience("custom", `{"include_guests":true}`)
	require.NoError(t, e)
	require.True(t, guest.IncludeGuests)
}
