package lfpconsentstore

import (
	"bytes"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/storylfpadmission"
)

func (s *Store) PublishActorOffer(ctx context.Context, a AuthorAction, actor lfpconsent.Actor, expected storylfpadmission.Origin, credentialExpiry time.Time) (Offer, error) {
	capture, e := newActorCapture(actor, expected, credentialExpiry)
	if e != nil {
		return Offer{}, e
	}
	if actor.ActionKind != lfpconsent.PublishOffer || actor.OperationID != a.OperationID || actor.ProfileID != a.ProfileID || actor.PrincipalRequestSHA256 != a.RequestSHA256 {
		return Offer{}, ErrConflict
	}
	return s.publishOffer(ctx, a, capture)
}
func (s *Store) RecordActorResponse(ctx context.Context, r Response, actor lfpconsent.Actor, expected storylfpadmission.Origin, credentialExpiry time.Time) (lfpconsent.Envelope, error) {
	capture, e := newActorCapture(actor, expected, credentialExpiry)
	if e != nil {
		return lfpconsent.Envelope{}, e
	}
	if actor.ActionKind != lfpconsent.Respond || actor.OperationID != r.OperationID || actor.AccountID != r.AccountID || actor.ProfileID != r.ProfileID || actor.SessionEpoch != r.SessionEpoch || actor.PrincipalRequestSHA256 != r.RequestSHA256 {
		return lfpconsent.Envelope{}, ErrConflict
	}
	c, _, _, e := s.recordResponse(ctx, r, capture)
	if e != nil {
		return lfpconsent.Envelope{}, e
	}
	// Re-read the ORIGINAL immutable sidecar after replay, never rebuild it from
	// today's type/epoch. A cohort change between commits cannot qualify success.
	return s.readCurrentActorConsent(ctx, c.ConsentID, expected)
}
func (s *Store) ReadActorConsentForDelivery(ctx context.Context, id uuid.UUID, qualified [32]byte, expected storylfpadmission.Origin) (lfpconsent.Envelope, error) {
	if qualified == [32]byte{} {
		return lfpconsent.Envelope{}, ErrUnavailable
	}
	envelope, e := s.readCurrentActorConsent(ctx, id, expected)
	if e != nil {
		return lfpconsent.Envelope{}, e
	}
	if envelope.QualifiedSHA256() != qualified {
		return lfpconsent.Envelope{}, ErrConflict
	}
	return envelope, nil
}

type ActorRegistration struct {
	Registration                 lfpconsent.Registration
	Origin                       storylfpadmission.Origin
	QualifiedSHA256, BoundSHA256 [32]byte
}

