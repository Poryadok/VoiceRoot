package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"testing"
	"time"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
)

func TestAutoModCapability_ExactMethodCallerAndMTLS(t *testing.T) {
	require.NotEmpty(t, Method("automod-evidence"))
	require.True(t, RequiresMutualTLS("automod-evidence"))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	verifier := &Verifier{Target: "moderation", Capability: "automod-evidence", Issuers: map[string]bool{"messaging": true, "auth": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
	methods := []string{automodv1.ModerationEvidenceService_ProvisionSubject_FullMethodName, automodv1.ModerationEvidenceService_IngestCommittedMessageEvidence_FullMethodName, automodv1.ModerationEvidenceService_GetAccountSendRestriction_FullMethodName, automodv1.ModerationEvidenceService_GetEffectiveRestrictionSnapshot_FullMethodName, automodv1.AccountLifecycleReceiverService_ApplyLifecycleFact_FullMethodName, "/voice.moderation.v1.ModerationService/CheckMessage"}
	for _, caller := range []string{"messaging", "auth"} {
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: caller, KeyID: "current", PrivateKey: key})
		require.NoError(t, err)
		for _, method := range methods {
			request := &emptypb.Empty{}
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			id := uuid.NewString()
			token, err := issuer.IssueService(principal.ServiceInput{Audience: "moderation", RPC: method, RequestID: id, RequestHash: hash})
			require.NoError(t, err)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
			called := false
			_, err = StrictUnaryInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(verified context.Context, request any) (any, error) {
				called = true
				return nil, RequireAutoModEvidence(verified, method, request.(*emptypb.Empty))
			})
			authMethod := method == automodv1.ModerationEvidenceService_GetEffectiveRestrictionSnapshot_FullMethodName || method == automodv1.AccountLifecycleReceiverService_ApplyLifecycleFact_FullMethodName
			allowed := caller == "auth" && authMethod || caller == "messaging" && !authMethod && method != "/voice.moderation.v1.ModerationService/CheckMessage"
			require.Equal(t, allowed, called, "caller=%s method=%s error=%v", caller, method, err)
			require.Equal(t, allowed, err == nil)
		}
	}
}
