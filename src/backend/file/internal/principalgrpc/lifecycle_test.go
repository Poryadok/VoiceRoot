package principalgrpc

import (
	"context"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

func TestLifecycleAndProducerRoutesRequireExactOwnerAndDenyOrdinaryListener(t *testing.T) {
	cases := []struct {
		method, issuer string
		request        proto.Message
	}{
		{filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, "space", &filev1.PrepareSpaceDeletionReferenceManifestRequest{}},
		{filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, "space", &filev1.ApplySpaceLifecycleFenceRequest{}},
		{filev1.FileService_PurgeSpace_FullMethodName, "space", &filev1.PurgeSpaceRequest{}},
		{filev1.FileService_GetSpacePurgeReceipt_FullMethodName, "space", &filev1.GetSpacePurgeReceiptRequest{}},
	}
	for ownerType, issuer := range map[filev1.FileReferenceOwnerType]string{
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE:        "messaging",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STICKER:        "messaging",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_GIF_ASSET:      "messaging",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STORY:          "user",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_PROFILE_AVATAR: "user",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_CHAT_AVATAR:    "chat",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_SPACE_AVATAR:   "space",
	} {
		cases = append(cases, struct {
			method, issuer string
			request        proto.Message
		}{filev1.FileService_IssueFileAccessCapability_FullMethodName, issuer, &filev1.IssueFileAccessCapabilityRequest{Reference: &filev1.FileReferenceKey{OwnerType: ownerType}}})
	}
	for producer, issuer := range map[filev1.FileReferenceProducerId]string{filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE: "space", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT: "chat", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING: "messaging"} {
		cases = append(cases, struct {
			method, issuer string
			request        proto.Message
		}{filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, issuer, &filev1.RegisterSpaceDeletionReferenceChunkRequest{ProducerId: producer}}, struct {
			method, issuer string
			request        proto.Message
		}{filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, issuer, &filev1.ReleaseSpaceDeletionProducerReferencesRequest{ProducerId: producer}})
	}
	for _, tc := range cases {
		t.Run(tc.method+"/"+tc.issuer, func(t *testing.T) {
			require.True(t, IsProtectedMethod(tc.method))
			calls := 0
			handler := func(ctx context.Context, _ any) (any, error) {
				calls++
				p, ok := principal.FromContext(ctx)
				require.True(t, ok)
				require.Equal(t, tc.issuer, p.Issuer)
				return nil, nil
			}
			_, err := OrdinaryUnaryInterceptor()(context.Background(), tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Zero(t, calls)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer fixture", "x-request-id", "request"))
			issuer := tc.issuer
			verifier := verifierFunc(func(_ context.Context, _ string, method, id, hash string) (principal.Principal, error) {
				return principal.Principal{Kind: "service", Issuer: issuer, Subject: "service:" + issuer, Audience: "file", RPC: method, RequestID: id, RequestHash: hash}, nil
			})
			_, err = StrictUnaryInterceptor(verifier)(ctx, tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			issuer = "untrusted"
			_, err = StrictUnaryInterceptor(verifier)(ctx, tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.Equal(t, 1, calls)
		})
	}
	request := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{}}
	request.Fence.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer fixture", "x-request-id", "request"))
	_, err := StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
		t.Fatal("unknown nested fields reached verifier")
		return principal.Principal{}, nil
	}))(ctx, request, &grpc.UnaryServerInfo{FullMethod: filev1.FileService_ApplySpaceLifecycleFence_FullMethodName}, func(context.Context, any) (any, error) {
		t.Fatal("unknown nested fields reached handler")
		return nil, nil
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
