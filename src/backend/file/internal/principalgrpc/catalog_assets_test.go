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
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

func TestCatalogAssetRoutesRequireExactProtectedServiceAndRequest(t *testing.T) {
	for _, tc := range []struct {
		method, issuer string
		request        proto.Message
	}{
		{filev1.CatalogAssetService_ImportCatalogAsset_FullMethodName, "chat", &filev1.ImportCatalogAssetRequest{ProtocolVersion: 1, DestinationChat: &chatv1.ChatRef{Id: "00000000-0000-4000-8000-000000000001"}}},
		{filev1.CatalogAssetService_ValidateCatalogAsset_FullMethodName, "messaging", &filev1.ValidateCatalogAssetRequest{ProtocolVersion: 1}},
		{filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, "messaging", &filev1.LookupCatalogAssetRequest{ProtocolVersion: 1}},
		{filev1.CatalogAssetService_GetCatalogAssetStatus_FullMethodName, "chat", &filev1.GetCatalogAssetStatusRequest{ProtocolVersion: 1}},
	} {
		t.Run(tc.issuer, func(t *testing.T) {
			hash, err := principal.RequestHash(tc.request)
			require.NoError(t, err)
			valid := principal.Principal{Kind: "service", Issuer: tc.issuer, Subject: "service:" + tc.issuer, Audience: "file", RPC: tc.method, RequestID: "request-1", RequestHash: hash}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "request-1"))
			called := false
			handler := func(ctx context.Context, request any) (any, error) { called = true; return nil, nil }
			_, err = OrdinaryUnaryInterceptor()(ctx, tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.False(t, called)
			interceptor := StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) { return valid, nil }))
			_, err = interceptor(ctx, tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)
			require.NoError(t, err)
			require.True(t, called)
			for name, mutate := range map[string]func(*principal.Principal){
				"wrong-service":     func(p *principal.Principal) { p.Issuer = "gateway"; p.Subject = "service:gateway" },
				"wrong-subject":     func(p *principal.Principal) { p.Subject = "service:space" },
				"delegated-profile": func(p *principal.Principal) { p.ProfileID = "body-profile" },
				"delegated-account": func(p *principal.Principal) { p.AccountID = "body-account" },
				"wrong-rpc":         func(p *principal.Principal) { p.RPC = filev1.FileService_GetBulkMetadata_FullMethodName },
				"wrong-audience":    func(p *principal.Principal) { p.Audience = "chat" },
				"wrong-hash":        func(p *principal.Principal) { p.RequestHash = "altered" },
				"wrong-request-id":  func(p *principal.Principal) { p.RequestID = "altered" },
			} {
				t.Run(name, func(t *testing.T) {
					p := valid
					mutate(&p)
					called = false
					deny := StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) { return p, nil }))
					_, err := deny(ctx, tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)
					require.Error(t, err)
					require.False(t, called)
				})
			}
			for _, pairs := range [][]string{nil, {"x-profile-id", "forged"}, {"authorization", "Bearer token", "authorization", "Bearer other", "x-request-id", "request-1"}, {"authorization", "Bearer token", "x-request-id", "request-1", "x-request-id", "request-2"}} {
				called = false
				_, err := interceptor(metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...)), tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)
				require.Error(t, err)
				require.False(t, called)
			}
			unknown := proto.Clone(tc.request)
			unknown.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			called = false
			_, err = interceptor(ctx, unknown, &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.False(t, called)
		})
	}
}
