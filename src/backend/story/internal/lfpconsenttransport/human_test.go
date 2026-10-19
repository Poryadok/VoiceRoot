package lfpconsenttransport

import (
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
)

func TestHumanActionRequiresExactVerifiedTypedPrincipal(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	now := time.Now().UTC().Truncate(time.Second)
	signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, e)
	operation, story := uuid.New(), uuid.New()
	method := storyv1.LfpConsentActionService_PublishLfpConsentOffer_FullMethodName
	tests := []struct {
		name, kind                                                       string
		raw, noTLS, noEpochs, replayed, tamper, wrongMethod, stale, late bool
		want                                                             codes.Code
	}{
		{name: "regular", kind: "regular", want: codes.OK}, {name: "guest", kind: "guest", want: codes.OK},
		{name: "unknown type", want: codes.PermissionDenied}, {name: "SDK account", kind: "sdk-account", want: codes.PermissionDenied},
		{name: "raw header", kind: "regular", raw: true, want: codes.Unauthenticated}, {name: "plaintext", kind: "guest", noTLS: true, want: codes.Unauthenticated},
		{name: "missing live epoch", kind: "guest", noEpochs: true, want: codes.Unavailable}, {name: "replayed token", kind: "regular", replayed: true, want: codes.Unavailable},
		{name: "changed operation", kind: "regular", tamper: true, want: codes.Unauthenticated}, {name: "wrong action", kind: "guest", wrongMethod: true, want: codes.Unauthenticated},
		{name: "stale epoch", kind: "regular", stale: true, want: codes.Unavailable},
		{name: "epoch lookup outlives credential", kind: "guest", late: true, want: codes.Unauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &storyv1.PublishLfpConsentOfferRequest{ProtocolVersion: 1, PublicationOperationId: operation[:], StoryId: story[:]}
			h, e := principal.RequestHash(r)
			require.NoError(t, e)
			account, profile := uuid.NewString(), uuid.NewString()
			token, e := signer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "story", RPC: method, RequestID: "request", RequestHash: h, AccountID: account, ProfileID: profile, SessionEpoch: 7, AccountType: tt.kind, ClientExpiresAt: now.Add(time.Minute)})
			require.NoError(t, e)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request")
			if tt.raw {
				md.Set("x-profile-id", profile)
			}
			ctx := metadata.NewIncomingContext(context.Background(), md)
			if !tt.noTLS {
				ctx = peer.NewContext(ctx, &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}}}})
			}
			verifyNow := now
			v := &HumanVerifier{Clock: func() time.Time { return verifyNow }, Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error {
				if tt.replayed {
					return errors.New("replayed")
				}
				return nil
			}, Epochs: func(_ context.Context, a string, epoch int64) error {
				require.Equal(t, account, a)
				require.Equal(t, int64(7), epoch)
				if tt.late {
					verifyNow = now.Add(time.Minute)
				}
				if tt.stale {
					return errors.New("revoked")
				}
				return nil
			}}
			if tt.noEpochs {
				v.Epochs = nil
			}
			if tt.tamper {
				other := uuid.New()
				r.PublicationOperationId = other[:]
			}
			invoke := method
			if tt.wrongMethod {
				invoke = storyv1.LfpConsentActionService_ObserveLfpConsentOffer_FullMethodName
			}
			got, e := v.Verify(ctx, r, invoke)
			require.Equal(t, tt.want, status.Code(e))
			if tt.want == codes.OK {
				require.Equal(t, account, got.Identity().AccountID.String())
				require.Equal(t, profile, got.Identity().ProfileID.String())
				require.Equal(t, tt.kind, got.Identity().AccountType)
				require.Equal(t, int64(7), got.Identity().SessionEpoch)
				require.NotEqual(t, [32]byte{}, got.RequestSHA256())
				actor, e := got.ActorMaterial()
				require.NoError(t, e)
				require.Equal(t, operation, actor.OperationID)
				require.Equal(t, got.Identity().AccountID, actor.AccountID)
				require.Equal(t, got.Identity().ProfileID, actor.ProfileID)
				require.Equal(t, int64(7), actor.SessionEpoch)
				require.Equal(t, lfpconsent.PublishOffer, actor.ActionKind)
				require.Equal(t, lfpconsent.HumanDelegatedUser, actor.ActorKind)
				wantType := uint64(lfpconsent.Regular)
				if tt.kind == "guest" {
					wantType = lfpconsent.Guest
				}
				require.Equal(t, wantType, actor.AccountType)
				require.Equal(t, got.RequestSHA256(), actor.PrincipalRequestSHA256)
				require.Equal(t, method, actor.FullRPC)
				require.Equal(t, "gateway", actor.Issuer)
				require.Equal(t, "story", actor.Audience)
				require.True(t, got.CredentialExpiry().After(now))
			} else {
				require.Equal(t, HumanAction{}, got)
			}
		})
	}
}

func TestZeroHumanActionCannotMintActor(t *testing.T) {
	actor, e := (HumanAction{}).ActorMaterial()
	require.Error(t, e)
	require.Equal(t, lfpconsent.Actor{}, actor)
}
