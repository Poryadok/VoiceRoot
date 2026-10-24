package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestAuditInitiatingContext_UsesVerifiedExplicitActorAndFreshCredential(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	claims := tokenClaims{UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 5, ExpiresAt: time.Now().Add(time.Minute), AccountType: "guest", AuthenticatedAccountType: "guest"}
	r := httptest.NewRequest("POST", "/api/v1/chats", nil)
	r = r.WithContext(context.WithValue(r.Context(), verifiedUserClaimsKey{}, claims))
	r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
	name := "group"
	req := &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, RequestId: uuid.NewString(), Name: &name}
	transcoder := &transcoder{lifecycleIssuer: issuer, clients: grpcClients{chatAudit: chatv1.NewChatServiceClient(nil)}}
	var tokens []string
	for range 2 {
		ctx, err := transcoder.auditInitiatingContext(r, req, chatv1.ChatService_CreateChat_FullMethodName, req.RequestId)
		require.NoError(t, err)
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md, 2)
		require.Equal(t, []string{req.RequestId}, md.Get("x-request-id"))
		require.Len(t, md.Get("authorization"), 1)
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		verified, err := principal.VerifyDelegatedUser(ctx, token, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "chat", ExpectedRPC: chatv1.ChatService_CreateChat_FullMethodName, ExpectedRequestID: req.RequestId, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, SessionEpochChecker: func(_ context.Context, account string, epoch int64) error {
			require.Equal(t, claims.UserID, account)
			require.Equal(t, claims.SessionEpoch, epoch)
			return nil
		}})
		require.NoError(t, err)
		require.Equal(t, claims.ProfileID, verified.ProfileID)
		require.Equal(t, "guest", verified.AccountType)
		tokens = append(tokens, token)
	}
	require.NotEqual(t, tokens[0], tokens[1])
}

func TestAuditInitiatingContext_NoRawIdentityOrLegacyFallback(t *testing.T) {
	name := "group"
	req := &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, Name: &name}
	for name, claims := range map[string]*tokenClaims{
		"raw headers only":     nil,
		"missing account type": {UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1, ExpiresAt: time.Now().Add(time.Minute)},
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/v1/chats", nil)
			r.Header.Set("X-Voice-User-Id", uuid.NewString())
			r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
			r.Header.Set("X-Voice-Account-Type", "regular")
			if claims != nil {
				r = r.WithContext(context.WithValue(r.Context(), verifiedUserClaimsKey{}, *claims))
			}
			transcoder := &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(nil)}}
			ctx, err := transcoder.auditInitiatingContext(r, req, chatv1.ChatService_CreateChat_FullMethodName, uuid.NewString())
			require.Nil(t, ctx)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}
