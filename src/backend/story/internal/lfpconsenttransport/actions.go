package lfpconsenttransport

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/storylfpadmission"
	"voice/backend/story/internal/lfpconsentstore"
)

type ActionJournal interface {
	PublishActorOffer(context.Context, lfpconsentstore.AuthorAction, lfpconsent.Actor, storylfpadmission.Origin, time.Time) (lfpconsentstore.Offer, error)
	ReadActorOfferContext(context.Context, uuid.UUID, storylfpadmission.Origin) (lfpconsentstore.OfferContext, error)
	FindActorResponse(context.Context, lfpconsentstore.Response, lfpconsent.Actor, storylfpadmission.Origin, time.Time) (lfpconsent.Envelope, bool, error)
	RecordActorResponse(context.Context, lfpconsentstore.Response, lfpconsent.Actor, storylfpadmission.Origin, time.Time) (lfpconsent.Envelope, error)
}
type QualifiedRegistrationDelivery interface {
	Deliver(context.Context, uuid.UUID, [32]byte) (lfpconsentstore.ActorRegistration, error)
}
type HumanOfferEligibility interface {
	ActorResponderEligibility
	CheckHumanOfferViewer(context.Context, lfpconsentstore.OfferContext, principal.UserIdentity) error
}

// ActionServer registers only on the protected delegated-user listener. The
// ordinary raw-profile StoryService handler cannot mint these durable records.
type ActionServer struct {
	storyv1.UnimplementedLfpConsentActionServiceServer
	Verifier    *HumanVerifier
	Origin      storylfpadmission.Origin
	Journal     ActionJournal
	Eligibility HumanOfferEligibility
	Registry    QualifiedRegistrationDelivery
}

func (s *ActionServer) ready() bool {
	return s != nil && s.Verifier != nil && s.Origin.Valid() && s.Journal != nil && s.Eligibility != nil && s.Registry != nil
}

func (s *ActionServer) PublishLfpConsentOffer(ctx context.Context, r *storyv1.PublishLfpConsentOfferRequest) (*storyv1.PublishLfpConsentOfferResponse, error) {
	if !s.ready() {
		return nil, actionError(lfpconsentstore.ErrUnavailable)
	}
	if r == nil || r.ProtocolVersion != 1 || !nonzero(r.PublicationOperationId, 16) || !nonzero(r.StoryId, 16) {
		return nil, invalidAction()
	}
	verified, e := s.Verifier.Verify(ctx, r, storyv1.LfpConsentActionService_PublishLfpConsentOffer_FullMethodName)
	if e != nil {
		return nil, e
	}
	actor, e := verified.ActorMaterial()
	if e != nil {
		return nil, actionError(e)
	}
	o, e := s.Journal.PublishActorOffer(ctx, lfpconsentstore.AuthorAction{OperationID: actor.OperationID, StoryID: uuid.UUID(r.StoryId), ProfileID: actor.ProfileID, RequestSHA256: verified.RequestSHA256()}, actor, s.Origin, verified.CredentialExpiry())
	if e != nil {
		return nil, actionError(e)
	}
	return &storyv1.PublishLfpConsentOfferResponse{Offer: wireOffer(o)}, nil
}

func (s *ActionServer) ObserveLfpConsentOffer(ctx context.Context, r *storyv1.ObserveLfpConsentOfferRequest) (*storyv1.ObserveLfpConsentOfferResponse, error) {
	if !s.ready() {
		return nil, actionError(lfpconsentstore.ErrUnavailable)
	}
	if r == nil || r.ProtocolVersion != 1 || !nonzero(r.StoryId, 16) {
		return nil, invalidAction()
	}
	verified, e := s.Verifier.Verify(ctx, r, storyv1.LfpConsentActionService_ObserveLfpConsentOffer_FullMethodName)
	if e != nil {
		return nil, e
	}
	id := uuid.UUID(r.StoryId)
	bound, e := s.Journal.ReadActorOfferContext(ctx, id, s.Origin)
	if e != nil {
		return nil, actionError(e)
	}
	if e = s.Eligibility.CheckHumanOfferViewer(ctx, bound, verified.Identity()); e != nil {
		return nil, actionError(e)
	}
	if !verified.CredentialExpiry().After(actorNow(s.Verifier.Clock)) {
		return nil, actionError(lfpconsentstore.ErrUnavailable)
	}
	current, e := s.Journal.ReadActorOfferContext(ctx, id, s.Origin)
	if e != nil {
		return nil, actionError(e)
	}
	if current.Offer() != bound.Offer() || !verified.CredentialExpiry().After(actorNow(s.Verifier.Clock)) {
		return nil, actionError(lfpconsentstore.ErrStale)
	}
	return &storyv1.ObserveLfpConsentOfferResponse{Offer: wireOffer(current.Offer())}, nil
}

