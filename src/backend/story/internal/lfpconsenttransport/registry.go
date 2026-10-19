package lfpconsenttransport

import (
	"bytes"
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/storylfpadmission"
)

type RegistrationJournal interface {
	ReadConsentForDelivery(context.Context, uuid.UUID, [32]byte, storylfpadmission.Origin) ([]byte, error)
	SaveRegistration(context.Context, lfpconsent.Registration, [32]byte) error
}

type RegistryProducer struct {
	Client        storyv1.LfpConsentRegistryServiceClient
	Signer        *principal.Issuer
	Origin        storylfpadmission.Origin
	PeerAuthority storylfpadmission.AuthenticatedPeerAuthority
	Journal       RegistrationJournal
}

func (p *RegistryProducer) Deliver(ctx context.Context, consentID uuid.UUID, hash [32]byte) (lfpconsent.Registration, [32]byte, error) {
	empty := func() (lfpconsent.Registration, [32]byte, error) {
		return lfpconsent.Registration{}, [32]byte{}, status.Error(codes.Unavailable, "LFP registration unavailable")
	}
	if p == nil || p.Client == nil || p.Signer == nil || !p.Origin.Valid() || p.PeerAuthority == nil || p.Journal == nil {
		return empty()
	}
	canonical, e := p.Journal.ReadConsentForDelivery(ctx, consentID, hash, p.Origin)
	if e != nil {
		return empty()
	}
	c, h, e := lfpconsent.Decode(canonical)
	if e != nil || h != hash || c.ConsentID != consentID {
		return empty()
	}
	read := &storyv1.ReadLfpConsentRegistrationRequest{ProtocolVersion: 1, RegistrationOperationId: bytes.Clone(c.ConsentID[:]), ConsentId: bytes.Clone(c.ConsentID[:]), ConsentSha256: bytes.Clone(h[:])}
	readCtx, cancel, e := p.signedContext(ctx, read, storyv1.LfpConsentRegistryService_ReadLfpConsentRegistration_FullMethodName)
	if e != nil {
		return empty()
	}
	var endpoint peer.Peer
	got, readError := p.Client.ReadLfpConsentRegistration(readCtx, read, grpc.Peer(&endpoint))
	peerError := p.checkPeer(readCtx, endpoint)
	cancel()
	// Even a definite absent result must originate at the admitted actual TLS
	// endpoint. Outage/ambiguity is never permission to recapture search intent.
	if peerError != nil {
		return empty()
	}
	var receipt *storyv1.LfpConsentRegistration
	if readError == nil {
		if got == nil || len(got.ProtoReflect().GetUnknown()) != 0 {
			return empty()
		}
		receipt = got.Registration
	} else if status.Code(readError) == codes.NotFound {
		request := &storyv1.RegisterLfpConsentRequest{ProtocolVersion: 1, RegistrationOperationId: bytes.Clone(c.ConsentID[:]), CanonicalConsent: bytes.Clone(canonical), ConsentSha256: bytes.Clone(hash[:])}
		registerCtx, stop, e := p.signedContext(ctx, request, storyv1.LfpConsentRegistryService_RegisterLfpConsent_FullMethodName)
		if e != nil {
			return empty()
		}
		endpoint = peer.Peer{}
		ack, registerError := p.Client.RegisterLfpConsent(registerCtx, request, grpc.Peer(&endpoint))
		peerError = p.checkPeer(registerCtx, endpoint)
		stop()
		if registerError != nil || peerError != nil || ack == nil || len(ack.ProtoReflect().GetUnknown()) != 0 {
			return empty()
		}
		receipt = ack.Registration
	} else {
		return empty()
	}
	if receipt == nil || len(receipt.ProtoReflect().GetUnknown()) != 0 || !nonzero(receipt.RegistrationSha256, 32) {
		return empty()
	}
	copyReceipt := proto.Clone(receipt).(*storyv1.LfpConsentRegistration)
	copyReceipt.RegistrationSha256 = nil
	b, e := proto.MarshalOptions{Deterministic: true}.Marshal(copyReceipt)
	if e != nil {
		return empty()
	}
	r, rh, e := lfpconsent.DecodeRegistration(b)
	if e != nil || !bytes.Equal(rh[:], receipt.RegistrationSha256) || r.OperationID != c.ConsentID || r.ConsentID != c.ConsentID || r.ConsentSHA256 != hash {
		return empty()
	}
	// Never acknowledge qualified success before the exact original capture is
	// durable locally. Network calls above execute outside journal SQL locks.
	if e = p.Journal.SaveRegistration(ctx, r, rh); e != nil {
		return empty()
	}
	return r, rh, nil
}

func (p *RegistryProducer) signedContext(ctx context.Context, request proto.Message, method string) (context.Context, context.CancelFunc, error) {
	h, e := principal.RequestHash(request)
	if e != nil {
		return nil, nil, e
	}
	requestID := uuid.NewString()
	token, e := p.Signer.IssueService(principal.ServiceInput{Audience: "matchmaking", RPC: method, RequestID: requestID, RequestHash: h})
	if e != nil {
		return nil, nil, e
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	return metadata.NewOutgoingContext(bounded, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), cancel, nil
}

func (p *RegistryProducer) checkPeer(ctx context.Context, endpoint peer.Peer) error {
	ti, ok := endpoint.AuthInfo.(credentials.TLSInfo)
	if !ok || !ti.State.HandshakeComplete || len(ti.State.VerifiedChains) == 0 {
		return storylfpadmission.ErrUnavailable
	}
	_, e := storylfpadmission.ResolvePeer(peer.NewContext(ctx, &endpoint), p.Origin, p.PeerAuthority)
	return e
}
