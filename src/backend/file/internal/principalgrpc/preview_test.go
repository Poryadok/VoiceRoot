package principalgrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

func TestPreviewProtectedRouteExactMessagingPrincipalBoundary(t *testing.T) {
	request := &filev1.ResolveMessageAttachmentPreviewRequest{ProtocolVersion: 1, Reference: &filev1.FileReferenceKey{FileId: "00000000-0000-4000-8000-000000000001", OwnerId: "00000000-0000-4000-8000-000000000002", OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE}}
	method := filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	valid := principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "file", RPC: method, RequestID: "request-1", RequestHash: hash}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "request-1"))
	cases := map[string]func(*principal.Principal){
		"story":          func(p *principal.Principal) { p.Issuer = "story"; p.Subject = "service:story" },
		"gateway":        func(p *principal.Principal) { p.Issuer = "gateway"; p.Subject = "service:gateway" },
		"wrong_audience": func(p *principal.Principal) { p.Audience = "messaging" },
		"wrong_rpc":      func(p *principal.Principal) { p.RPC = filev1.FileService_GetFileMetadata_FullMethodName },
		"wrong_hash":     func(p *principal.Principal) { p.RequestHash = "wrong" },
		"wrong_request":  func(p *principal.Principal) { p.RequestID = "wrong" },
		"account":        func(p *principal.Principal) { p.AccountID = "raw" },
		"profile":        func(p *principal.Principal) { p.ProfileID = "raw" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := valid
			mutate(&p)
			interceptor := StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) { return p, nil }))
			called := false
			_, err := interceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			require.Error(t, err)
			require.False(t, called)
		})
	}
	called := false
	handler := func(context.Context, any) (any, error) { called = true; return nil, nil }
	_, err = OrdinaryUnaryInterceptor()(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
	interceptor := StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) { return valid, nil }))
	_, err = interceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	require.NoError(t, err)
	require.True(t, called)
	unknown := proto.Clone(request).(*filev1.ResolveMessageAttachmentPreviewRequest)
	unknown.Reference.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	called = false
	_, err = interceptor(ctx, unknown, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.False(t, called)
}