func (s *ActionServer) RespondWithLfpConsent(ctx context.Context, r *storyv1.RespondWithLfpConsentRequest) (*storyv1.RespondWithLfpConsentResponse, error) {
	if !s.ready() {
		return nil, actionError(lfpconsentstore.ErrUnavailable)
	}
	if r == nil || r.ProtocolVersion != 1 || !nonzero(r.ResponseOperationId, 16) || (uint64(r.ResponseType) != lfpconsent.Join && uint64(r.ResponseType) != lfpconsent.Invite) {
		return nil, invalidAction()
	}
	offer, e := readOfferView(r.Offer)
	if e != nil {
		return nil, invalidAction()
	}
	verified, e := s.Verifier.Verify(ctx, r, storyv1.LfpConsentActionService_RespondWithLfpConsent_FullMethodName)
	if e != nil {
		return nil, e
	}
	actor, e := verified.ActorMaterial()
	if e != nil {
		return nil, actionError(e)
	}
	response := lfpconsentstore.Response{OperationID: actor.OperationID, AccountID: actor.AccountID, ProfileID: actor.ProfileID, SessionEpoch: actor.SessionEpoch, RequestSHA256: actor.PrincipalRequestSHA256, Offer: offer, Type: uint64(r.ResponseType)}
	lookup := func() (lfpconsent.Envelope, bool, error) {
		return s.Journal.FindActorResponse(ctx, response, actor, s.Origin, verified.CredentialExpiry())
	}
	envelope, found, e := lookup()
	if e != nil {
		return nil, actionError(e)
	}
	if !found {
		bound, checkError := s.Journal.ReadActorOfferContext(ctx, offer.StoryID, s.Origin)
		if checkError == nil && !protoOfferMatches(offer, bound.Offer()) {
			checkError = lfpconsentstore.ErrStale
		}
		if checkError == nil {
			checkError = s.Eligibility.CheckActorResponder(ctx, bound, actor)
		}
		if checkError != nil {
			// A concurrent original operation may have committed while fresh
			// privacy was waiting. Return only that original exact replay.
			envelope, found, e = lookup()
			if e != nil {
				return nil, actionError(e)
			}
			if !found {
				return nil, actionError(checkError)
			}
		} else {
			envelope, e = s.Journal.RecordActorResponse(ctx, response, actor, s.Origin, verified.CredentialExpiry())
			if e != nil {
				return nil, actionError(e)
			}
		}
	}
	ack, e := s.Registry.Deliver(ctx, envelope.Consent().ConsentID, envelope.QualifiedSHA256())
	if e != nil {
		return nil, actionError(e)
	}
	_, registrationSHA, hashError := lfpconsent.EncodeRegistration(ack.Registration)
	bound, qualifiedError := lfpconsent.HashActorBoundRegistration(registrationSHA, envelope.QualifiedSHA256())
	if hashError != nil || qualifiedError != nil || bound != ack.BoundSHA256 || ack.Origin != s.Origin || ack.QualifiedSHA256 != envelope.QualifiedSHA256() || ack.Registration.OperationID != envelope.Consent().ConsentID || ack.Registration.ConsentID != envelope.Consent().ConsentID || ack.Registration.ConsentSHA256 != envelope.ConsentSHA256() {
		return nil, actionError(lfpconsentstore.ErrUnavailable)
	}
	return &storyv1.RespondWithLfpConsentResponse{}, nil
}

func invalidAction() error { return status.Error(codes.InvalidArgument, "invalid LFP action") }
func actionError(e error) error {
	if errors.Is(e, ErrResponderIneligible) || errors.Is(e, lfpconsentstore.ErrStale) || errors.Is(e, lfpconsentstore.ErrConflict) || status.Code(e) == codes.FailedPrecondition {
		return status.Error(codes.FailedPrecondition, "LFP action ineligible")
	}
	return status.Error(codes.Unavailable, "LFP action unavailable")
}

func wireOffer(o lfpconsentstore.Offer) *storyv1.LfpConsentOfferView {
	return &storyv1.LfpConsentOfferView{ProtocolVersion: 1, StoryId: bytes.Clone(o.StoryID[:]), StoryIncarnation: bytes.Clone(o.Incarnation[:]), OfferRevision: o.Revision, AuthorProfileId: bytes.Clone(o.AuthorProfileID[:]), CriteriaSha256: bytes.Clone(o.CriteriaSHA256[:]), OfferPayloadSha256: bytes.Clone(o.PayloadSHA256[:]), OfferExpiresAtUnixMillis: o.ExpiresAtUnixMillis}
}
func readOfferView(p *storyv1.LfpConsentOfferView) (lfpconsentstore.Offer, error) {
	if p == nil || p.ProtocolVersion != 1 || len(p.ProtoReflect().GetUnknown()) != 0 || !nonzero(p.StoryId, 16) || !nonzero(p.StoryIncarnation, 16) || p.OfferRevision == 0 || !nonzero(p.AuthorProfileId, 16) || !nonzero(p.CriteriaSha256, 32) || !nonzero(p.OfferPayloadSha256, 32) || p.OfferExpiresAtUnixMillis <= 0 {
		return lfpconsentstore.Offer{}, lfpconsent.ErrInvalid
	}
	return lfpconsentstore.Offer{StoryID: uuid.UUID(p.StoryId), Incarnation: uuid.UUID(p.StoryIncarnation), Revision: p.OfferRevision, AuthorProfileID: uuid.UUID(p.AuthorProfileId), CriteriaSHA256: [32]byte(p.CriteriaSha256), PayloadSHA256: [32]byte(p.OfferPayloadSha256), ExpiresAtUnixMillis: p.OfferExpiresAtUnixMillis}, nil
}
func protoOfferMatches(a, b lfpconsentstore.Offer) bool {
	return a.StoryID == b.StoryID && a.Incarnation == b.Incarnation && a.Revision == b.Revision && a.AuthorProfileID == b.AuthorProfileID && a.CriteriaSHA256 == b.CriteriaSHA256 && a.PayloadSHA256 == b.PayloadSHA256 && a.ExpiresAtUnixMillis == b.ExpiresAtUnixMillis
}