func (s *Store) SaveActorRegistration(ctx context.Context, ack ActorRegistration) error {
	b, h, e := lfpconsent.EncodeRegistration(ack.Registration)
	if e != nil || !ack.Origin.Valid() {
		return ErrUnavailable
	}
	bound, e := lfpconsent.HashActorBoundRegistration(h, ack.QualifiedSHA256)
	if e != nil || bound != ack.BoundSHA256 {
		return ErrUnavailable
	}
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return e
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	if !sameOrigin(o, ack.Origin) {
		return ErrStale
	}
	envelope, e := readActorEnvelope(ctx, tx, ack.Registration.ConsentID)
	if e != nil {
		return e
	}
	if envelope.Origin() != ack.Origin || envelope.QualifiedSHA256() != ack.QualifiedSHA256 || envelope.ConsentSHA256() != ack.Registration.ConsentSHA256 || ack.Registration.OperationID != ack.Registration.ConsentID {
		return ErrConflict
	}
	_, e = tx.Exec(ctx, `INSERT INTO story_lfp_registration_acks(consent_id,canonical_registration,registration_sha256) VALUES($1,$2,$3) ON CONFLICT(consent_id) DO NOTHING`, ack.Registration.ConsentID, b, h[:])
	if e != nil {
		return ErrUnavailable
	}
	var saved, hash []byte
	e = tx.QueryRow(ctx, `SELECT canonical_registration,registration_sha256 FROM story_lfp_registration_acks WHERE consent_id=$1`, ack.Registration.ConsentID).Scan(&saved, &hash)
	if e != nil {
		return ErrUnavailable
	}
	if !bytes.Equal(saved, b) || !bytes.Equal(hash, h[:]) {
		return ErrConflict
	}
	_, e = tx.Exec(ctx, `INSERT INTO story_lfp_actor_bound_acks(consent_id,qualified_consent_sha256,actor_bound_registration_sha256) VALUES($1,$2,$3) ON CONFLICT(consent_id) DO NOTHING`, ack.Registration.ConsentID, ack.QualifiedSHA256[:], ack.BoundSHA256[:])
	if e != nil {
		return ErrUnavailable
	}
	var qualified, recorded []byte
	e = tx.QueryRow(ctx, `SELECT qualified_consent_sha256,actor_bound_registration_sha256 FROM story_lfp_actor_bound_acks WHERE consent_id=$1`, ack.Registration.ConsentID).Scan(&qualified, &recorded)
	if e != nil {
		return ErrUnavailable
	}
	if !bytes.Equal(qualified, ack.QualifiedSHA256[:]) || !bytes.Equal(recorded, ack.BoundSHA256[:]) {
		return ErrConflict
	}
	if e = tx.Commit(ctx); e != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Store) ReadActorRegistration(ctx context.Context, id uuid.UUID, qualified [32]byte) (ActorRegistration, error) {
	if qualified == [32]byte{} {
		return ActorRegistration{}, ErrUnavailable
	}
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return ActorRegistration{}, e
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	envelope, e := readActorEnvelope(ctx, tx, id)
	if e != nil {
		return ActorRegistration{}, e
	}
	if !sameOrigin(o, envelope.Origin()) {
		return ActorRegistration{}, ErrStale
	}
	if envelope.QualifiedSHA256() != qualified {
		return ActorRegistration{}, ErrConflict
	}
	var b, hash, q, bound []byte
	e = tx.QueryRow(ctx, `SELECT r.canonical_registration,r.registration_sha256,a.qualified_consent_sha256,a.actor_bound_registration_sha256 FROM story_lfp_registration_acks r JOIN story_lfp_actor_bound_acks a USING(consent_id) WHERE consent_id=$1`, id).Scan(&b, &hash, &q, &bound)
	if e != nil {
		return ActorRegistration{}, ErrUnavailable
	}
	r, rh, e := lfpconsent.DecodeRegistration(b)
	if e != nil || !bytes.Equal(hash, rh[:]) || r.ConsentID != id || r.OperationID != id || r.ConsentSHA256 != envelope.ConsentSHA256() || !bytes.Equal(q, qualified[:]) {
		return ActorRegistration{}, ErrUnavailable
	}
	bh, e := lfpconsent.HashActorBoundRegistration(rh, qualified)
	if e != nil || !bytes.Equal(bound, bh[:]) {
		return ActorRegistration{}, ErrUnavailable
	}
	if e = tx.Commit(ctx); e != nil {
		return ActorRegistration{}, ErrUnavailable
	}
	return ActorRegistration{Registration: r, Origin: envelope.Origin(), QualifiedSHA256: qualified, BoundSHA256: bh}, nil
}

type actorCapture struct {
	actor     lfpconsent.Actor
	canonical []byte
	hash      [32]byte
	expected  storylfpadmission.Origin
	expiry    time.Time
}

func newActorCapture(a lfpconsent.Actor, expected storylfpadmission.Origin, expiry time.Time) (*actorCapture, error) {
	b, h, e := lfpconsent.EncodeActor(a)
	if e != nil || !expected.Valid() || expiry.IsZero() {
		return nil, ErrUnavailable
	}
	return &actorCapture{actor: a, canonical: b, hash: h, expected: expected, expiry: expiry}, nil
}

func (c *actorCapture) current(ctx context.Context, tx pgx.Tx) error {
	var now time.Time
	if e := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); e != nil || !c.expiry.After(now) {
		return ErrUnavailable
	}
	return nil
}

func sharedOrigin(o origin) (storylfpadmission.Origin, error) {
	return storylfpadmission.NewOrigin(o.environment, o.installation, o.incarnation, o.database)
}

