package lfpconsentstore

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/lfpconsent"
)

// Canonical bytes and EVERY relational selector must agree before following
// the immutable offer FK. Correct hashes cannot repair mismatched provenance.
func readConsent(ctx context.Context, tx pgx.Tx, id uuid.UUID, byConsentID bool) (lfpconsent.Consent, []byte, [32]byte, Offer, error) {
	empty := func(e error) (lfpconsent.Consent, []byte, [32]byte, Offer, error) {
		return lfpconsent.Consent{}, nil, [32]byte{}, Offer{}, e
	}
	predicate := "response_operation_id=$1"
	if byConsentID {
		predicate = "consent_id=$1"
	}
	var op, consent, story, inc, account, profile uuid.UUID
	var revision int64
	var b, hash []byte
	e := tx.QueryRow(ctx, `SELECT response_operation_id,consent_id,story_id,incarnation,offer_revision,responder_account_id,responder_profile_id,canonical_consent,consent_sha256 FROM story_lfp_consents WHERE `+predicate+` FOR SHARE`, id).Scan(&op, &consent, &story, &inc, &revision, &account, &profile, &b, &hash)
	if errors.Is(e, pgx.ErrNoRows) {
		return empty(pgx.ErrNoRows)
	}
	if e != nil {
		return empty(ErrUnavailable)
	}
	c, h, e := lfpconsent.Decode(b)
	if e != nil || !bytes.Equal(h[:], hash) || revision <= 0 || c.ResponseOperationID != op || c.ConsentID != consent || c.StoryID != story || c.StoryIncarnation != inc || c.OfferRevision != uint64(revision) || c.ResponderAccountID != account || c.ResponderProfileID != profile {
		return empty(ErrUnavailable)
	}
	offer, e := offerAt(ctx, tx, c.StoryID, c.StoryIncarnation, int64(c.OfferRevision))
	if e != nil || offer.AuthorProfileID != c.AuthorProfileID || offer.CriteriaSHA256 != c.CriteriaSHA256 || offer.PayloadSHA256 != c.OfferPayloadSHA256 || offer.ExpiresAtUnixMillis != c.OfferExpiresAtUnixMillis {
		return empty(ErrUnavailable)
	}
	return c, b, h, offer, nil
}
