package botmembershipprincipal

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
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func installationRequest(remove bool) (proto.Message, string) {
	intent := &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: uuid.NewString(), ActorProfileId: uuid.NewString(), Generation: 1}
	if remove {
		return &spacev1.RemoveBotMemberRequest{SpaceId: intent.SpaceId, ProfileId: intent.ActorProfileId, InstallationIntent: intent, InstallOperationId: uuid.NewString()}, spacev1.SpaceService_RemoveBotMember_FullMethodName
	}
	return &spacev1.AddBotMemberRequest{SpaceId: intent.SpaceId, ProfileId: intent.ActorProfileId, InstallationIntent: intent}, spacev1.SpaceService_AddBotMember_FullMethodName
}

func TestInstallationPrincipalRejectsUnboundCalls(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	for _, name := range []string{"changed-body", "unknown-nested", "wrong-top-level", "request-id", "no-replay", "replay", "wrong-method", "raw-context"} {
		t.Run(name, func(t *testing.T) {
			request, method := installationRequest(false)
			req := request.(*spacev1.AddBotMemberRequest)
			ctx := signInstallation(t, key, now, request, method)
			v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
			switch name {
			case "changed-body":
				req.InstallationIntent.BotId = uuid.NewString()
			case "unknown-nested":
				req.InstallationIntent.ProtoReflect().SetUnknown([]byte{0x78, 0x01})
			case "wrong-top-level":
				req.SpaceId = uuid.NewString()
			case "request-id":
				md, _ := metadata.FromIncomingContext(ctx)
				md = md.Copy()
				md.Set("x-request-id", uuid.NewString())
				ctx = metadata.NewIncomingContext(ctx, md)
			case "no-replay":
				v.Replay = nil
			case "replay":
				v.Replay = func(context.Context, string, string, time.Time) error { return errors.New("replayed") }
			case "wrong-method":
				method = spacev1.SpaceService_CreateSpace_FullMethodName
			case "raw-context":
				_, err := RequireAdd(principal.WithVerified(context.Background(), principal.Principal{Issuer: "bot"}), req)
				require.Error(t, err)
				return
			}
			called := false
			_, err := v.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			require.Error(t, err)
			require.False(t, called)
		})
	}
}

func TestInstallationCapabilityExpiresAndOrdinaryListenerDenies(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	request, method := installationRequest(false)
	ctx := signInstallation(t, key, now, request, method)
	v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	_, err = v.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		proof, err := RequireAdd(ctx, request.(*spacev1.AddBotMemberRequest))
		require.NoError(t, err)
		now = now.Add(time.Minute)
		_, err = proof.Check(request)
		require.Error(t, err)
		return nil, nil
	})
	require.NoError(t, err)
	_, err = OrdinaryUnary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
		t.Fatal("ordinary listener admitted Bot mutation")
		return nil, nil
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func signInstallation(t *testing.T, key *rsa.PrivateKey, now time.Time, request proto.Message, method string) context.Context {
	t.Helper()
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	var operation string
	switch req := request.(type) {
	case *spacev1.AddBotMemberRequest:
		operation = req.InstallationIntent.OperationId
	case *spacev1.RemoveBotMemberRequest:
		operation = req.InstallationIntent.OperationId
	}
	token, err := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: hash})
	require.NoError(t, err)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation))
}

func TestInstallationPrincipalAuthenticatesExactAddAndCancel(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	v := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	for _, remove := range []bool{false, true} {
		request, method := installationRequest(remove)
		called := false
		_, err := v.Unary()(signInstallation(t, key, now, request, method), request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, value any) (any, error) {
			called = true
			var proof *VerifiedInstallation
			var err error
			if remove {
				proof, err = RequireRemove(ctx, request.(*spacev1.RemoveBotMemberRequest))
			} else {
				proof, err = RequireAdd(ctx, request.(*spacev1.AddBotMemberRequest))
			}
			require.NoError(t, err)
			binding, err := proof.Check(request)
			require.NoError(t, err)
			require.Equal(t, remove, binding.Remove)
			require.EqualValues(t, 1, binding.Generation)
			changed := proto.Clone(request)
			if remove {
				changed.(*spacev1.RemoveBotMemberRequest).InstallationIntent.InstallationId = uuid.NewString()
			} else {
				changed.(*spacev1.AddBotMemberRequest).InstallationIntent.InstallationId = uuid.NewString()
			}
			_, err = proof.Check(changed)
			require.Error(t, err)
			return nil, nil
		})
		require.NoError(t, err)
		require.True(t, called)
	}
}
