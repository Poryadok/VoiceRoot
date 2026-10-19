package lfpconsenttransport

import (
	"bytes"
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/storylfpadmission"
	"voice/backend/story/internal/lfpconsentstore"
)

type ActorRegistrationJournal interface {
	ReadActorConsentForDelivery(context.Context, uuid.UUID, [32]byte, storylfpadmission.Origin) (lfpconsent.Envelope, error)
	SaveActorRegistration(context.Context, lfpconsentstore.ActorRegistration) error
}

type ActorRegistryProducer struct {
	Client        storyv1.LfpActorBoundConsentRegistryServiceClient
	Signer        *principal.Issuer
	Origin        storylfpadmission.Origin
	PeerAuthority storylfpadmission.AuthenticatedPeerAuthority
	Journal       ActorRegistrationJournal
}

func (p *ActorRegistryProducer) Deliver(ctx context.Context, id uuid.UUID, qualified [32]byte) (lfpconsentstore.ActorRegistration, error) {
	fail := func() (lfpconsentstore.ActorRegistration, error) {
		return lfpconsentstore.ActorRegistration{}, status.Error(codes.Unavailable, "LFP registration unavailable")
	}
	if p == nil || p.Client == nil || p.Signer == nil || !p.Origin.Valid() || p.PeerAuthority == nil || p.Journal == nil || id == uuid.Nil || qualified == [32]byte{} {
		return fail()
	}
	load := func() (lfpconsent.Envelope, error) {
		envelope, e := p.Journal.ReadActorConsentForDelivery(ctx, id, qualified, p.Origin)
		if e != nil {
			return lfpconsent.Envelope{}, e
		}
		if !envelope.Valid() || envelope.Origin() != p.Origin || envelope.Consent().ConsentID != id || envelope.QualifiedSHA256() != qualified {
			return lfpconsent.Envelope{}, lfpconsentstore.ErrUnavailable
		}
		return envelope, nil
	}
	envelope, e := load()
	if e != nil {
		return lfpconsentstore.ActorRegistration{}, actorOfferError(e)
	}
	ch, ah := envelope.ConsentSHA256(), envelope.ActorSHA256()
	request := &storyv1.ReadActorBoundLfpConsentRegistrationRequest{ProtocolVersion: 1, RegistrationOperationId: bytes.Clone(id[:]), ConsentId: bytes.Clone(id[:]), ConsentSha256: bytes.Clone(ch[:]), ActorSha256: bytes.Clone(ah[:]), QualifiedConsentSha256: bytes.Clone(qualified[:]), Origin: wireOrigin(p.Origin)}
	// These helpers sign exact bodies and authenticate actual TLS endpoint peers;
	// the client below exposes ONLY ActorBound methods, never barev1 fallback.
	network := RegistryProducer{Signer: p.Signer, Origin: p.Origin, PeerAuthority: p.PeerAuthority}
	readCtx, cancel, e := network.signedContext(ctx, request, storyv1.LfpActorBoundConsentRegistryService_ReadActorBoundLfpConsentRegistration_FullMethodName)
	if e != nil {
		return fail()
	}
	var endpoint peer.Peer
	read, readError := p.Client.ReadActorBoundLfpConsentRegistration(readCtx, request, grpc.Peer(&endpoint))
	peerError := network.checkPeer(readCtx, endpoint)
	cancel()
	if peerError != nil {
		return fail()
	}
	var receipt *storyv1.LfpActorBoundRegistrationAck
	if readError == nil {
		if read == nil || len(read.ProtoReflect().GetUnknown()) != 0 {
			return fail()
		}
		receipt = read.Acknowledgement
	} else if status.Code(readError) == codes.NotFound {
		// Revalidate original durable sidecar/cohort immediately before EVERY
		// outbound request. A read that raced retirement cannot authorize Register.
		again, e := load()
		if e != nil {
			return lfpconsentstore.ActorRegistration{}, actorOfferError(e)
		}
		if !bytes.Equal(again.ConsentBytes(), envelope.ConsentBytes()) || !bytes.Equal(again.ActorBytes(), envelope.ActorBytes()) {
			return fail()
		}
		register := &storyv1.RegisterActorBoundLfpConsentRequest{ProtocolVersion: 1, RegistrationOperationId: bytes.Clone(id[:]), Envelope: wireEnvelope(again)}
		registerCtx, stop, e := network.signedContext(ctx, register, storyv1.LfpActorBoundConsentRegistryService_RegisterActorBoundLfpConsent_FullMethodName)
		if e != nil {
			return fail()
		}
		endpoint = peer.Peer{}
		ack, registerError := p.Client.RegisterActorBoundLfpConsent(registerCtx, register, grpc.Peer(&endpoint))
		peerError = network.checkPeer(registerCtx, endpoint)
		stop()
		if registerError != nil || peerError != nil || ack == nil || len(ack.ProtoReflect().GetUnknown()) != 0 {
			return fail()
		}
		receipt = ack.Acknowledgement
	} else {
		return fail()
	}
	ack, e := readActorAck(receipt, envelope)
	if e != nil {
		return fail()
	}
	// Save rechecks admitted origin and binds both base and qualified ACK in a
	// single SQL transaction. No response success precedes durable local capture.
	if e = p.Journal.SaveActorRegistration(ctx, ack); e != nil {
		return lfpconsentstore.ActorRegistration{}, actorOfferError(e)
	}
	return ack, nil
}

