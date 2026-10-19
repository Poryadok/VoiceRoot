package lfpconsenttransport

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/storylfpadmission"
	"voice/backend/story/internal/lfpconsentstore"
)

type ActorOfferJournal interface {
	ReadActorConsentContext(context.Context, uuid.UUID, [32]byte, storylfpadmission.Origin) (lfpconsent.Envelope, lfpconsentstore.OfferContext, error)
}

// ActorResponderEligibility MUST evaluate current privacy rules for the saved
// original explicit human viewer and THIS immutable audience. An implementation
// that looks up current account type or defaults a service context is invalid.
type ActorResponderEligibility interface {
	CheckActorResponder(context.Context, lfpconsentstore.OfferContext, lfpconsent.Actor) error
}

type ActorOfferConfig struct {
	Origin        storylfpadmission.Origin
	PeerAuthority storylfpadmission.AuthenticatedPeerAuthority
	Keys          principal.KeyResolver
	Replay        principal.ReplayGuard
	Clock         func() time.Time
	Eligibility   ActorResponderEligibility
}

type ActorOfferServer struct {
	storyv1.UnimplementedLfpActorBoundConsentOfferServiceServer
	Journal ActorOfferJournal
	Config  ActorOfferConfig
}

func (s *ActorOfferServer) ValidateActorBoundLfpConsentOffer(ctx context.Context, r *storyv1.ValidateActorBoundLfpConsentOfferRequest) (*storyv1.ValidateActorBoundLfpConsentOfferResponse, error) {
	if s == nil || s.Journal == nil || s.Config.Eligibility == nil {
		return nil, actorOfferError(lfpconsentstore.ErrUnavailable)
	}
	if r == nil || r.ProtocolVersion != 1 || len(r.ProtoReflect().GetUnknown()) != 0 || r.Offer == nil || len(r.Offer.ProtoReflect().GetUnknown()) != 0 || !nonzero(r.QualifiedConsentSha256, 32) || !nonzero(r.ActorSha256, 32) {
		return nil, status.Error(codes.InvalidArgument, "invalid LFP validation request")
	}
	o, e := readOrigin(r.Origin)
	if e != nil || o != s.Config.Origin {
		return nil, status.Error(codes.FailedPrecondition, "LFP origin ineligible")
	}
	v := r.Offer
	if v.ProtocolVersion != 1 || !nonzero(v.ConsentId, 16) || !nonzero(v.StoryId, 16) || !nonzero(v.StoryIncarnation, 16) || v.OfferRevision == 0 || !nonzero(v.ConsentSha256, 32) || !nonzero(v.OfferPayloadSha256, 32) || !nonzero(v.CriteriaSha256, 32) {
		return nil, status.Error(codes.InvalidArgument, "invalid LFP validation request")
	}
	verified, expiry, e := verifyActorService(ctx, r, storyv1.LfpActorBoundConsentOfferService_ValidateActorBoundLfpConsentOffer_FullMethodName, s.Config)
	if e != nil {
		return nil, e
	}
	id, qualified := uuid.UUID(v.ConsentId), [32]byte(r.QualifiedConsentSha256)
	envelope, bound, e := s.Journal.ReadActorConsentContext(ctx, id, qualified, verified.Origin())
	if e != nil {
		return nil, actorOfferError(e)
	}
	if !actorValidationMatches(r, envelope, bound) {
		return nil, status.Error(codes.FailedPrecondition, "LFP offer ineligible")
	}
	// No journal lock spans remote dependencies. The original actor snapshot is
	// explicit even for guest conversion/replay; no current-ID type lookup.
	e = s.Config.Eligibility.CheckActorResponder(ctx, bound, envelope.Actor())
	if errors.Is(e, ErrResponderIneligible) {
		return nil, status.Error(codes.FailedPrecondition, "LFP offer ineligible")
	}
	if e != nil || !expiry.After(actorNow(s.Config.Clock)) {
		return nil, actorOfferError(lfpconsentstore.ErrUnavailable)
	}
	final, current, e := s.Journal.ReadActorConsentContext(ctx, id, qualified, verified.Origin())
	if e != nil {
		return nil, actorOfferError(e)
	}
	if !actorValidationMatches(r, final, current) || current.Offer() != bound.Offer() || !expiry.After(actorNow(s.Config.Clock)) {
		return nil, status.Error(codes.FailedPrecondition, "LFP offer ineligible")
	}
	return &storyv1.ValidateActorBoundLfpConsentOfferResponse{ProtocolVersion: 1, Offer: &storyv1.ValidateLfpConsentOfferResponse{ProtocolVersion: 1, ConsentId: bytes.Clone(v.ConsentId), ConsentSha256: bytes.Clone(v.ConsentSha256), StoryId: bytes.Clone(v.StoryId), StoryIncarnation: bytes.Clone(v.StoryIncarnation), OfferRevision: v.OfferRevision, OfferPayloadSha256: bytes.Clone(v.OfferPayloadSha256), CriteriaSha256: bytes.Clone(v.CriteriaSha256), OfferExpiresAtUnixMillis: current.Offer().ExpiresAtUnixMillis}, QualifiedConsentSha256: bytes.Clone(r.QualifiedConsentSha256), ActorSha256: bytes.Clone(r.ActorSha256), Origin: wireOrigin(verified.Origin()), CanonicalCriteriaJson: []byte(current.CriteriaJSON())}, nil
}

