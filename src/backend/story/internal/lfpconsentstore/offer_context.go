package lfpconsentstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/storylfpadmission"
)

// OfferContext carries the exact immutable audience/criteria snapshot committed
// by this offer. Fresh remote privacy reads must evaluate THIS audience, rather
// than a different mutable Story row during an A->B->A race.
type OfferContext struct {
	offer                                  Offer
	criteriaJSON, visibility, audienceJSON string
}

func (c OfferContext) Offer() Offer         { return c.offer }
func (c OfferContext) CriteriaJSON() string { return c.criteriaJSON }
func (c OfferContext) Visibility() string   { return c.visibility }
func (c OfferContext) AudienceJSON() string { return c.audienceJSON }

func (s *Store) ReadActorOfferContext(ctx context.Context, id uuid.UUID, expected storylfpadmission.Origin) (OfferContext, error) {
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return OfferContext{}, e
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	if !sameOrigin(o, expected) {
		return OfferContext{}, ErrStale
	}
	bound, e := readCurrentOfferContext(ctx, tx, id, o)
	if e != nil {
		return OfferContext{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return OfferContext{}, ErrUnavailable
	}
	return bound, nil
}

func readCurrentOfferContext(ctx context.Context, tx pgx.Tx, id uuid.UUID, o origin) (OfferContext, error) {
	state, e := lockStory(ctx, tx, id)
	if e != nil {
		return OfferContext{}, e
	}
	current, e := lastOffer(ctx, tx, id)
	if e != nil {
		return OfferContext{}, ErrUnavailable
	}
	if current.StateSHA256 != state.StateSHA256 || current.provenance != o {
		return OfferContext{}, ErrStale
	}
	_, bound, e := readPublicationProof(ctx, tx, current)
	return bound, e
}

func canonicalStoryState(id, author uuid.UUID, criteria, visibility, audience string, expires, created time.Time) ([]byte, error) {
	return json.Marshal([]string{id.String(), author.String(), criteria, visibility, audience, expires.UTC().Format(time.RFC3339Nano), created.UTC().Format(time.RFC3339Nano)})
}

func captureOfferState(ctx context.Context, tx pgx.Tx, offer Offer) ([]byte, error) {
	// The publication already holds the Story row FOR UPDATE; no remote lookup
	// or second mutable audience source is introduced inside this transaction.
	var id, author uuid.UUID
	var criteria, visibility, audience string
	var expires, created time.Time
	e := tx.QueryRow(ctx, `SELECT id,author_profile_id,COALESCE(lfp_criteria::text,''),visibility,COALESCE(visibility_audience::text,''),expires_at,created_at FROM stories WHERE id=$1`, offer.StoryID).Scan(&id, &author, &criteria, &visibility, &audience, &expires, &created)
	if e != nil {
		return nil, ErrUnavailable
	}
	b, e := canonicalStoryState(id, author, criteria, visibility, audience, expires, created)
	if e != nil || len(b) > 1048576 || domainHash("voice.story.lfp-local-state.v1\x00", b) != offer.StateSHA256 {
		return nil, ErrUnavailable
	}
	return b, nil
}

func parseOfferContext(offer Offer, snapshot []byte) (OfferContext, error) {
	if len(snapshot) == 0 || len(snapshot) > 1048576 || domainHash("voice.story.lfp-local-state.v1\x00", snapshot) != offer.StateSHA256 {
		return OfferContext{}, ErrUnavailable
	}
	var fields []string
	if e := json.Unmarshal(snapshot, &fields); e != nil || len(fields) != 7 {
		return OfferContext{}, ErrUnavailable
	}
	canonical, e := json.Marshal(fields)
	if e != nil || !bytes.Equal(canonical, snapshot) || fields[0] != offer.StoryID.String() || fields[1] != offer.AuthorProfileID.String() || sha256.Sum256([]byte(fields[2])) != offer.CriteriaSHA256 {
		return OfferContext{}, ErrUnavailable
	}
	expires, e := time.Parse(time.RFC3339Nano, fields[5])
	if e != nil || expires.UnixMilli() != offer.ExpiresAtUnixMillis {
		return OfferContext{}, ErrUnavailable
	}
	return OfferContext{offer: offer, criteriaJSON: fields[2], visibility: fields[3], audienceJSON: fields[4]}, nil
}
