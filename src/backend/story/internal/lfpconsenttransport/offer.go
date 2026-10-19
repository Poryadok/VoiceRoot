// Package lfpconsenttransport provides private qualified consent RPCs. It does
// not install listeners, credentials, trust manifests or restore authority.
package lfpconsenttransport

import (
	"bytes"
	"context"
	"crypto/rsa"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/storylfpadmission"
	"voice/backend/story/internal/lfpconsentstore"
)

type OfferJournal interface {
	ReadConsentForDelivery(context.Context, uuid.UUID, [32]byte, storylfpadmission.Origin) ([]byte, error)
	ValidateConsent(context.Context, lfpconsentstore.Validation, storylfpadmission.VerifiedPeer) (lfpconsentstore.Offer, error)
}

type OfferConfig struct {
	Origin        storylfpadmission.Origin
	PeerAuthority storylfpadmission.AuthenticatedPeerAuthority
	Keys          principal.KeyResolver
	Replay        principal.ReplayGuard
	Clock         func() time.Time
	Eligibility   ResponderEligibility
}

var ErrResponderIneligible = errors.New("LFP responder ineligible")

// ResponderEligibility supplies fresh Story visibility/privacy and supported
// offer checks. The installation root must bind its actual implementation.
type ResponderEligibility interface {
	CheckResponder(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}

type OfferServer struct {
	storyv1.UnimplementedLfpConsentOfferServiceServer
	Journal OfferJournal
	Config  OfferConfig
}

func (s *OfferServer) ValidateLfpConsentOffer(ctx context.Context, r *storyv1.ValidateLfpConsentOfferRequest) (*storyv1.ValidateLfpConsentOfferResponse, error) {
	unavailable := func() (*storyv1.ValidateLfpConsentOfferResponse, error) {
		return nil, status.Error(codes.Unavailable, "LFP offer validation unavailable")
	}
	if s == nil || s.Journal == nil || !s.Config.Origin.Valid() || s.Config.PeerAuthority == nil || s.Config.Keys == nil || s.Config.Replay == nil || s.Config.Eligibility == nil {
		return unavailable()
	}
	if r == nil || len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || !nonzero(r.ConsentId, 16) || !nonzero(r.StoryId, 16) || !nonzero(r.StoryIncarnation, 16) || r.OfferRevision == 0 || !nonzero(r.ConsentSha256, 32) || !nonzero(r.OfferPayloadSha256, 32) || !nonzero(r.CriteriaSha256, 32) {
		return nil, status.Error(codes.InvalidArgument, "invalid LFP validation request")
	}
	tlsPeer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "protected LFP credential required")
	}
	tlsInfo, ok := tlsPeer.AuthInfo.(credentials.TLSInfo)
	if !ok || !tlsInfo.State.HandshakeComplete || len(tlsInfo.State.VerifiedChains) == 0 {
		return nil, status.Error(codes.Unauthenticated, "protected LFP credential required")
	}
	md, e := principal.IncomingMetadata(ctx)
	if e != nil {
		return nil, status.Error(codes.Unauthenticated, "protected LFP credential required")
	}
	hash, e := principal.RequestHash(r)
	if e != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid LFP validation request")
	}
	// Preserve dependency outages as Unavailable without exposing trust, key,
	// replay-store or principal detail. Replay denial also remains fail-closed.
	dependency := errors.New("LFP verifier dependency unavailable")
	p, e := principal.VerifyService(ctx, md.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: "matchmaking", ExpectedAudience: "story", ExpectedRPC: storyv1.LfpConsentOfferService_ValidateLfpConsentOffer_FullMethodName,
		ExpectedRequestID: md.RequestID, ExpectedRequestHash: hash, Clock: s.Config.Clock,
		KeyResolver: func(c context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			k, e := s.Config.Keys(c, issuer, kid)
			if e != nil {
				return nil, dependency
			}
			return k, nil
		},
		ReplayGuard: func(c context.Context, issuer, jti string, expiry time.Time) error {
			if e := s.Config.Replay(c, issuer, jti, expiry); e != nil {
				return dependency
			}
			return nil
		},
	})
	if e != nil {
		if errors.Is(e, dependency) {
			return unavailable()
		}
		return nil, status.Error(codes.Unauthenticated, "protected LFP credential required")
	}
	// The authority resolves actual TLS+verified credential against admitted
	// trust. WithVerified alone does not authenticate a cohort or responding peer.
	verified, e := storylfpadmission.ResolvePeer(principal.WithVerified(ctx, p), s.Config.Origin, s.Config.PeerAuthority)
	if errors.Is(e, storylfpadmission.ErrIneligible) {
		return nil, status.Error(codes.FailedPrecondition, "LFP origin ineligible")
	}
	if e != nil {
		return unavailable()
	}
	v := lfpconsentstore.Validation{ConsentID: uuid.UUID(r.ConsentId), StoryID: uuid.UUID(r.StoryId), Incarnation: uuid.UUID(r.StoryIncarnation), Revision: r.OfferRevision, ConsentSHA256: [32]byte(r.ConsentSha256), PayloadSHA256: [32]byte(r.OfferPayloadSha256), CriteriaSHA256: [32]byte(r.CriteriaSha256)}
	canonical, e := s.Journal.ReadConsentForDelivery(ctx, v.ConsentID, v.ConsentSHA256, verified.Origin())
	if e != nil {
		return unavailable()
	}
	c, ch, e := lfpconsent.Decode(canonical)
	if e != nil || ch != v.ConsentSHA256 || c.ConsentID != v.ConsentID || c.StoryID != v.StoryID || c.StoryIncarnation != v.Incarnation || c.OfferRevision != v.Revision || c.CriteriaSHA256 != v.CriteriaSHA256 || c.OfferPayloadSHA256 != v.PayloadSHA256 {
		return nil, status.Error(codes.FailedPrecondition, "LFP offer ineligible")
	}
	// Dependencies run without journal locks. The following admitted transaction
	// rechecks exact Story state after this fresh privacy observation.
	e = s.Config.Eligibility.CheckResponder(ctx, c.StoryID, c.AuthorProfileID, c.ResponderProfileID)
	if errors.Is(e, ErrResponderIneligible) {
		return nil, status.Error(codes.FailedPrecondition, "LFP offer ineligible")
	}
	if e != nil {
		return unavailable()
	}
	o, e := s.Journal.ValidateConsent(ctx, v, verified)
	if errors.Is(e, lfpconsentstore.ErrStale) || errors.Is(e, lfpconsentstore.ErrConflict) {
		return nil, status.Error(codes.FailedPrecondition, "LFP offer ineligible")
	}
	if e != nil || o.StoryID != v.StoryID || o.Incarnation != v.Incarnation || o.Revision != v.Revision || o.PayloadSHA256 != v.PayloadSHA256 || o.CriteriaSHA256 != v.CriteriaSHA256 || o.ExpiresAtUnixMillis <= 0 {
		return unavailable()
	}
	return &storyv1.ValidateLfpConsentOfferResponse{ProtocolVersion: 1, ConsentId: bytes.Clone(r.ConsentId), ConsentSha256: bytes.Clone(r.ConsentSha256), StoryId: bytes.Clone(r.StoryId), StoryIncarnation: bytes.Clone(r.StoryIncarnation), OfferRevision: r.OfferRevision, OfferPayloadSha256: bytes.Clone(r.OfferPayloadSha256), CriteriaSha256: bytes.Clone(r.CriteriaSha256), OfferExpiresAtUnixMillis: o.ExpiresAtUnixMillis}, nil
}

func nonzero(b []byte, n int) bool { return len(b) == n && !bytes.Equal(b, make([]byte, n)) }
