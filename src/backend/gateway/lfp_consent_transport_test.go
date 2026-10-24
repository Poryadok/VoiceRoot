package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
)

func TestStoryConsentUsesExactAuthenticatedActorType(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
	require.NoError(t, e)
	method := storyv1.LfpConsentActionService_PublishLfpConsentOffer_FullMethodName
	op, story := uuid.New(), uuid.New()
	request := &storyv1.PublishLfpConsentOfferRequest{ProtocolVersion: 1, PublicationOperationId: op[:], StoryId: story[:]}
	tests := []struct {
		name, exact, compat          string
		noContext, noSigner, expired bool
		want                         codes.Code
	}{
		{name: "regular", exact: "regular", compat: "regular", want: codes.OK}, {name: "guest preserves exact type", exact: "guest", compat: "regular", want: codes.OK},
		{name: "legacy default is not proof", compat: "regular", want: codes.PermissionDenied}, {name: "SDK cannot become human", exact: "sdk-account", compat: "regular", want: codes.PermissionDenied},
		{name: "raw headers", exact: "regular", noContext: true, want: codes.Unauthenticated}, {name: "missing signer", exact: "guest", noSigner: true, want: codes.Unavailable},
		{name: "expired session", exact: "regular", expired: true, want: codes.Unauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := tokenClaims{UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: tt.compat, AuthenticatedAccountType: tt.exact, ExpiresAt: time.Now().Add(time.Minute)}
			if tt.expired {
				claims.ExpiresAt = time.Now().Add(-time.Minute)
			}
			r := httptest.NewRequest("POST", "http://gateway/api/v1/stories/fixture/lfp-offer", nil)
			r.Header.Set("X-Voice-Profile-Id", claims.ProfileID)
			r.Header.Set("X-Voice-Account-Type", "regular")
			ctx := metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer original", "x-profile-id", "forged"))
			if !tt.noContext {
				ctx = context.WithValue(ctx, verifiedUserClaimsKey{}, claims)
			}
			r = r.WithContext(ctx)
			transcoder := &transcoder{lifecycleIssuer: signer}
			if tt.noSigner {
				transcoder.lifecycleIssuer = nil
			}
			out, e := transcoder.storyConsentContext(r, request, method)
			require.Equal(t, tt.want, status.Code(e))
			if tt.want != codes.OK {
				require.Nil(t, out)
				return
			}
			md, ok := metadata.FromOutgoingContext(out)
			require.True(t, ok)
			require.Len(t, md.Get("authorization"), 1)
			require.Len(t, md.Get("x-request-id"), 1)
			require.Empty(t, md.Get("x-profile-id"))
			require.Empty(t, md.Get("x-voice-account-type"))
			h, e := principal.RequestHash(request)
			require.NoError(t, e)
			p, e := principal.VerifyDelegatedUser(out, md.Get("authorization")[0][7:], principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "story", ExpectedRPC: method, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: h, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, SessionEpochChecker: func(_ context.Context, a string, epoch int64) error {
				require.Equal(t, claims.UserID, a)
				require.Equal(t, int64(7), epoch)
				return nil
			}})
			require.NoError(t, e)
			require.Equal(t, claims.UserID, p.AccountID)
			require.Equal(t, claims.ProfileID, p.ProfileID)
			require.Equal(t, tt.exact, p.AccountType)
			require.NotEmpty(t, p.JWTID)
		})
	}
}
