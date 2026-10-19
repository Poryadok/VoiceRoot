package lifecycleprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/authctx"
)

func TestAuditReadRequiresProtectedListener(t *testing.T) {
	_, err := OrdinaryUnary()(context.Background(), &spacev1.GetAuditLogRequest{}, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetAuditLog_FullMethodName}, func(context.Context, any) (any, error) { t.Fatal("ordinary audit bypass"); return nil, nil })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestAuditReadDelegatedPrincipalRejectsTamperingAndRevocation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, profile, space, requestID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, mode := range []string{"valid", "changed filter", "changed space", "changed cursor", "raw identity", "revoked", "replay", "wrong rpc", "missing dependencies"} {
		t.Run(mode, func(t *testing.T) {
			action := "role_created"
			req := &spacev1.GetAuditLogRequest{SpaceId: space, Action: &action, Page: &commonv1.CursorPageRequest{PageSize: 2, Cursor: "signed"}}
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			method := spacev1.SpaceService_GetAuditLog_FullMethodName
			if mode == "wrong rpc" {
				method = spacev1.SpaceService_GetSpace_FullMethodName
			}
			token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: requestID, RequestHash: hash, AccountID: account, ProfileID: profile, SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
			require.NoError(t, err)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)
			switch mode {
			case "changed filter":
				action = "role_deleted"
			case "changed space":
				req.SpaceId = uuid.NewString()
			case "changed cursor":
				req.Page.Cursor = "other"
			case "raw identity":
				md.Set(authctx.HeaderProfileID, profile)
			}
			verifier := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error {
				if mode == "replay" {
					return errors.New("replayed")
				}
				return nil
			}, Epoch: func(context.Context, string, int64) error {
				if mode == "revoked" {
					return errors.New("revoked")
				}
				return nil
			}}
			if mode == "missing dependencies" {
				verifier.Keys = nil
			}
			called := false
			_, err = verifier.Unary()(metadata.NewIncomingContext(context.Background(), md), req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetAuditLog_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
				called = true
				incoming, _ := metadata.FromIncomingContext(ctx)
				for name := range incoming {
					require.NotContains(t, name, "x-voice-", "audit must never synthesize raw identity")
				}
				id, ok := principal.FromContext(ctx)
				require.True(t, ok)
				require.Equal(t, profile, id.ProfileID)
				return nil, nil
			})
			if mode == "valid" {
				require.NoError(t, err)
				require.True(t, called)
			} else {
				require.Equal(t, codes.Unauthenticated, status.Code(err))
				require.False(t, called)
			}
		})
	}
}
