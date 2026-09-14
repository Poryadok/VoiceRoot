package principalgrpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
)

type verifierFunc func(context.Context, string, string, string, string) (principal.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token, method, id, hash string) (principal.Principal, error) {
	return f(ctx, token, method, id, hash)
}

func TestOrdinaryListenerRejectsProtectedStoryMediaBeforeHandler(t *testing.T) {
	called := 0
	handler := func(context.Context, any) (any, error) { called++; return nil, nil }
	i := OrdinaryUnaryInterceptor()
	require.True(t, IsProtectedMethod(filev1.FileService_ValidateStoryMedia_FullMethodName))
	_, err := i(context.Background(), &filev1.ValidateStoryMediaRequest{}, &grpc.UnaryServerInfo{FullMethod: filev1.FileService_ValidateStoryMedia_FullMethodName}, handler)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, called)
	require.False(t, IsProtectedMethod(filev1.FileService_GetFileMetadata_FullMethodName))
	_, err = i(context.Background(), &filev1.GetFileMetadataRequest{}, &grpc.UnaryServerInfo{FullMethod: filev1.FileService_GetFileMetadata_FullMethodName}, handler)
	require.NoError(t, err)
	require.Equal(t, 1, called)
}

func TestStrictListenerRejectsMetadataBeforeVerifierOrHandler(t *testing.T) {
	cases := map[string]metadata.MD{
		"missing":                 {},
		"missing request id":      metadata.Pairs("authorization", "Bearer token"),
		"missing authorization":   metadata.Pairs("x-request-id", "request-1"),
		"duplicate authorization": metadata.Pairs("authorization", "Bearer token", "authorization", "Bearer token", "x-request-id", "request-1"),
		"duplicate request id":    metadata.Pairs("authorization", "Bearer token", "x-request-id", "request-1", "x-request-id", "request-1"),
		"non bearer":              metadata.Pairs("authorization", "Basic token", "x-request-id", "request-1"),
		"empty request id":        metadata.Pairs("authorization", "Bearer token", "x-request-id", ""),
		"ambiguous request id":    metadata.Pairs("authorization", "Bearer token", "x-request-id", " request-1"),
	}
	for _, key := range []string{"x-voice-profile-id", "x-voice-user-id", "x-voice-internal-caller", "x-voice-subscription", "x-voice-role", "x-voice-anything", "x-profile-id", "x-account-id", "x-user-id", "x-actor-id", "x-internal-caller"} {
		cases[key] = metadata.Pairs("authorization", "Bearer token", "x-request-id", "request-1", key, "story")
	}
	for name, md := range cases {
		t.Run(name, func(t *testing.T) {
			verifierCalls, handlerCalls := 0, 0
			i := StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
				verifierCalls++
				return principal.Principal{}, nil
			}))
			_, err := i(metadata.NewIncomingContext(context.Background(), md), &filev1.ValidateStoryMediaRequest{}, &grpc.UnaryServerInfo{FullMethod: filev1.FileService_ValidateStoryMedia_FullMethodName}, func(context.Context, any) (any, error) { handlerCalls++; return nil, nil })
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.Zero(t, verifierCalls)
			require.Zero(t, handlerCalls)
		})
	}
}

func TestStrictListenerBindsRequestAndPublishesVerifiedPrincipal(t *testing.T) {
	req := &filev1.ValidateStoryMediaRequest{FileId: "7f1e69c1-0dfa-49fc-a356-980e128d686d", AuthorProfileId: "d097d722-6c61-4565-9483-65db9bc7909f", ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	expected := principal.Principal{Kind: "service", Issuer: "story", Subject: "service:story", Audience: "file", RPC: filev1.FileService_ValidateStoryMedia_FullMethodName, RequestID: "request-1", RequestHash: hash}
	i := StrictUnaryInterceptor(verifierFunc(func(_ context.Context, token, method, id, gotHash string) (principal.Principal, error) {
		require.Equal(t, "token", token)
		require.Equal(t, expected.RPC, method)
		require.Equal(t, expected.RequestID, id)
		require.Equal(t, hash, gotHash)
		return expected, nil
	}))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "request-1", "traceparent", "non-authority-tracing"))
	called := 0
	_, err = i(ctx, req, &grpc.UnaryServerInfo{FullMethod: expected.RPC}, func(ctx context.Context, got any) (any, error) {
		called++
		p, ok := principal.FromContext(ctx)
		require.True(t, ok)
		require.Equal(t, expected, p)
		require.True(t, proto.Equal(req, got.(proto.Message)))
		return nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, called)
}

func TestStrictListenerRejectsWrongRouteUnknownFieldsAndVerifierFailure(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "request-1"))
	handler := func(context.Context, any) (any, error) { t.Fatal("rejected request reached handler"); return nil, nil }
	verifier := verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
		t.Fatal("invalid route or request reached verifier")
		return principal.Principal{}, nil
	})
	_, err := StrictUnaryInterceptor(verifier)(ctx, &filev1.GetFileMetadataRequest{}, &grpc.UnaryServerInfo{FullMethod: filev1.FileService_GetFileMetadata_FullMethodName}, handler)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	req := &filev1.ValidateStoryMediaRequest{}
	req.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	_, err = StrictUnaryInterceptor(verifier)(ctx, req, &grpc.UnaryServerInfo{FullMethod: filev1.FileService_ValidateStoryMedia_FullMethodName}, handler)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	for _, v := range []Verifier{nil, verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
		return principal.Principal{}, errors.New("JWKS or Redis unavailable")
	})} {
		_, err = StrictUnaryInterceptor(v)(ctx, &filev1.ValidateStoryMediaRequest{}, &grpc.UnaryServerInfo{FullMethod: filev1.FileService_ValidateStoryMedia_FullMethodName}, handler)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
}