func readPublicationActor(ctx context.Context, tx pgx.Tx, offer Offer) (lfpconsent.Actor, error) {
	a, _, e := readPublicationProof(ctx, tx, offer)
	return a, e
}

func readPublicationProof(ctx context.Context, tx pgx.Tx, offer Offer) (lfpconsent.Actor, OfferContext, error) {
	empty := func() (lfpconsent.Actor, OfferContext, error) {
		return lfpconsent.Actor{}, OfferContext{}, ErrUnavailable
	}
	rows, e := tx.Query(ctx, `SELECT p.operation_id,p.author_profile_id,p.request_sha256,a.canonical_actor,a.actor_sha256,a.canonical_offer_state FROM story_lfp_publications p JOIN story_lfp_publication_actors a ON a.publication_operation_id=p.operation_id WHERE p.story_id=$1 AND p.incarnation=$2 AND p.revision=$3 LIMIT 2 FOR SHARE OF p,a`, offer.StoryID, offer.Incarnation, int64(offer.Revision))
	if e != nil {
		return empty()
	}
	defer rows.Close()
	if !rows.Next() {
		return empty()
	}
	var op, author uuid.UUID
	var request, b, hash, snapshot []byte
	if e = rows.Scan(&op, &author, &request, &b, &hash, &snapshot); e != nil {
		return empty()
	}
	if rows.Next() || rows.Err() != nil {
		return empty()
	}
	a, h, e := lfpconsent.DecodeActor(b)
	if e != nil || !bytes.Equal(hash, h[:]) || a.ActionKind != lfpconsent.PublishOffer || a.OperationID != op || a.ProfileID != author || author != offer.AuthorProfileID || !bytes.Equal(request, a.PrincipalRequestSHA256[:]) {
		return empty()
	}
	bound, e := parseOfferContext(offer, snapshot)
	if e != nil {
		return empty()
	}
	return a, bound, nil
}

func readActorEnvelope(ctx context.Context, tx pgx.Tx, id uuid.UUID) (lfpconsent.Envelope, error) {
	_, b, _, offer, e := readConsent(ctx, tx, id, true)
	if e != nil {
		return lfpconsent.Envelope{}, ErrUnavailable
	}
	var actor, hash, qualified []byte
	e = tx.QueryRow(ctx, `SELECT canonical_actor,actor_sha256,qualified_consent_sha256 FROM story_lfp_consent_actors WHERE consent_id=$1 FOR SHARE`, id).Scan(&actor, &hash, &qualified)
	if e != nil {
		return lfpconsent.Envelope{}, ErrUnavailable
	}
	origin, e := sharedOrigin(offer.provenance)
	if e != nil {
		return lfpconsent.Envelope{}, ErrUnavailable
	}
	envelope, e := lfpconsent.NewEnvelope(b, actor, origin)
	ah, q := envelope.ActorSHA256(), envelope.QualifiedSHA256()
	if e != nil || !bytes.Equal(hash, ah[:]) || !bytes.Equal(qualified, q[:]) {
		return lfpconsent.Envelope{}, ErrUnavailable
	}
	return envelope, nil
}

func (s *Store) readCurrentActorConsent(ctx context.Context, id uuid.UUID, expected storylfpadmission.Origin) (lfpconsent.Envelope, error) {
	if id == uuid.Nil || !expected.Valid() {
		return lfpconsent.Envelope{}, ErrUnavailable
	}
	tx, o, release, e := s.begin(ctx)
	if e != nil {
		return lfpconsent.Envelope{}, e
	}
	defer release()
	defer func() { _ = tx.Rollback(ctx) }()
	if !sameOrigin(o, expected) {
		return lfpconsent.Envelope{}, ErrStale
	}
	envelope, e := readActorEnvelope(ctx, tx, id)
	if e != nil {
		return lfpconsent.Envelope{}, e
	}
	if envelope.Origin() != expected {
		return lfpconsent.Envelope{}, ErrStale
	}
	if e = tx.Commit(ctx); e != nil {
		return lfpconsent.Envelope{}, ErrUnavailable
	}
	return envelope, nil
}