func wireEnvelope(e lfpconsent.Envelope) *storyv1.LfpActorBoundConsentEnvelope {
	ch, ah, q := e.ConsentSHA256(), e.ActorSHA256(), e.QualifiedSHA256()
	return &storyv1.LfpActorBoundConsentEnvelope{ProtocolVersion: 1, CanonicalV1Consent: e.ConsentBytes(), CanonicalActor: e.ActorBytes(), Origin: wireOrigin(e.Origin()), ConsentSha256: bytes.Clone(ch[:]), ActorSha256: bytes.Clone(ah[:]), QualifiedConsentSha256: bytes.Clone(q[:])}
}

func readActorAck(p *storyv1.LfpActorBoundRegistrationAck, envelope lfpconsent.Envelope) (lfpconsentstore.ActorRegistration, error) {
	empty := func() (lfpconsentstore.ActorRegistration, error) {
		return lfpconsentstore.ActorRegistration{}, lfpconsent.ErrInvalid
	}
	if p == nil || p.ProtocolVersion != 1 || len(p.ProtoReflect().GetUnknown()) != 0 || p.Registration == nil || len(p.Registration.ProtoReflect().GetUnknown()) != 0 || !nonzero(p.QualifiedConsentSha256, 32) || !nonzero(p.ActorBoundRegistrationSha256, 32) || !nonzero(p.Registration.RegistrationSha256, 32) {
		return empty()
	}
	o, e := readOrigin(p.Origin)
	if e != nil || o != envelope.Origin() || [32]byte(p.QualifiedConsentSha256) != envelope.QualifiedSHA256() {
		return empty()
	}
	base := proto.Clone(p.Registration).(*storyv1.LfpConsentRegistration)
	base.RegistrationSha256 = nil
	b, e := proto.MarshalOptions{Deterministic: true}.Marshal(base)
	if e != nil {
		return empty()
	}
	r, h, e := lfpconsent.DecodeRegistration(b)
	if e != nil || !bytes.Equal(h[:], p.Registration.RegistrationSha256) || r.OperationID != envelope.Consent().ConsentID || r.ConsentID != envelope.Consent().ConsentID || r.ConsentSHA256 != envelope.ConsentSHA256() {
		return empty()
	}
	bound, e := lfpconsent.HashActorBoundRegistration(h, envelope.QualifiedSHA256())
	if e != nil || !bytes.Equal(bound[:], p.ActorBoundRegistrationSha256) {
		return empty()
	}
	return lfpconsentstore.ActorRegistration{Registration: r, Origin: o, QualifiedSHA256: envelope.QualifiedSHA256(), BoundSHA256: bound}, nil
}
