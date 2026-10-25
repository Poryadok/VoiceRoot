package botsendprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type realVerifier struct {
	key  *rsa.PublicKey
	seen map[string]bool
}

func (v *realVerifier) Verify(ctx context.Context, token, rpc, id, hash string) (principal.Principal, error) {
	return principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "messaging", ExpectedRPC: rpc, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return v.key, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
		if v.seen[jti] {
			return errors.New("replay")
		}
		v.seen[jti] = true
		return nil
	}})
}
func TestExactBotSendProofAndFiniteAttempt(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	body := &messagingv1.SendBotMessageRequest{Message: &messagingv1.SendMessageRequest{Content: "hello"}, Binding: &commonv1.BotMessageSendBinding{BotId: uuid.NewString()}}
	rpc := messagingv1.MessagingService_SendBotMessage_FullMethodName
	sign := func(request *messagingv1.SendBotMessageRequest) context.Context {
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		id := uuid.NewString()
		token, e := issuer.IssueService(principal.ServiceInput{Audience: "messaging", RPC: rpc, RequestID: id, RequestHash: hash})
		require.NoError(t, e)
		return metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	interceptor := UnaryServerInterceptor(&realVerifier{key: &key.PublicKey, seen: map[string]bool{}})
	call := sign(body)
	original := proto.Clone(body).(*messagingv1.SendBotMessageRequest)
	var retained context.Context
	_, err = interceptor(call, body, &grpc.UnaryServerInfo{FullMethod: rpc}, func(ctx context.Context, value any) (any, error) {
		retained = ctx
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 5*time.Second)
		request := value.(*messagingv1.SendBotMessageRequest)
		proof, e := Require(ctx, request)
		require.NoError(t, e)
		body.Message.Content = "mutated caller memory"
		require.NoError(t, proof.Validate(request))
		request.Message.Content = "changed signed message"
		require.Error(t, proof.Validate(request))
		return nil, nil
	})
	require.NoError(t, err)
	require.Error(t, retained.Err())
	_, err = interceptor(call, original, &grpc.UnaryServerInfo{FullMethod: rpc}, func(context.Context, any) (any, error) { t.Fatal("replay admitted"); return nil, nil })
	require.Error(t, err)
	for _, kind := range []string{"body", "nested_unknown", "binding_unknown", "mixed", "ordinary"} {
		t.Run(kind, func(t *testing.T) {
			request := proto.Clone(body).(*messagingv1.SendBotMessageRequest)
			ctx := sign(request)
			method := rpc
			switch kind {
			case "body":
				request.Message.Content = "unsigned change"
			case "nested_unknown":
				request.Message.ProtoReflect().SetUnknown([]byte{0x80, 0x06, 1})
			case "binding_unknown":
				request.Binding.ProtoReflect().SetUnknown([]byte{0x80, 0x06, 1})
			case "mixed":
				md, _ := metadata.FromIncomingContext(ctx)
				md = md.Copy()
				md.Set("x-voice-profile-id", uuid.NewString())
				ctx = metadata.NewIncomingContext(ctx, md)
			case "ordinary":
				method = messagingv1.MessagingService_SendMessage_FullMethodName
			}
			_, e := interceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { t.Fatal("untrusted request admitted"); return nil, nil })
			require.Error(t, e)
		})
	}
	_, err = Require(context.Background(), body)
	require.Error(t, err)
}
