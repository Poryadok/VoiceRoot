package lfpconsenttransport

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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

type ackJournal struct {
	canonical []byte
	readError error
	saved     lfpconsent.Registration
	hash      [32]byte
	calls     int
}

func (j *ackJournal) ReadConsentForDelivery(context.Context, uuid.UUID, [32]byte, storylfpadmission.Origin) ([]byte, error) {
	return bytes.Clone(j.canonical), j.readError
}

func (j *ackJournal) SaveRegistration(_ context.Context, r lfpconsent.Registration, h [32]byte) error {
	j.calls++
	j.saved = r
	j.hash = h
	return nil
}

type responsePeerAuthority struct{ origin storylfpadmission.Origin }

func (a responsePeerAuthority) ResolveAuthenticatedPeer(ctx context.Context) (storylfpadmission.Origin, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.AuthInfo == nil {
		return storylfpadmission.Origin{}, errors.New("no actual endpoint")
	}
	return a.origin, nil
}

type registryFixture struct {
	t              *testing.T
	key            *rsa.PublicKey
	seen           map[string]bool
	read, register int
	ack            *storyv1.LfpConsentRegistration
	lost           bool
	readError      error
	noTLS          bool
}

func (f *registryFixture) authenticate(ctx context.Context, r proto.Message, method string, opts []grpc.CallOption) {
	f.t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(f.t, ok)
	require.Empty(f.t, md.Get("x-profile-id"))
	require.Len(f.t, md.Get("authorization"), 1)
	require.Len(f.t, md.Get("x-request-id"), 1)
	h, e := principal.RequestHash(r)
	require.NoError(f.t, e)
	p, e := principal.VerifyService(ctx, md.Get("authorization")[0][7:], principal.VerifyConfig{ExpectedIssuer: "story", ExpectedAudience: "matchmaking", ExpectedRPC: method, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: h, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return f.key, nil }})
	require.NoError(f.t, e)
	require.False(f.t, f.seen[p.JWTID])
	f.seen[p.JWTID] = true
	for _, o := range opts {
		if po, ok := o.(grpc.PeerCallOption); ok && !f.noTLS {
			*po.PeerAddr = peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}}}
		}
	}
}
func (f *registryFixture) ReadLfpConsentRegistration(ctx context.Context, r *storyv1.ReadLfpConsentRegistrationRequest, opts ...grpc.CallOption) (*storyv1.ReadLfpConsentRegistrationResponse, error) {
	f.read++
	f.authenticate(ctx, r, storyv1.LfpConsentRegistryService_ReadLfpConsentRegistration_FullMethodName, opts)
	if f.readError != nil {
		return nil, f.readError
	}
	if f.ack == nil {
		return nil, status.Error(codes.NotFound, "absent")
	}
	return &storyv1.ReadLfpConsentRegistrationResponse{Registration: proto.Clone(f.ack).(*storyv1.LfpConsentRegistration)}, nil
}
func (f *registryFixture) RegisterLfpConsent(ctx context.Context, r *storyv1.RegisterLfpConsentRequest, opts ...grpc.CallOption) (*storyv1.RegisterLfpConsentResponse, error) {
	f.register++
	f.authenticate(ctx, r, storyv1.LfpConsentRegistryService_RegisterLfpConsent_FullMethodName, opts)
	c, h, e := lfpconsent.Decode(r.CanonicalConsent)
	require.NoError(f.t, e)
	require.Equal(f.t, h[:], r.ConsentSha256)
	require.Equal(f.t, c.ConsentID[:], r.RegistrationOperationId)
	reg := lfpconsent.Registration{OperationID: c.ConsentID, ConsentID: c.ConsentID, ConsentSHA256: h, RegistrationID: uuid.New(), Generation: 7, CapturedSearchIntentSHA256: [32]byte{9}}
	b, rh, e := lfpconsent.EncodeRegistration(reg)
	require.NoError(f.t, e)
	f.ack = &storyv1.LfpConsentRegistration{}
	require.NoError(f.t, proto.Unmarshal(b, f.ack))
	f.ack.RegistrationSha256 = bytes.Clone(rh[:])
	if f.lost {
		f.lost = false
		return nil, status.Error(codes.Unavailable, "lost ACK")
	}
	return &storyv1.RegisterLfpConsentResponse{Registration: f.ack}, nil
}

func TestRegistryLostAckRecoveryKeepsOriginalCapture(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "story", KeyID: "test", PrivateKey: key})
	require.NoError(t, e)
	origin, e := storylfpadmission.NewOrigin([16]byte{1}, [16]byte{2}, [16]byte{3}, [32]byte{4})
	require.NoError(t, e)
	now := time.Now().UnixMilli()
	c := lfpconsent.Consent{ConsentID: uuid.New(), ResponseOperationID: uuid.New(), StoryID: uuid.New(), StoryIncarnation: uuid.New(), OfferRevision: 1, AuthorProfileID: uuid.New(), ResponderAccountID: uuid.New(), ResponderProfileID: uuid.New(), ResponderSessionEpoch: 5, ResponseType: lfpconsent.Join, CriteriaSHA256: [32]byte{1}, OfferPayloadSHA256: [32]byte{2}, CreatedAtUnixMillis: now, OfferExpiresAtUnixMillis: now + 60000, PrincipalRequestSHA256: [32]byte{3}}
	b, h, e := lfpconsent.Encode(c)
	require.NoError(t, e)
	f := &registryFixture{t: t, key: &key.PublicKey, seen: map[string]bool{}, lost: true}
	j := &ackJournal{canonical: b}
	p := &RegistryProducer{Client: f, Signer: signer, Origin: origin, PeerAuthority: responsePeerAuthority{origin}, Journal: j}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-profile-id", "must-not-forward"))
	_, _, e = p.Deliver(ctx, c.ConsentID, h)
	require.Error(t, e)
	require.Equal(t, 1, f.register)
	require.Zero(t, j.calls)
	first := proto.Clone(f.ack).(*storyv1.LfpConsentRegistration)
	got, rh, e := p.Deliver(ctx, c.ConsentID, h)
	require.NoError(t, e)
	require.Equal(t, 1, f.register)
	require.Equal(t, 2, f.read)
	require.Equal(t, first.RegistrationId, got.RegistrationID[:])
	require.Equal(t, first.RegistrationSha256, rh[:])
	require.Equal(t, uint64(7), got.Generation)
	require.Equal(t, got, j.saved)
	f.readError = status.Error(codes.Unavailable, "unknown outcome")
	_, _, e = p.Deliver(ctx, c.ConsentID, h)
	require.Error(t, e)
	require.Equal(t, 1, f.register)
	f.readError = status.Error(codes.NotFound, "untrusted absence")
	f.noTLS = true
	_, _, e = p.Deliver(ctx, c.ConsentID, h)
	require.Error(t, e)
	require.Equal(t, 1, f.register)
	reads := f.read
	j.readError = errors.New("retired stored origin")
	_, _, e = p.Deliver(ctx, c.ConsentID, h)
	require.Error(t, e)
	require.Equal(t, reads, f.read)
	require.Equal(t, 1, f.register)
}
