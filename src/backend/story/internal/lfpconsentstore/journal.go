package lfpconsentstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/story/internal/lfperasure"
)

type Offer struct {
	StoryID, Incarnation, AuthorProfileID      uuid.UUID
	Revision                                   uint64
	CriteriaSHA256, PayloadSHA256, StateSHA256 [32]byte
	ExpiresAtUnixMillis                        int64
	provenance                                 origin
}

// AuthorAction is the exact publication intent supplied only by the protected
// verified author handler, not by an ordinary raw-profile Story route.
type AuthorAction struct {
	OperationID, StoryID, ProfileID uuid.UUID
	RequestSHA256                   [32]byte
}
type Response struct {
	OperationID, AccountID, ProfileID uuid.UUID
	SessionEpoch                      int64
	Type                              uint64
	RequestSHA256                     [32]byte
	Offer                             Offer
}

// PublishOffer requires a NEW verified author action for a restored cohort.
// It never relabels old rows. New operations advance revision even for A->A;
// exact action replay returns only its immutable original publication.
func (s *Store) PublishOffer(ctx context.Context, a AuthorAction) (Offer, error) {
	return s.publishOffer(ctx, a, nil)
}

func (s *Store) publishOffer(ctx context.Context, a AuthorAction, capture *actorCapture) (Offer, error) {
	if a.OperationID == uuid.Nil || a.StoryID == uuid.Nil || a.ProfileID == uuid.Nil || a.RequestSHA256 == [32]byte{} {
		return Offer{}, ErrUnavailable
	}
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return Offer{}, e
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	refs := lfperasure.References{Operations: []uuid.UUID{a.OperationID}, Profiles: []uuid.UUID{a.ProfileID}}
	if capture != nil {
		refs.Accounts = []uuid.UUID{capture.actor.AccountID}
	}
	if e = checkNegative(tx, refs); e != nil {
		return Offer{}, e
	}
	if capture != nil && !sameOrigin(o, capture.expected) {
		return Offer{}, ErrStale
	}
	_, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "story-lfp-publication:"+a.OperationID.String())
	if e != nil {
		return Offer{}, ErrUnavailable
	}
	var id, inc, author uuid.UUID
	var rev int64
	var request []byte
	e = tx.QueryRow(ctx, `SELECT story_id,incarnation,revision,author_profile_id,request_sha256 FROM story_lfp_publications WHERE operation_id=$1`, a.OperationID).Scan(&id, &inc, &rev, &author, &request)
	if e == nil {
		if id != a.StoryID || author != a.ProfileID || !bytes.Equal(request, a.RequestSHA256[:]) {
			return Offer{}, ErrConflict
		}
		saved, e := offerAt(ctx, tx, id, inc, rev)
		if e != nil {
			return Offer{}, ErrUnavailable
		}
		if saved.provenance != o {
			return Offer{}, ErrStale
		}
		if capture != nil {
			old, e := readPublicationActor(ctx, tx, saved)
			if e != nil {
				return Offer{}, e
			}
			if old.AccountID != capture.actor.AccountID || old.ProfileID != capture.actor.ProfileID {
				return Offer{}, ErrConflict
			}
			if e = capture.current(ctx, tx); e != nil {
				return Offer{}, e
			}
		}
		if e = tx.Commit(ctx); e != nil {
			return Offer{}, ErrUnavailable
		}
		return saved, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Offer{}, ErrUnavailable
	}
	state, e := lockStory(ctx, tx, a.StoryID)
	if e != nil {
		return Offer{}, e
	}
	if state.AuthorProfileID != a.ProfileID {
		return Offer{}, ErrConflict
	}
	latest, e := lastOffer(ctx, tx, a.StoryID)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return Offer{}, ErrUnavailable
	}
	state.Incarnation = uuid.New()
	state.Revision = 1
	state.provenance = o
	if e == nil && latest.provenance == o {
		if latest.Revision >= math.MaxInt64 {
			return Offer{}, ErrUnavailable
		}
		state.Incarnation = latest.Incarnation
		state.Revision = latest.Revision + 1
	}
	state.PayloadSHA256 = payloadHash(state)
	if capture != nil {
		if e = capture.current(ctx, tx); e != nil {
			return Offer{}, e
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO story_lfp_offers(story_id,incarnation,revision,author_profile_id,criteria_sha256,payload_sha256,state_sha256,expires_at_millis,environment_id,installation_id,authority_incarnation,database_binding_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, state.StoryID, state.Incarnation, int64(state.Revision), state.AuthorProfileID, state.CriteriaSHA256[:], state.PayloadSHA256[:], state.StateSHA256[:], state.ExpiresAtUnixMillis, o.environment, o.installation, o.incarnation, o.database[:])
	if e != nil {
		return Offer{}, ErrUnavailable
	}
	_, e = tx.Exec(ctx, `INSERT INTO story_lfp_offer_heads(story_id,incarnation,revision) VALUES($1,$2,$3) ON CONFLICT(story_id) DO UPDATE SET incarnation=EXCLUDED.incarnation,revision=EXCLUDED.revision`, state.StoryID, state.Incarnation, int64(state.Revision))
	if e != nil {
		return Offer{}, ErrUnavailable
	}
	_, e = tx.Exec(ctx, `INSERT INTO story_lfp_publications(operation_id,story_id,incarnation,revision,author_profile_id,request_sha256) VALUES($1,$2,$3,$4,$5,$6)`, a.OperationID, state.StoryID, state.Incarnation, int64(state.Revision), a.ProfileID, a.RequestSHA256[:])
	if e != nil {
		return Offer{}, ErrUnavailable
	}
	if capture != nil {
		snapshot, err := captureOfferState(ctx, tx, state)
		if err != nil {
			return Offer{}, err
		}
		_, e = tx.Exec(ctx, `INSERT INTO story_lfp_publication_actors(publication_operation_id,canonical_actor,actor_sha256,canonical_offer_state) VALUES($1,$2,$3,$4)`, a.OperationID, capture.canonical, capture.hash[:], snapshot)
		if e != nil {
			return Offer{}, ErrUnavailable
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return Offer{}, ErrUnavailable
	}
	return state, nil
}

// ObserveOffer cannot create a publication or adopt a legacy/restored row.
func (s *Store) ObserveOffer(ctx context.Context, id uuid.UUID) (Offer, error) {
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return Offer{}, e
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	state, e := lockStory(ctx, tx, id)
	if e != nil {
		return Offer{}, e
	}
	current, e := lastOffer(ctx, tx, id)
	if errors.Is(e, pgx.ErrNoRows) {
		return Offer{}, ErrStale
	}
	if e != nil {
		return Offer{}, ErrUnavailable
	}
	if current.provenance != o || current.StateSHA256 != state.StateSHA256 {
		return Offer{}, ErrStale
	}
	if e = tx.Commit(ctx); e != nil {
		return Offer{}, ErrUnavailable
	}
	return current, nil
}

func (s *Store) RecordResponse(ctx context.Context, r Response) (lfpconsent.Consent, []byte, [32]byte, error) {
	return s.recordResponse(ctx, r, nil)
}

func (s *Store) recordResponse(ctx context.Context, r Response, capture *actorCapture) (lfpconsent.Consent, []byte, [32]byte, error) {
	empty := func(e error) (lfpconsent.Consent, []byte, [32]byte, error) {
		return lfpconsent.Consent{}, nil, [32]byte{}, e
	}
	if r.OperationID == uuid.Nil || r.AccountID == uuid.Nil || r.ProfileID == uuid.Nil || r.SessionEpoch <= 0 || r.RequestSHA256 == [32]byte{} || (r.Type != lfpconsent.Join && r.Type != lfpconsent.Invite) {
		return empty(ErrUnavailable)
	}
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return empty(e)
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	if e = checkNegative(tx, responseReferences(r)); e != nil {
		return empty(e)
	}
	if capture != nil && !sameOrigin(o, capture.expected) {
		return empty(ErrStale)
	}
	_, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "story-lfp-response:"+r.OperationID.String())
	if e != nil {
		return empty(ErrUnavailable)
	}
	c, saved, h, offer, e := readConsent(ctx, tx, r.OperationID, false)
	if e == nil {
		if offer.provenance != o {
			return empty(ErrStale)
		}
		if !matches(c, r) {
			return empty(ErrConflict)
		}
		if capture != nil {
			if _, e = readActorEnvelope(ctx, tx, c.ConsentID); e != nil {
				return empty(e)
			}
			if e = capture.current(ctx, tx); e != nil {
				return empty(e)
			}
		}
		if e = tx.Commit(ctx); e != nil {
			return empty(ErrUnavailable)
		}
		return c, bytes.Clone(saved), h, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return empty(ErrUnavailable)
	}
	state, e := lockStory(ctx, tx, r.Offer.StoryID)
	if e != nil {
		return empty(e)
	}
	current, e := lastOffer(ctx, tx, r.Offer.StoryID)
	if e != nil {
		return empty(ErrUnavailable)
	}
	if !sameOfferView(current, r.Offer) || current.provenance != o || current.StateSHA256 != state.StateSHA256 || current.AuthorProfileID == r.ProfileID {
		return empty(ErrStale)
	}
	if capture != nil {
		if _, e = readPublicationActor(ctx, tx, current); e != nil {
			return empty(e)
		}
	}
	var now time.Time
	if e = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); e != nil {
		return empty(ErrUnavailable)
	}
	if now.UnixMilli() >= current.ExpiresAtUnixMillis {
		return empty(ErrStale)
	}
	if capture != nil && !capture.expiry.After(now) {
		return empty(ErrUnavailable)
	}
	c = lfpconsent.Consent{ConsentID: uuid.New(), ResponseOperationID: r.OperationID, StoryID: current.StoryID, StoryIncarnation: current.Incarnation, OfferRevision: current.Revision, AuthorProfileID: current.AuthorProfileID, ResponderAccountID: r.AccountID, ResponderProfileID: r.ProfileID, ResponderSessionEpoch: r.SessionEpoch, ResponseType: r.Type, CriteriaSHA256: current.CriteriaSHA256, OfferPayloadSHA256: current.PayloadSHA256, CreatedAtUnixMillis: now.UnixMilli(), OfferExpiresAtUnixMillis: current.ExpiresAtUnixMillis, PrincipalRequestSHA256: r.RequestSHA256}
	b, h, e := lfpconsent.Encode(c)
	if e != nil {
		return empty(ErrUnavailable)
	}
	_, e = tx.Exec(ctx, `INSERT INTO story_lfp_consents(response_operation_id,consent_id,story_id,incarnation,offer_revision,responder_account_id,responder_profile_id,canonical_consent,consent_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, r.OperationID, c.ConsentID, c.StoryID, c.StoryIncarnation, int64(c.OfferRevision), c.ResponderAccountID, c.ResponderProfileID, b, h[:])
	if e != nil {
		return empty(ErrUnavailable)
	}
	if capture != nil {
		origin, e := sharedOrigin(o)
		if e != nil {
			return empty(ErrUnavailable)
		}
		envelope, e := lfpconsent.NewEnvelope(b, capture.canonical, origin)
		if e != nil {
			return empty(ErrUnavailable)
		}
		qualified := envelope.QualifiedSHA256()
		_, e = tx.Exec(ctx, `INSERT INTO story_lfp_consent_actors(consent_id,canonical_actor,actor_sha256,qualified_consent_sha256) VALUES($1,$2,$3,$4)`, c.ConsentID, capture.canonical, capture.hash[:], qualified[:])
		if e != nil {
			return empty(ErrUnavailable)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return empty(ErrUnavailable)
	}
	return c, b, h, nil
}

func matches(c lfpconsent.Consent, r Response) bool {
	return c.ResponseOperationID == r.OperationID && c.ResponderAccountID == r.AccountID && c.ResponderProfileID == r.ProfileID && c.StoryID == r.Offer.StoryID && c.StoryIncarnation == r.Offer.Incarnation && c.OfferRevision == r.Offer.Revision && c.AuthorProfileID == r.Offer.AuthorProfileID && c.CriteriaSHA256 == r.Offer.CriteriaSHA256 && c.OfferPayloadSHA256 == r.Offer.PayloadSHA256 && c.OfferExpiresAtUnixMillis == r.Offer.ExpiresAtUnixMillis && c.ResponseType == r.Type && c.PrincipalRequestSHA256 == r.RequestSHA256
}

func sameOfferView(a, b Offer) bool {
	return a.StoryID == b.StoryID && a.Incarnation == b.Incarnation && a.AuthorProfileID == b.AuthorProfileID && a.Revision == b.Revision && a.CriteriaSHA256 == b.CriteriaSHA256 && a.PayloadSHA256 == b.PayloadSHA256 && a.ExpiresAtUnixMillis == b.ExpiresAtUnixMillis
}

func lockStory(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Offer, error) {
	var o Offer
	var criteria, visibility, audience string
	var expires, created time.Time
	var lfp, deleted, expired, hidden bool
	e := tx.QueryRow(ctx, `SELECT id,author_profile_id,COALESCE(lfp_criteria::text,''),visibility,COALESCE(visibility_audience::text,''),expires_at,created_at,is_looking_for_party,deleted_at IS NOT NULL,expired_at IS NOT NULL,hidden_from_feed_at IS NOT NULL FROM stories WHERE id=$1 FOR UPDATE`, id).Scan(&o.StoryID, &o.AuthorProfileID, &criteria, &visibility, &audience, &expires, &created, &lfp, &deleted, &expired, &hidden)
	if errors.Is(e, pgx.ErrNoRows) {
		return Offer{}, ErrStale
	}
	if e != nil {
		return Offer{}, ErrUnavailable
	}
	// AFTER LockRows has waited; SELECT target timestamps and now() can be old.
	var now time.Time
	if e = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); e != nil {
		return Offer{}, ErrUnavailable
	}
	if !lfp || deleted || expired || hidden || !expires.After(now) || len(criteria) == 0 || len(criteria) > 65536 || !json.Valid([]byte(criteria)) || criteria[0] != '{' {
		return Offer{}, ErrStale
	}
	o.CriteriaSHA256 = sha256.Sum256([]byte(criteria))
	o.ExpiresAtUnixMillis = expires.UnixMilli()
	b, e := canonicalStoryState(o.StoryID, o.AuthorProfileID, criteria, visibility, audience, expires, created)
	if e != nil {
		return Offer{}, ErrUnavailable
	}
	o.StateSHA256 = domainHash("voice.story.lfp-local-state.v1\x00", b)
	return o, nil
}

const offerColumns = `o.story_id,o.revision,o.incarnation,o.author_profile_id,o.criteria_sha256,o.payload_sha256,o.state_sha256,o.expires_at_millis,o.environment_id,o.installation_id,o.authority_incarnation,o.database_binding_sha256`

func lastOffer(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Offer, error) {
	o, e := scanOffer(tx.QueryRow(ctx, `SELECT `+offerColumns+` FROM story_lfp_offer_heads h JOIN story_lfp_offers o ON(o.story_id,o.incarnation,o.revision)=(h.story_id,h.incarnation,h.revision) WHERE h.story_id=$1`, id))
	if e != nil {
		return Offer{}, e
	}
	if e = checkNegative(tx, offerReferences(o)); e != nil {
		return Offer{}, e
	}
	return o, nil
}
func offerAt(ctx context.Context, tx pgx.Tx, id, inc uuid.UUID, rev int64) (Offer, error) {
	if rev <= 0 {
		return Offer{}, ErrUnavailable
	}
	if e := checkNegative(tx, lfperasure.References{Offers: []lfperasure.OfferReference{{Story: id, Incarnation: inc, Revision: uint64(rev)}}}); e != nil {
		return Offer{}, e
	}
	o, e := scanOffer(tx.QueryRow(ctx, `SELECT `+offerColumns+` FROM story_lfp_offers o WHERE o.story_id=$1 AND o.incarnation=$2 AND o.revision=$3`, id, inc, rev))
	if e != nil {
		return Offer{}, e
	}
	if e = checkNegative(tx, offerReferences(o)); e != nil {
		return Offer{}, e
	}
	return o, nil
}

func offerReferences(o Offer) lfperasure.References {
	return lfperasure.References{Profiles: []uuid.UUID{o.AuthorProfileID}, Offers: []lfperasure.OfferReference{{Story: o.StoryID, Incarnation: o.Incarnation, Revision: o.Revision}}}
}
func responseReferences(r Response) lfperasure.References {
	refs := offerReferences(r.Offer)
	refs.Accounts = []uuid.UUID{r.AccountID}
	refs.Profiles = append(refs.Profiles, r.ProfileID)
	refs.Operations = []uuid.UUID{r.OperationID}
	return refs
}
func scanOffer(row pgx.Row) (Offer, error) {
	var o Offer
	var revision int64
	var criteria, payload, state, db []byte
	e := row.Scan(&o.StoryID, &revision, &o.Incarnation, &o.AuthorProfileID, &criteria, &payload, &state, &o.ExpiresAtUnixMillis, &o.provenance.environment, &o.provenance.installation, &o.provenance.incarnation, &db)
	if e != nil {
		return Offer{}, e
	}
	if revision <= 0 || o.Incarnation == uuid.Nil || o.AuthorProfileID == uuid.Nil || len(criteria) != 32 || len(payload) != 32 || len(state) != 32 || len(db) != 32 || o.ExpiresAtUnixMillis <= 0 {
		return Offer{}, ErrUnavailable
	}
	o.Revision = uint64(revision)
	copy(o.CriteriaSHA256[:], criteria)
	copy(o.PayloadSHA256[:], payload)
	copy(o.StateSHA256[:], state)
	copy(o.provenance.database[:], db)
	if !o.provenance.valid() || o.PayloadSHA256 != payloadHash(o) {
		return Offer{}, ErrUnavailable
	}
	return o, nil
}
func payloadHash(o Offer) [32]byte {
	b := append([]byte{}, o.StoryID[:]...)
	b = append(b, o.Incarnation[:]...)
	b = binary.BigEndian.AppendUint64(b, o.Revision)
	b = append(b, o.StateSHA256[:]...)
	b = append(b, o.provenance.environment[:]...)
	b = append(b, o.provenance.installation[:]...)
	b = append(b, o.provenance.incarnation[:]...)
	b = append(b, o.provenance.database[:]...)
	return domainHash("voice.story.lfp-offer-payload.v1\x00", b)
}
func domainHash(domain string, b []byte) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write(b)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
