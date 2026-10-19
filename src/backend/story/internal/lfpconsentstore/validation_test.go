package lfpconsentstore

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/storylfpadmission"
)

type fixturePeer struct{ o storylfpadmission.Origin }

func (p fixturePeer) ResolveAuthenticatedPeer(context.Context) (storylfpadmission.Origin, error) {
	return p.o, nil
}

func validationFixture(t *testing.T, s *Store) storylfpadmission.VerifiedPeer {
	t.Helper()
	o := s.authority.(fixtureAdmission).o
	shared, e := storylfpadmission.NewOrigin(o.environment, o.installation, o.incarnation, o.database)
	require.NoError(t, e)
	peer, e := storylfpadmission.ResolvePeer(context.Background(), shared, fixturePeer{shared})
	require.NoError(t, e)
	return peer
}

func TestJournalValidationExactCurrentOffer(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	c, _, h, e := s.RecordResponse(ctx, response(o))
	require.NoError(t, e)
	v := Validation{ConsentID: c.ConsentID, StoryID: c.StoryID, Incarnation: c.StoryIncarnation, Revision: c.OfferRevision, ConsentSHA256: h, PayloadSHA256: c.OfferPayloadSHA256, CriteriaSHA256: c.CriteriaSHA256}
	peer := validationFixture(t, s)
	got, e := s.ValidateConsent(ctx, v, peer)
	require.NoError(t, e)
	require.Equal(t, o, got)
	_, e = s.ValidateConsent(ctx, v, storylfpadmission.VerifiedPeer{})
	require.ErrorIs(t, e, ErrUnavailable)
	bad := v
	bad.ConsentSHA256[0]++
	_, e = s.ValidateConsent(ctx, bad, peer)
	require.ErrorIs(t, e, ErrConflict)
	// A new explicit A->A author action invalidates current-offer validation,
	// while replay still returns the original private historical receipt.
	publish(t, s, id)
	_, e = s.ValidateConsent(ctx, v, peer)
	require.ErrorIs(t, e, ErrStale)
}

func TestJournalDeliveryRequiresOriginalAdmittedOrigin(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	c, b, h, e := s.RecordResponse(ctx, response(o))
	require.NoError(t, e)
	expected := validationFixture(t, s).Origin()
	got, e := s.ReadConsentForDelivery(ctx, c.ConsentID, h, expected)
	require.NoError(t, e)
	require.Equal(t, b, got)
	bad := h
	bad[0]++
	got, e = s.ReadConsentForDelivery(ctx, c.ConsentID, bad, expected)
	require.Error(t, e)
	require.Nil(t, got)
	next := s.authority.(fixtureAdmission).o
	next.incarnation = uuid.New()
	_, e = s.Pool.Exec(ctx, "UPDATE story_lfp_admission_head SET authority_incarnation=$1", next.incarnation)
	require.NoError(t, e)
	s.authority = fixtureAdmission{next}
	newExpected := validationFixture(t, s).Origin()
	for _, origin := range []storylfpadmission.Origin{expected, newExpected} {
		got, e = s.ReadConsentForDelivery(ctx, c.ConsentID, h, origin)
		require.ErrorIs(t, e, ErrStale)
		require.Nil(t, got)
	}
}
