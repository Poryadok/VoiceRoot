package lfpconsenttransport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
	"voice/backend/story/internal/lfpconsentstore"
)

type fakeJournal struct {
	canonical []byte
	calls     int
	observed  storylfpadmission.VerifiedPeer
}

func (j *fakeJournal) ReadConsentForDelivery(context.Context, uuid.UUID, [32]byte, storylfpadmission.Origin) ([]byte, error) {
	return j.canonical, nil
}

type eligibilityFunc func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error

func (f eligibilityFunc) CheckResponder(c context.Context, s, a, r uuid.UUID) error {
	return f(c, s, a, r)
}

func (j *fakeJournal) ValidateConsent(_ context.Context, v lfpconsentstore.Validation, p storylfpadmission.VerifiedPeer) (lfpconsentstore.Offer, error) {
	j.calls++
	j.observed = p
	return lfpconsentstore.Offer{StoryID: v.StoryID, Incarnation: v.Incarnation, Revision: v.Revision, PayloadSHA256: v.PayloadSHA256, CriteriaSHA256: v.CriteriaSHA256, ExpiresAtUnixMillis: time.Now().Add(time.Minute).UnixMilli()}, nil
}

type peerAuthority struct {
	origin storylfpadmission.Origin
	err    error
}

func (a peerAuthority) ResolveAuthenticatedPeer(ctx context.Context) (storylfpadmission.Origin, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Subject != "service:matchmaking" {
		return storylfpadmission.Origin{}, errors.New("missing verified service")
	}
	return a.origin, a.err
}

func TestOfferProtectedCredentialBoundary(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	now := time.Now().UTC().Truncate(time.Second)
	origin, e := storylfpadmission.NewOrigin([16]byte{1}, [16]byte{2}, [16]byte{3}, [32]byte{4})
	require.NoError(t, e)
	consentID, storyID, incarnation := uuid.New(), uuid.New(), uuid.New()
	c := lfpconsent.Consent{ConsentID: consentID, ResponseOperationID: uuid.New(), StoryID: storyID, StoryIncarnation: incarnation, OfferRevision: 1, AuthorProfileID: uuid.New(), ResponderAccountID: uuid.New(), ResponderProfileID: uuid.New(), ResponderSessionEpoch: 7, ResponseType: lfpconsent.Join, CriteriaSHA256: [32]byte{3}, OfferPayloadSHA256: [32]byte{2}, CreatedAtUnixMillis: now.UnixMilli(), OfferExpiresAtUnixMillis: now.Add(time.Minute).UnixMilli(), PrincipalRequestSHA256: [32]byte{4}}
	canonical, hash, e := lfpconsent.Encode(c)
	require.NoError(t, e)
	request := &storyv1.ValidateLfpConsentOfferRequest{ProtocolVersion: 1, ConsentId: consentID[:], StoryId: storyID[:], StoryIncarnation: incarnation[:], OfferRevision: 1, ConsentSha256: make([]byte, 32), OfferPayloadSha256: make([]byte, 32), CriteriaSha256: make([]byte, 32)}
	request.ConsentSha256 = hash[:]
	request.OfferPayloadSha256[0] = 2
	request.CriteriaSha256[0] = 3
	tests := []struct {
		name, issuer, method                                     string
		raw, duplicate, noTLS, tamper, noReplay, noPeer, unknown bool
		want                                                     codes.Code
		privacy                                                  int
	}{
		{name: "qualified", want: codes.OK}, {name: "wrong issuer", issuer: "gateway", want: codes.Unauthenticated},
		{name: "wrong method", method: "/voice.story.v1.StoryService/GetStory", want: codes.Unauthenticated},
		{name: "raw identity", raw: true, want: codes.Unauthenticated}, {name: "duplicate bearer", duplicate: true, want: codes.Unauthenticated},
		{name: "plaintext", noTLS: true, want: codes.Unauthenticated}, {name: "changed body", tamper: true, want: codes.Unauthenticated},
		{name: "missing replay", noReplay: true, want: codes.Unavailable}, {name: "missing cohort peer", noPeer: true, want: codes.Unavailable},
		{name: "unknown field", unknown: true, want: codes.InvalidArgument},
		{name: "missing visibility", privacy: 1, want: codes.Unavailable},
		{name: "visibility revoked", privacy: 2, want: codes.FailedPrecondition},
		{name: "visibility outage", privacy: 3, want: codes.Unavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := proto.Clone(request).(*storyv1.ValidateLfpConsentOfferRequest)
			issuer := tt.issuer
			if issuer == "" {
				issuer = "matchmaking"
			}
			method := tt.method
			if method == "" {
				method = storyv1.LfpConsentOfferService_ValidateLfpConsentOffer_FullMethodName
			}
			signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "test", PrivateKey: key, Clock: func() time.Time { return now }})
			require.NoError(t, e)
			h, e := principal.RequestHash(r)
			require.NoError(t, e)
			token, e := signer.IssueService(principal.ServiceInput{Audience: "story", RPC: method, RequestID: "test-request", RequestHash: h})
			require.NoError(t, e)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "test-request")
			if tt.raw {
				md.Set("x-profile-id", uuid.NewString())
			}
			if tt.duplicate {
				md.Append("authorization", "Bearer "+token)
			}
			ctx := metadata.NewIncomingContext(context.Background(), md)
			// Synthetic TLS provenance exercises the component boundary only; no
			// installed trust manifest or live listener is claimed by this fixture.
			if !tt.noTLS {
				ctx = peer.NewContext(ctx, &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}}})
			}
			j := &fakeJournal{canonical: canonical}
			var mu sync.Mutex
			seen := map[string]bool{}
			cfg := OfferConfig{Origin: origin, PeerAuthority: peerAuthority{origin: origin}, Clock: func() time.Time { return now }, Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(_ context.Context, iss, jti string, _ time.Time) error {
				mu.Lock()
				defer mu.Unlock()
				if seen[iss+jti] {
					return errors.New("replayed")
				}
				seen[iss+jti] = true
				return nil
			}}
			cfg.Eligibility = eligibilityFunc(func(_ context.Context, story, author, responder uuid.UUID) error {
				require.Equal(t, c.StoryID, story)
				require.Equal(t, c.AuthorProfileID, author)
				require.Equal(t, c.ResponderProfileID, responder)
				if tt.privacy == 2 {
					return ErrResponderIneligible
				}
				if tt.privacy == 3 {
					return errors.New("privacy dependency unavailable")
				}
				return nil
			})
			if tt.privacy == 1 {
				cfg.Eligibility = nil
			}
			if tt.noReplay {
				cfg.Replay = nil
			}
			if tt.noPeer {
				cfg.PeerAuthority = nil
			}
			if tt.tamper {
				r.OfferRevision++
			}
			if tt.unknown {
				r.ProtoReflect().SetUnknown([]byte{0x50, 1})
			}
			s := &OfferServer{Journal: j, Config: cfg}
			resp, e := s.ValidateLfpConsentOffer(ctx, r)
			require.Equal(t, tt.want, status.Code(e))
			if tt.want == codes.OK {
				require.NotNil(t, resp)
				require.Equal(t, 1, j.calls)
				require.True(t, j.observed.Valid())
				require.Equal(t, origin, j.observed.Origin())
				require.Equal(t, r.ConsentId, resp.ConsentId)
				_, e = s.ValidateLfpConsentOffer(ctx, r)
				require.Error(t, e)
				require.Equal(t, 1, j.calls)
			} else {
				require.Nil(t, resp)
				require.Zero(t, j.calls)
			}
		})
	}
}
