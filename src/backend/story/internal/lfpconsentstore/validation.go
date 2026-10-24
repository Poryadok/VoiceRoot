package lfpconsentstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/storylfpadmission"
)

type Validation struct {
	ConsentID, StoryID, Incarnation              uuid.UUID
	Revision                                     uint64
	ConsentSHA256, PayloadSHA256, CriteriaSHA256 [32]byte
}

func (s *Store) ValidateConsent(ctx context.Context, v Validation, peer storylfpadmission.VerifiedPeer) (Offer, error) {
	if !peer.Valid() || v.ConsentID == uuid.Nil || v.StoryID == uuid.Nil || v.Incarnation == uuid.Nil || v.Revision == 0 || v.ConsentSHA256 == [32]byte{} || v.PayloadSHA256 == [32]byte{} || v.CriteriaSHA256 == [32]byte{} {
		return Offer{}, ErrUnavailable
	}
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return Offer{}, e
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	p := peer.Origin()
	if !sameOrigin(o, p) {
		return Offer{}, ErrStale
	}
	c, _, h, saved, e := readConsent(ctx, tx, v.ConsentID, true)
	if errors.Is(e, pgx.ErrNoRows) {
		return Offer{}, ErrStale
	}
	if e != nil {
		return Offer{}, ErrUnavailable
	}
	if c.StoryID != v.StoryID || c.StoryIncarnation != v.Incarnation || c.OfferRevision != v.Revision || h != v.ConsentSHA256 || c.OfferPayloadSHA256 != v.PayloadSHA256 || c.CriteriaSHA256 != v.CriteriaSHA256 {
		return Offer{}, ErrConflict
	}
	if saved.provenance != o {
		return Offer{}, ErrStale
	}
	state, e := lockStory(ctx, tx, c.StoryID)
	if e != nil {
		return Offer{}, e
	}
	current, e := lastOffer(ctx, tx, c.StoryID)
	if errors.Is(e, pgx.ErrNoRows) {
		return Offer{}, ErrStale
	}
	if e != nil {
		return Offer{}, ErrUnavailable
	}
	if current != saved || current.StateSHA256 != state.StateSHA256 {
		return Offer{}, ErrStale
	}
	if e = tx.Commit(ctx); e != nil {
		return Offer{}, ErrUnavailable
	}
	return saved, nil
}
