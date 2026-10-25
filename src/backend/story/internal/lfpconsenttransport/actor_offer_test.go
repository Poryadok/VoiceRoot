package lfpconsenttransport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
	"voice/backend/story/internal/lfpconsentstore"
)

type actorEligibilityFunc func(context.Context, lfpconsentstore.OfferContext, lfpconsent.Actor) error

func (f actorEligibilityFunc) CheckActorResponder(c context.Context, o lfpconsentstore.OfferContext, a lfpconsent.Actor) error {
	return f(c, o, a)
}

func TestActorOfferActualPostgresAndMTLS(t *testing.T) {
	for _, accountType := range []uint64{lfpconsent.Guest, lfpconsent.Regular} {
		name := "guest"
		if accountType == lfpconsent.Regular {
			name = "regular"
		}
		t.Run(name, func(t *testing.T) {
			journal, envelope := actorJournalFixture(t, accountType)
			key, e := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, e)
			now := time.Now().UTC().Truncate(time.Second)
			var clock atomic.Int64
			clock.Store(now.UnixNano())
			var mode atomic.Int32
			var mu sync.Mutex
			seen := map[string]bool{}
			cfg := ActorOfferConfig{Origin: envelope.Origin(), PeerAuthority: peerAuthority{origin: envelope.Origin()}, Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: func() time.Time { return time.Unix(0, clock.Load()) }, Replay: func(_ context.Context, issuer, jti string, _ time.Time) error {
				mu.Lock()
				defer mu.Unlock()
				if seen[issuer+jti] {
					return errors.New("replay")
				}
				seen[issuer+jti] = true
				return nil
			}}
			cfg.Eligibility = actorEligibilityFunc(func(ctx context.Context, bound lfpconsentstore.OfferContext, actor lfpconsent.Actor) error {
				if actor != envelope.Actor() || actor.AccountType != accountType || bound.Visibility() != "everyone" || bound.Offer().StoryID != envelope.Consent().StoryID {
					return errors.New("wrong original saved viewer/audience")
				}
				switch mode.Load() {
				case 1:
					return ErrResponderIneligible
				case 2:
					return errors.New("privacy unavailable")
				case 3:
					_, e := journal.Pool.Exec(ctx, "UPDATE stories SET visibility='friends' WHERE id=$1", envelope.Consent().StoryID)
					return e
				case 4:
					clock.Store(now.Add(time.Minute).UnixNano())
				}
				return nil
			})
			server := &ActorOfferServer{Journal: journal, Config: cfg}
			conn := actorTLSClient(t, func(g *grpc.Server) { storyv1.RegisterLfpActorBoundConsentOfferServiceServer(g, server) })
			client := storyv1.NewLfpActorBoundConsentOfferServiceClient(conn)
			c := envelope.Consent()
			q, ah, ch := envelope.QualifiedSHA256(), envelope.ActorSHA256(), envelope.ConsentSHA256()
			base := &storyv1.ValidateActorBoundLfpConsentOfferRequest{ProtocolVersion: 1, Origin: wireOrigin(envelope.Origin()), QualifiedConsentSha256: q[:], ActorSha256: ah[:], Offer: &storyv1.ValidateLfpConsentOfferRequest{ProtocolVersion: 1, ConsentId: c.ConsentID[:], ConsentSha256: ch[:], StoryId: c.StoryID[:], StoryIncarnation: c.StoryIncarnation[:], OfferRevision: c.OfferRevision, OfferPayloadSha256: c.OfferPayloadSHA256[:], CriteriaSha256: c.CriteriaSHA256[:]}}
			for _, tt := range []struct {
				name                                     string
				mode                                     int32
				tamper, wrongIssuer, unknown, wrongActor bool
				want                                     codes.Code
			}{
				{name: "qualified", want: codes.OK},
				{name: "fresh privacy denies", mode: 1, want: codes.FailedPrecondition},
				{name: "privacy outage", mode: 2, want: codes.Unavailable},
				{name: "privacy wait crosses credential expiry", mode: 4, want: codes.Unavailable},
				{name: "final SQL sees changed audience", mode: 3, want: codes.FailedPrecondition},
				{name: "changed signed request", tamper: true, want: codes.Unauthenticated},
				{name: "wrong service", wrongIssuer: true, want: codes.Unauthenticated},
				{name: "unknown field", unknown: true, want: codes.InvalidArgument},
				{name: "wrong saved actor hash", wrongActor: true, want: codes.FailedPrecondition},
			} {
				t.Run(tt.name, func(t *testing.T) {
					_, e := journal.Pool.Exec(context.Background(), "UPDATE stories SET visibility='everyone' WHERE id=$1", c.StoryID)
					require.NoError(t, e)
					clock.Store(now.UnixNano())
					mode.Store(tt.mode)
					r := proto.Clone(base).(*storyv1.ValidateActorBoundLfpConsentOfferRequest)
					if tt.unknown {
						r.ProtoReflect().SetUnknown([]byte{0x50, 1})
					}
					if tt.wrongActor {
						r.ActorSha256[0]++
					}
					issuer := "matchmaking"
					if tt.wrongIssuer {
						issuer = "story"
					}
					signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "test", PrivateKey: key, Clock: func() time.Time { return now }})
					require.NoError(t, e)
					h, e := principal.RequestHash(r)
					require.NoError(t, e)
					token, e := signer.IssueService(principal.ServiceInput{Audience: "story", RPC: storyv1.LfpActorBoundConsentOfferService_ValidateActorBoundLfpConsentOffer_FullMethodName, RequestID: "actor-bound-request", RequestHash: h})
					require.NoError(t, e)
					if tt.tamper {
						r.Offer.OfferRevision++
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "actor-bound-request"))
					got, e := client.ValidateActorBoundLfpConsentOffer(ctx, r)
					require.Equal(t, tt.want, status.Code(e))
					if tt.want == codes.OK {
						require.Equal(t, ch[:], got.Offer.ConsentSha256)
						require.Equal(t, q[:], got.QualifiedConsentSha256)
						require.Equal(t, ah[:], got.ActorSha256)
						require.Equal(t, c.CriteriaSHA256, sha256.Sum256(got.CanonicalCriteriaJson))
						require.True(t, criteriaMatches(got.CanonicalCriteriaJson, c.CriteriaSHA256))
						_, e = client.ValidateActorBoundLfpConsentOffer(ctx, r)
						require.Equal(t, codes.Unavailable, status.Code(e), "sameJTI replay denied")
					} else {
						require.Nil(t, got)
					}
				})
			}
			// Registering ONLY the new service provides no barev1 downgrade.
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			e = conn.Invoke(ctx, storyv1.LfpConsentOfferService_ValidateLfpConsentOffer_FullMethodName, base.Offer, &storyv1.ValidateLfpConsentOfferResponse{})
			require.Equal(t, codes.Unimplemented, status.Code(e))
		})
	}
}