func actorValidationMatches(r *storyv1.ValidateActorBoundLfpConsentOfferRequest, envelope lfpconsent.Envelope, bound lfpconsentstore.OfferContext) bool {
	if !envelope.Valid() || envelope.Origin() == (storylfpadmission.Origin{}) {
		return false
	}
	v, c, o := r.Offer, envelope.Consent(), bound.Offer()
	return envelope.QualifiedSHA256() == [32]byte(r.QualifiedConsentSha256) && envelope.ActorSHA256() == [32]byte(r.ActorSha256) && envelope.ConsentSHA256() == [32]byte(v.ConsentSha256) && c.ConsentID == uuid.UUID(v.ConsentId) && c.StoryID == uuid.UUID(v.StoryId) && c.StoryIncarnation == uuid.UUID(v.StoryIncarnation) && c.OfferRevision == v.OfferRevision && c.OfferPayloadSHA256 == [32]byte(v.OfferPayloadSha256) && c.CriteriaSHA256 == [32]byte(v.CriteriaSha256) && o.StoryID == c.StoryID && o.Incarnation == c.StoryIncarnation && o.Revision == c.OfferRevision && o.AuthorProfileID == c.AuthorProfileID && o.CriteriaSHA256 == c.CriteriaSHA256 && o.PayloadSHA256 == c.OfferPayloadSHA256 && o.ExpiresAtUnixMillis == c.OfferExpiresAtUnixMillis && criteriaMatches([]byte(bound.CriteriaJSON()), c.CriteriaSHA256)
}

func criteriaMatches(b []byte, expected [32]byte) bool {
	if len(b) == 0 || len(b) > 65536 || sha256.Sum256(b) != expected {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(b, &object) == nil && object != nil
}

func actorOfferError(e error) error {
	if errors.Is(e, lfpconsentstore.ErrStale) || errors.Is(e, lfpconsentstore.ErrConflict) || errors.Is(e, storylfpadmission.ErrIneligible) {
		return status.Error(codes.FailedPrecondition, "LFP offer ineligible")
	}
	return status.Error(codes.Unavailable, "LFP offer validation unavailable")
}

func actorNow(clock func() time.Time) time.Time {
	if clock != nil {
		return clock()
	}
	return time.Now()
}

func verifyActorService(ctx context.Context, r proto.Message, method string, cfg ActorOfferConfig) (storylfpadmission.VerifiedPeer, time.Time, error) {
	fail := func(code codes.Code) (storylfpadmission.VerifiedPeer, time.Time, error) {
		return storylfpadmission.VerifiedPeer{}, time.Time{}, status.Error(code, "LFP validation authority unavailable")
	}
	if !cfg.Origin.Valid() || cfg.PeerAuthority == nil || cfg.Keys == nil || cfg.Replay == nil {
		return fail(codes.Unavailable)
	}
	endpoint, ok := peer.FromContext(ctx)
	if !ok {
		return fail(codes.Unauthenticated)
	}
	tlsInfo, ok := endpoint.AuthInfo.(credentials.TLSInfo)
	if !ok || !tlsInfo.State.HandshakeComplete || len(tlsInfo.State.VerifiedChains) == 0 {
		return fail(codes.Unauthenticated)
	}
	md, e := principal.IncomingMetadata(ctx)
	if e != nil {
		return fail(codes.Unauthenticated)
	}
	h, e := principal.RequestHash(r)
	if e != nil {
		return fail(codes.InvalidArgument)
	}
	dependency := errors.New("LFP verifier dependency unavailable")
	p, e := principal.VerifyService(ctx, md.BearerToken, principal.VerifyConfig{ExpectedIssuer: "matchmaking", ExpectedAudience: "story", ExpectedRPC: method, ExpectedRequestID: md.RequestID, ExpectedRequestHash: h, Clock: cfg.Clock,
		KeyResolver: func(c context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			key, e := cfg.Keys(c, issuer, kid)
			if e != nil {
				return nil, dependency
			}
			return key, nil
		}, ReplayGuard: func(c context.Context, issuer, jti string, expiry time.Time) error {
			if e := cfg.Replay(c, issuer, jti, expiry); e != nil {
				return dependency
			}
			return nil
		}})
	if e != nil {
		if errors.Is(e, dependency) {
			return fail(codes.Unavailable)
		}
		return fail(codes.Unauthenticated)
	}
	verified, e := storylfpadmission.ResolvePeer(principal.WithVerified(ctx, p), cfg.Origin, cfg.PeerAuthority)
	if e != nil {
		return storylfpadmission.VerifiedPeer{}, time.Time{}, actorOfferError(e)
	}
	if !p.ExpiresAt.After(actorNow(cfg.Clock)) {
		return fail(codes.Unauthenticated)
	}
	return verified, p.ExpiresAt, nil
}

func readOrigin(p *storyv1.LfpConsentOrigin) (storylfpadmission.Origin, error) {
	if p == nil || len(p.ProtoReflect().GetUnknown()) != 0 || !nonzero(p.EnvironmentId, 16) || !nonzero(p.InstallationId, 16) || !nonzero(p.AuthorityIncarnation, 16) || !nonzero(p.DatabaseBindingSha256, 32) {
		return storylfpadmission.Origin{}, lfpconsent.ErrInvalid
	}
	return storylfpadmission.NewOrigin([16]byte(p.EnvironmentId), [16]byte(p.InstallationId), [16]byte(p.AuthorityIncarnation), [32]byte(p.DatabaseBindingSha256))
}

func wireOrigin(o storylfpadmission.Origin) *storyv1.LfpConsentOrigin {
	env, install, incarnation, db := o.Environment(), o.Installation(), o.Incarnation(), o.DatabaseBinding()
	return &storyv1.LfpConsentOrigin{EnvironmentId: bytes.Clone(env[:]), InstallationId: bytes.Clone(install[:]), AuthorityIncarnation: bytes.Clone(incarnation[:]), DatabaseBindingSha256: bytes.Clone(db[:])}
}
