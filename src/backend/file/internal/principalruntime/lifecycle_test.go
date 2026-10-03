package principalruntime

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

func (f *protectedFileFixture) lifecycleOwner(ctx context.Context, owner string) error {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != owner || p.Subject != "service:"+owner {
		return status.Error(codes.Internal, "missing verified lifecycle owner")
	}
	f.calls.Add(1)
	return nil
}

func (f *protectedFileFixture) PrepareSpaceDeletionReferenceManifest(ctx context.Context, _ *filev1.PrepareSpaceDeletionReferenceManifestRequest) (*filev1.PrepareSpaceDeletionReferenceManifestResponse, error) {
	return &filev1.PrepareSpaceDeletionReferenceManifestResponse{}, f.lifecycleOwner(ctx, "space")
}
func (f *protectedFileFixture) ApplySpaceLifecycleFence(ctx context.Context, _ *filev1.ApplySpaceLifecycleFenceRequest) (*filev1.ApplySpaceLifecycleFenceResponse, error) {
	return &filev1.ApplySpaceLifecycleFenceResponse{}, f.lifecycleOwner(ctx, "space")
}
func (f *protectedFileFixture) PurgeSpace(ctx context.Context, _ *filev1.PurgeSpaceRequest) (*filev1.PurgeSpaceResponse, error) {
	return &filev1.PurgeSpaceResponse{}, f.lifecycleOwner(ctx, "space")
}
func (f *protectedFileFixture) GetSpacePurgeReceipt(ctx context.Context, _ *filev1.GetSpacePurgeReceiptRequest) (*filev1.GetSpacePurgeReceiptResponse, error) {
	return &filev1.GetSpacePurgeReceiptResponse{}, f.lifecycleOwner(ctx, "space")
}
func (f *protectedFileFixture) IssueFileAccessCapability(ctx context.Context, req *filev1.IssueFileAccessCapabilityRequest) (*filev1.IssueFileAccessCapabilityResponse, error) {
	owner := map[filev1.FileReferenceOwnerType]string{
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE:        "messaging",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STICKER:        "messaging",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_GIF_ASSET:      "messaging",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STORY:          "user",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_PROFILE_AVATAR: "user",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_CHAT_AVATAR:    "chat",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_SPACE_AVATAR:   "space",
	}[req.GetReference().GetOwnerType()]
	return &filev1.IssueFileAccessCapabilityResponse{}, f.lifecycleOwner(ctx, owner)
}
func fixtureProducerOwner(id filev1.FileReferenceProducerId) string {
	return map[filev1.FileReferenceProducerId]string{filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE: "space", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT: "chat", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING: "messaging"}[id]
}
func (f *protectedFileFixture) RegisterSpaceDeletionReferenceChunk(ctx context.Context, req *filev1.RegisterSpaceDeletionReferenceChunkRequest) (*filev1.RegisterSpaceDeletionReferenceChunkResponse, error) {
	return &filev1.RegisterSpaceDeletionReferenceChunkResponse{}, f.lifecycleOwner(ctx, fixtureProducerOwner(req.ProducerId))
}
func (f *protectedFileFixture) ReleaseSpaceDeletionProducerReferences(ctx context.Context, req *filev1.ReleaseSpaceDeletionProducerReferencesRequest) (*filev1.ReleaseSpaceDeletionProducerReferencesResponse, error) {
	return &filev1.ReleaseSpaceDeletionProducerReferencesResponse{}, f.lifecycleOwner(ctx, fixtureProducerOwner(req.ProducerId))
}

func TestFileLifecycleRoutesAuthenticateExactOwnerThroughMTLS(t *testing.T) {
	f := newTransportFixture(t, false)
	cases := []struct {
		method, owner     string
		request, response proto.Message
	}{
		{filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, "space", &filev1.PrepareSpaceDeletionReferenceManifestRequest{}, &filev1.PrepareSpaceDeletionReferenceManifestResponse{}},
		{filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, "space", &filev1.ApplySpaceLifecycleFenceRequest{}, &filev1.ApplySpaceLifecycleFenceResponse{}},
		{filev1.FileService_PurgeSpace_FullMethodName, "space", &filev1.PurgeSpaceRequest{}, &filev1.PurgeSpaceResponse{}},
		{filev1.FileService_GetSpacePurgeReceipt_FullMethodName, "space", &filev1.GetSpacePurgeReceiptRequest{}, &filev1.GetSpacePurgeReceiptResponse{}},
	}
	for ownerType, owner := range map[filev1.FileReferenceOwnerType]string{
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE:        "messaging",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STICKER:        "messaging",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_GIF_ASSET:      "messaging",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STORY:          "user",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_PROFILE_AVATAR: "user",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_CHAT_AVATAR:    "chat",
		filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_SPACE_AVATAR:   "space",
	} {
		cases = append(cases, struct {
			method, owner     string
			request, response proto.Message
		}{filev1.FileService_IssueFileAccessCapability_FullMethodName, owner, &filev1.IssueFileAccessCapabilityRequest{Reference: &filev1.FileReferenceKey{OwnerType: ownerType}}, &filev1.IssueFileAccessCapabilityResponse{}})
	}
	for producer, owner := range map[filev1.FileReferenceProducerId]string{filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE: "space", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT: "chat", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING: "messaging"} {
		cases = append(cases, struct {
			method, owner     string
			request, response proto.Message
		}{filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, owner, &filev1.RegisterSpaceDeletionReferenceChunkRequest{ProducerId: producer}, &filev1.RegisterSpaceDeletionReferenceChunkResponse{}}, struct {
			method, owner     string
			request, response proto.Message
		}{filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, owner, &filev1.ReleaseSpaceDeletionProducerReferencesRequest{ProducerId: producer}, &filev1.ReleaseSpaceDeletionProducerReferencesResponse{}})
	}
	for _, tc := range cases {
		t.Run(tc.method+"/"+tc.owner, func(t *testing.T) {
			hash, err := principal.RequestHash(tc.request)
			require.NoError(t, err)
			invoke := func(owner string) error {
				issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: owner, KeyID: "current", PrivateKey: f.active})
				require.NoError(t, err)
				id := uuid.NewString()
				token, err := issuer.IssueService(principal.ServiceInput{Audience: "file", RPC: tc.method, RequestID: id, RequestHash: hash})
				require.NoError(t, err)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return f.connection.Invoke(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), tc.method, tc.request, tc.response)
			}
			before := f.service.calls.Load()
			require.NoError(t, invoke(tc.owner))
			require.Equal(t, before+1, f.service.calls.Load())
			wrong := "story"
			require.Equal(t, codes.PermissionDenied, status.Code(invoke(wrong)))
			require.Equal(t, before+1, f.service.calls.Load())
		})
	}
}
