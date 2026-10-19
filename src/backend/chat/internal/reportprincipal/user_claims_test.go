package reportprincipal

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strings"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestReportPolicyServiceUserClaimsDenied(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	request := &chatv1.CheckMessageReadEntitlementRequest{ChatId: uuid.NewString(), ProfileId: uuid.NewString(), MessageCreatedAt: timestamppb.Now()}
	verifier := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
	for _, claim := range []string{"control", "account_id", "profile_id", "session_epoch"} {
		t.Run(claim, func(t *testing.T) {
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			operation := uuid.NewString()
			token, err := issuer.IssueService(principal.ServiceInput{Audience: "chat", RPC: Method, RequestID: operation, RequestHash: hash})
			require.NoError(t, err)
			if claim != "control" {
				parts := strings.Split(token, ".")
				payload, err := base64.RawURLEncoding.DecodeString(parts[1])
				require.NoError(t, err)
				var claims map[string]any
				require.NoError(t, json.Unmarshal(payload, &claims))
				claims[claim] = uuid.NewString()
				if claim == "session_epoch" {
					claims[claim] = 7
				}
				payload, err = json.Marshal(claims)
				require.NoError(t, err)
				parts[1] = base64.RawURLEncoding.EncodeToString(payload)
				digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
				signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
				require.NoError(t, err)
				parts[2] = base64.RawURLEncoding.EncodeToString(signature)
				token = strings.Join(parts, ".")
			}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation))
			reached := false
			_, err = verifier.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: Method}, func(context.Context, any) (any, error) { reached = true; return nil, nil })
			if claim == "control" {
				require.NoError(t, err)
				require.True(t, reached)
			} else {
				require.Equal(t, codes.Unauthenticated, status.Code(err))
				require.False(t, reached, "service route must reject user identity claims before handler")
			}
		})
	}
}
