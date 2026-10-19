package lfpconsenttransport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/privacy"
	"voice/backend/story/internal/lfpconsentstore"
)

type actorDynamicFixtureDelivery struct {
	p  *ActorRegistryProducer
	mm *actorMMFixture
	j  *lfpconsentstore.Store
}

func (f actorDynamicFixtureDelivery) Deliver(ctx context.Context, id uuid.UUID, q [32]byte) (lfpconsentstore.ActorRegistration, error) {
	// Fixture learns the original durable Story envelope, then the actual
	// producer still performs its own signed network Read/Register/readback.
	envelope, e := f.j.ReadActorConsentForDelivery(ctx, id, q, f.p.Origin)
	if e != nil {
		return lfpconsentstore.ActorRegistration{}, e
	}
	f.mm.mu.Lock()
	if f.mm.expected.Consent().ConsentID != id {
		f.mm.expected = envelope
		f.mm.ack = nil
	}
	f.mm.mu.Unlock()
	return f.p.Deliver(ctx, id, q)
}

func TestActionsActualHumanMTLSAndPostgres(t *testing.T) {
	for _, initialType := range []string{"guest", "regular"} {
		t.Run(initialType, func(t *testing.T) {
			journal, old := actorJournalFixture(t, lfpconsent.Guest)
			producer, mm := actorRegistryFixture(t, journal, old)
			key, e := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, e)
			signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
			require.NoError(t, e)
			var replayMu sync.Mutex
			seen := map[string]bool{}
			verifier := &HumanVerifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Epochs: func(context.Context, string, int64) error { return nil }, Replay: func(_ context.Context, iss, jti string, _ time.Time) error {
				replayMu.Lock()
				defer replayMu.Unlock()
				if seen[iss+jti] {
					return errors.New("replayed")
				}
				seen[iss+jti] = true
				return nil
			}}
			var deny atomic.Bool
			var floorCalls atomic.Int32
			eligibility := &ActorPrivacy{Floor: actorFloorFunc(func(context.Context, uuid.UUID) (privacy.Audience, error) {
				floorCalls.Add(1)
				if deny.Load() {
					return privacy.Nobody(), nil
				}
				return privacy.EveryoneWithGuests(), nil
			})}
			handler := &ActionServer{Verifier: verifier, Origin: old.Origin(), Journal: journal, Eligibility: eligibility, Registry: actorDynamicFixtureDelivery{producer, mm, journal}}
			conn := actorTLSClient(t, func(s *grpc.Server) { storyv1.RegisterLfpConsentActionServiceServer(s, handler) })
			client := storyv1.NewLfpConsentActionServiceClient(conn)
			account, profile := uuid.New(), uuid.New()
			signed := func(r proto.Message, method, kind string, epoch int64, owner bool) context.Context {
				a, p := account, profile
				if owner {
					a = uuid.New()
					p = old.Consent().AuthorProfileID
				}
				h, e := principal.RequestHash(r)
				require.NoError(t, e)
				requestID := uuid.NewString()
				token, e := signer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "story", RPC: method, RequestID: requestID, RequestHash: h, AccountID: a.String(), ProfileID: p.String(), SessionEpoch: epoch, AccountType: kind, ClientExpiresAt: time.Now().Add(time.Minute)})
				require.NoError(t, e)
				return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
			}
			pubOp, storyID := uuid.New(), old.Consent().StoryID
			pub := &storyv1.PublishLfpConsentOfferRequest{ProtocolVersion: 1, PublicationOperationId: pubOp[:], StoryId: storyID[:]}
			published, e := client.PublishLfpConsentOffer(signed(pub, storyv1.LfpConsentActionService_PublishLfpConsentOffer_FullMethodName, "regular", 7, true), pub)
			require.NoError(t, e)
			observe := &storyv1.ObserveLfpConsentOfferRequest{ProtocolVersion: 1, StoryId: storyID[:]}
			observed, e := client.ObserveLfpConsentOffer(signed(observe, storyv1.LfpConsentActionService_ObserveLfpConsentOffer_FullMethodName, initialType, 9, false), observe)
			require.NoError(t, e)
			require.True(t, proto.Equal(published.Offer, observed.Offer))
			for _, responseType := range []storyv1.LfpConsentResponseType{storyv1.LfpConsentResponseType(lfpconsent.Join), storyv1.LfpConsentResponseType(lfpconsent.Invite)} {
				op := uuid.New()
				request := &storyv1.RespondWithLfpConsentRequest{ProtocolVersion: 1, ResponseOperationId: op[:], Offer: proto.Clone(observed.Offer).(*storyv1.LfpConsentOfferView), ResponseType: responseType}
				mm.mode.Store(3)
				_, e = client.RespondWithLfpConsent(signed(request, storyv1.LfpConsentActionService_RespondWithLfpConsent_FullMethodName, initialType, 9, false), request)
				require.Equal(t, codes.Unavailable, status.Code(e), "lost committed MM ACK cannot report public success")
				var canonical []byte
				require.NoError(t, journal.Pool.QueryRow(context.Background(), "SELECT a.canonical_actor FROM story_lfp_consent_actors a JOIN story_lfp_consents c USING(consent_id) WHERE c.response_operation_id=$1", op).Scan(&canonical))
				original, _, e := lfpconsent.DecodeActor(canonical)
				require.NoError(t, e)
				wantType := uint64(lfpconsent.Regular)
				if initialType == "guest" {
					wantType = lfpconsent.Guest
				}
				require.Equal(t, wantType, original.AccountType)
				require.Equal(t, int64(9), original.SessionEpoch)
				deny.Store(true)
				calls := floorCalls.Load()
				// Fresh authenticated type/epoch may differ. Exact operation replay
				// recovers original sidecar/MM witness even after current privacy deny.
				got, e := client.RespondWithLfpConsent(signed(request, storyv1.LfpConsentActionService_RespondWithLfpConsent_FullMethodName, "regular", 11, false), request)
				require.NoError(t, e)
				require.Zero(t, proto.Size(got))
				require.Equal(t, calls, floorCalls.Load())
				var saved []byte
				require.NoError(t, journal.Pool.QueryRow(context.Background(), "SELECT a.canonical_actor FROM story_lfp_consent_actors a JOIN story_lfp_consents c USING(consent_id) WHERE c.response_operation_id=$1", op).Scan(&saved))
				require.Equal(t, canonical, saved)
				_, e = client.ObserveLfpConsentOffer(signed(observe, storyv1.LfpConsentActionService_ObserveLfpConsentOffer_FullMethodName, initialType, 11, false), observe)
				require.Equal(t, codes.FailedPrecondition, status.Code(e))
				fresh := proto.Clone(request).(*storyv1.RespondWithLfpConsentRequest)
				nextOp := uuid.New()
				fresh.ResponseOperationId = nextOp[:]
				_, e = client.RespondWithLfpConsent(signed(fresh, storyv1.LfpConsentActionService_RespondWithLfpConsent_FullMethodName, initialType, 11, false), fresh)
				require.Equal(t, codes.FailedPrecondition, status.Code(e))
				var count int
				require.NoError(t, journal.Pool.QueryRow(context.Background(), "SELECT count(*) FROM story_lfp_consents WHERE response_operation_id=$1", nextOp).Scan(&count))
				require.Zero(t, count)
				deny.Store(false)
			}
			_, e = client.ObserveLfpConsentOffer(context.Background(), observe)
			require.Equal(t, codes.Unauthenticated, status.Code(e))
			_, e = client.ObserveLfpConsentOffer(signed(observe, storyv1.LfpConsentActionService_ObserveLfpConsentOffer_FullMethodName, "sdk-account", 11, false), observe)
			require.Equal(t, codes.PermissionDenied, status.Code(e))
		})
	}
}
