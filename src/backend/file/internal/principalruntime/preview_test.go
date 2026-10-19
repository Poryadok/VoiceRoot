package principalruntime

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

func (f *protectedFileFixture) ResolveMessageAttachmentPreview(ctx context.Context, _ *filev1.ResolveMessageAttachmentPreviewRequest) (*filev1.ResolveMessageAttachmentPreviewResponse, error) {
	return &filev1.ResolveMessageAttachmentPreviewResponse{}, f.lifecycleOwner(ctx, "messaging")
}

func TestPreviewTransportExactBindingReplayAndAuthorityOutage(t *testing.T) {
	f := newTransportFixture(t, false)
	request := &filev1.ResolveMessageAttachmentPreviewRequest{ProtocolVersion: 1, Reference: &filev1.FileReferenceKey{FileId: "00000000-0000-4000-8000-000000000001", OwnerId: "00000000-0000-4000-8000-000000000002", OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE}}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	token := func(mutate func(map[string]any)) string {
		return signedCredential(t, f.active, "current", func(c map[string]any) {
			c["iss"], c["sub"] = "messaging", "service:messaging"
			c["rpc"], c["request_hash"] = filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName, hash
			if mutate != nil {
				mutate(c)
			}
		})
	}
	call := func(raw string, req *filev1.ResolveMessageAttachmentPreviewRequest, extra ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		pairs := append([]string{"authorization", "Bearer " + raw, "x-request-id", "request-1"}, extra...)
		_, e := f.client.ResolveMessageAttachmentPreview(metadata.NewOutgoingContext(ctx, metadata.Pairs(pairs...)), req)
		return e
	}
	first := token(nil)
	require.NoError(t, call(first, request))
	require.Equal(t, codes.Unauthenticated, status.Code(call(first, request)))
	for _, owner := range []string{"story", "chat", "gateway"} {
		require.Equal(t, codes.PermissionDenied, status.Code(call(token(func(c map[string]any) { c["iss"], c["sub"] = owner, "service:"+owner }), request)))
	}
	for _, mutate := range []func(map[string]any){
		func(c map[string]any) { c["aud"] = "story" },
		func(c map[string]any) { c["rpc"] = filev1.FileService_AcquireFileReferences_FullMethodName },
		func(c map[string]any) { c["request_id"] = "wrong" },
		func(c map[string]any) { c["request_hash"] = "wrong" },
	} {
		require.Equal(t, codes.Unauthenticated, status.Code(call(token(mutate), request)))
	}
	changed := proto.Clone(request).(*filev1.ResolveMessageAttachmentPreviewRequest)
	changed.Reference.OwnerId = "00000000-0000-4000-8000-000000000003"
	require.Equal(t, codes.Unauthenticated, status.Code(call(token(nil), changed)))
	require.Equal(t, codes.Unauthenticated, status.Code(call(token(nil), request, "x-profile-id", "forged")))
	require.Equal(t, codes.Unauthenticated, status.Code(call(token(nil), request, "x-request-id", "duplicate")))
	unknown := proto.Clone(request).(*filev1.ResolveMessageAttachmentPreviewRequest)
	unknown.Reference.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	require.Equal(t, codes.InvalidArgument, status.Code(call(token(nil), unknown)))
	f.redis.Close()
	require.Equal(t, codes.Unauthenticated, status.Code(call(token(nil), request)))
	require.Equal(t, int32(1), f.service.calls.Load(), "all denials precede handler/store")
	// A fresh verifier has no cached authority to use during JWKS outage.
	f = newTransportFixture(t, false)
	f.jwksStatus.Store(http.StatusServiceUnavailable)
	require.Equal(t, codes.Unauthenticated, status.Code(call(token(nil), request)))
	require.Zero(t, f.service.calls.Load())
}
