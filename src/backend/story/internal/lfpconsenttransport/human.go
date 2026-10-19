package lfpconsenttransport

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
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
)

// HumanAction is minted only by exact network verification, not raw metadata or
// a general principal context. It does not establish offer/cohort eligibility.
type HumanAction struct {
	identity      principal.UserIdentity
	requestSHA256 [32]byte
	actor         lfpconsent.Actor
	expiry        time.Time
}

func (a HumanAction) Identity() principal.UserIdentity { return a.identity }
func (a HumanAction) RequestSHA256() [32]byte          { return a.requestSHA256 }

// CredentialExpiry is an in-flight bound, never stored as delayed authority.
func (a HumanAction) CredentialExpiry() time.Time { return a.expiry }
func (a HumanAction) ActorMaterial() (lfpconsent.Actor, error) {
	if _, _, e := lfpconsent.EncodeActor(a.actor); e != nil {
		return lfpconsent.Actor{}, e
	}
	return a.actor, nil
}
func (HumanAction) String() string   { return "verified LFP human action" }
func (HumanAction) GoString() string { return "verified LFP human action" }

type HumanVerifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Epochs principal.SessionEpochChecker
	Clock  func() time.Time
}

func (v *HumanVerifier) Verify(ctx context.Context, r proto.Message, method string) (HumanAction, error) {
	fail := func(code codes.Code) (HumanAction, error) {
		return HumanAction{}, status.Error(code, "LFP action unavailable")
	}
	if v == nil || v.Keys == nil || v.Replay == nil || v.Epochs == nil {
		return fail(codes.Unavailable)
	}
	if !actionRequest(r, method) {
		return fail(codes.Unauthenticated)
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
	hash, e := principal.RequestHash(r)
	if e != nil {
		return fail(codes.InvalidArgument)
	}
	dependency := errors.New("LFP action authority unavailable")
	p, e := principal.VerifyDelegatedUser(ctx, md.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "story", ExpectedRPC: method, ExpectedRequestID: md.RequestID, ExpectedRequestHash: hash, Clock: v.Clock,
		KeyResolver: func(c context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			key, e := v.Keys(c, issuer, kid)
			if e != nil {
				return nil, dependency
			}
			return key, nil
		},
		ReplayGuard: func(c context.Context, issuer, jti string, expiry time.Time) error {
			if e := v.Replay(c, issuer, jti, expiry); e != nil {
				return dependency
			}
			return nil
		},
		SessionEpochChecker: func(c context.Context, account string, epoch int64) error {
			if e := v.Epochs(c, account, epoch); e != nil {
				return dependency
			}
			return nil
		},
	})
	if e != nil {
		if errors.Is(e, dependency) {
			return fail(codes.Unavailable)
		}
		return fail(codes.Unauthenticated)
	}
	// Live epoch/replay/key dependencies may have waited beyond token expiry.
	// Verification at ingress alone cannot authorize a later durable capture.
	now := time.Now()
	if v.Clock != nil {
		now = v.Clock()
	}
	if !p.ExpiresAt.After(now) {
		return fail(codes.Unauthenticated)
	}
	identity, e := principal.RequireDelegatedUser(principal.WithVerified(ctx, p), "story", method, r)
	if e != nil {
		return fail(codes.Unauthenticated)
	}
	// Preserve the verifier's explicit type. SDK, bot/service and missing type
	// cannot be promoted to human/regular by an ID, floor or default context.
	if identity.AccountType != "regular" && identity.AccountType != "guest" {
		return fail(codes.PermissionDenied)
	}
	b, e := proto.MarshalOptions{Deterministic: true}.Marshal(r)
	if e != nil {
		return fail(codes.InvalidArgument)
	}
	action := HumanAction{identity: identity, requestSHA256: sha256.Sum256(b), expiry: p.ExpiresAt}
	var operation []byte
	var kind uint64
	switch request := r.(type) {
	case *storyv1.PublishLfpConsentOfferRequest:
		operation, kind = request.PublicationOperationId, lfpconsent.PublishOffer
	case *storyv1.RespondWithLfpConsentRequest:
		operation, kind = request.ResponseOperationId, lfpconsent.Respond
	default: // Observe creates no persistent actor provenance.
		return action, nil
	}
	op, e := uuid.FromBytes(operation)
	if e != nil || op == uuid.Nil {
		return fail(codes.InvalidArgument)
	}
	accountType := uint64(lfpconsent.Regular)
	if identity.AccountType == "guest" {
		accountType = lfpconsent.Guest
	}
	action.actor = lfpconsent.Actor{ActionKind: kind, ActorKind: lfpconsent.HumanDelegatedUser, AccountType: accountType, AccountID: identity.AccountID, ProfileID: identity.ProfileID, SessionEpoch: identity.SessionEpoch, OperationID: op, Issuer: p.Issuer, Audience: p.Audience, FullRPC: method, PrincipalRequestSHA256: action.requestSHA256}
	if _, e := action.ActorMaterial(); e != nil {
		return fail(codes.Unauthenticated)
	}
	return action, nil
}

func actionRequest(r proto.Message, method string) bool {
	if r == nil || !r.ProtoReflect().IsValid() || len(r.ProtoReflect().GetUnknown()) != 0 {
		return false
	}
	switch method {
	case storyv1.LfpConsentActionService_PublishLfpConsentOffer_FullMethodName:
		_, ok := r.(*storyv1.PublishLfpConsentOfferRequest)
		return ok
	case storyv1.LfpConsentActionService_ObserveLfpConsentOffer_FullMethodName:
		_, ok := r.(*storyv1.ObserveLfpConsentOfferRequest)
		return ok
	case storyv1.LfpConsentActionService_RespondWithLfpConsent_FullMethodName:
		request, ok := r.(*storyv1.RespondWithLfpConsentRequest)
		return ok && request.Offer != nil && len(request.Offer.ProtoReflect().GetUnknown()) == 0
	default:
		return false
	}
}
