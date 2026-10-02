package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestValidateSpacePurgeManifestImportPageRequiresExactPageBinding(t *testing.T) {
	request := validSpacePurgeManifestPageRequest()
	require.NoError(t, sealSpacePurgeManifestPage(request.GetPage()))
	require.NoError(t, validateSpacePurgeManifestPageRequest(request))

	tampered := proto.Clone(request).(*messagingv1.ImportSpacePurgeManifestPageRequest)
	tampered.Page.ItemIds[0] = "20000000-0000-4000-8000-000000000099"
	require.Error(t, validateSpacePurgeManifestPageRequest(tampered), "page bytes must match the page hash")

	badID := proto.Clone(request).(*messagingv1.ImportSpacePurgeManifestPageRequest)
	badID.Page.ItemIds[0] = "not-a-canonical-uuid"
	require.Error(t, validateSpacePurgeManifestPageRequest(badID), "manifest IDs must be canonical UUIDs")

	unknownPage := proto.Clone(request).(*messagingv1.ImportSpacePurgeManifestPageRequest)
	unknownPage.Page.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 100, protowire.VarintType), 7))
	require.NoError(t, sealSpacePurgeManifestPage(unknownPage.GetPage()))
	require.NoError(t, validateSpacePurgeManifestPageRequest(unknownPage), "page unknown fields are preserved as immutable evidence")

	unknownBinding := proto.Clone(request).(*messagingv1.ImportSpacePurgeManifestPageRequest)
	unknownBinding.Page.Manifest.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 100, protowire.VarintType), 7))
	require.Error(t, validateSpacePurgeManifestPageRequest(unknownBinding), "the known root binding rejects unknown fields")
}

type spaceManifestPageImporterFunc func(context.Context, store.SpacePurgeManifestPageInput) ([]byte, error)

func (f spaceManifestPageImporterFunc) ImportSpacePurgeManifestPage(ctx context.Context, input store.SpacePurgeManifestPageInput) ([]byte, error) {
	return f(ctx, input)
}

func TestImportSpacePurgeManifestPageRequiresRequestBoundSpacePrincipal(t *testing.T) {
	request := validSpacePurgeManifestPageRequest()
	require.NoError(t, sealSpacePurgeManifestPage(request.GetPage()))
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	called := false
	service := &MessagingGRPC{SpaceManifestImporter: spaceManifestPageImporterFunc(func(_ context.Context, input store.SpacePurgeManifestPageInput) ([]byte, error) {
		called = true
		require.Equal(t, uuid.MustParse(request.GetSpaceId()), input.SpaceID)
		require.Equal(t, uuid.MustParse(request.GetDeletionOperationId()), input.DeletionOperationID)
		require.EqualValues(t, request.GetScheduleGeneration(), input.ScheduleGeneration)
		require.Equal(t, request.GetPage().GetPageSha256(), input.Page.GetPageSha256())
		require.Len(t, input.RequestSHA256, sha256.Size)
		response := &messagingv1.ImportSpacePurgeManifestPageResponse{}
		require.NoError(t, proto.Unmarshal(input.ReceiptBytes, response))
		require.True(t, response.GetReceipt().GetManifestSealed())
		return input.ReceiptBytes, nil
	})}
	ctx := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "messaging",
		RPC:       messagingv1.MessagingService_ImportSpacePurgeManifestPage_FullMethodName,
		RequestID: uuid.NewString(), RequestHash: hash,
	})
	response, err := service.ImportSpacePurgeManifestPage(ctx, request)
	require.NoError(t, err)
	require.True(t, called)
	require.True(t, response.GetReceipt().GetManifestSealed())

	called = false
	wrongIssuer := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging",
		RPC:       messagingv1.MessagingService_ImportSpacePurgeManifestPage_FullMethodName,
		RequestID: uuid.NewString(), RequestHash: hash,
	})
	_, err = service.ImportSpacePurgeManifestPage(wrongIssuer, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.False(t, called, "untrusted callers cannot import manifest data")

	service.SpaceManifestImporter = spaceManifestPageImporterFunc(func(context.Context, store.SpacePurgeManifestPageInput) ([]byte, error) {
		return []byte("corrupt receipt bytes"), nil
	})
	_, err = service.ImportSpacePurgeManifestPage(ctx, request)
	require.Equal(t, codes.Internal, status.Code(err), "a corrupt stored receipt cannot be returned as owner evidence")
}

type spaceLifecycleFenceApplierFunc func(context.Context, store.SpaceLifecycleFenceInput) ([]byte, error)

func (f spaceLifecycleFenceApplierFunc) ApplySpaceLifecycleFence(ctx context.Context, input store.SpaceLifecycleFenceInput) ([]byte, error) {
	return f(ctx, input)
}

func TestApplySpaceLifecycleFenceRequiresBoundSpacePrincipalAndReturnsReceipt(t *testing.T) {
	request := &messagingv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             "20000000-0000-4000-8000-000000000301",
		DeletionOperationId: "20000000-0000-4000-8000-000000000302",
		Generation:          8,
		DesiredState:        commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:            &commonv1.ManifestBinding{ManifestId: "20000000-0000-4000-8000-000000000303", ManifestSha256: make([]byte, sha256.Size), ItemCount: 1},
	}}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	called := false
	service := &MessagingGRPC{SpaceFileProducer:testSpaceFileProducer(t),SpaceLifecycleFences: spaceLifecycleFenceApplierFunc(func(_ context.Context, input store.SpaceLifecycleFenceInput) ([]byte, error) {
		called = true
		require.Equal(t, uuid.MustParse(request.GetFence().GetSpaceId()), input.SpaceID)
		require.Equal(t, uuid.MustParse(request.GetFence().GetDeletionOperationId()), input.DeletionOperationID)
		require.EqualValues(t, 8, input.Generation)
		require.Equal(t, request.GetFence().GetManifest().GetManifestId(), input.Manifest.GetManifestId())
		response := &messagingv1.ApplySpaceLifecycleFenceResponse{Receipt: &commonv1.SpaceLifecycleFenceReceipt{
			ProtocolVersion: 1, ReceiptId: "20000000-0000-4000-8000-000000000304", SpaceId: request.GetFence().GetSpaceId(),
			DeletionOperationId: request.GetFence().GetDeletionOperationId(), Generation: input.Generation,
			ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, AppliedState: request.GetFence().GetDesiredState(),
			RequestSha256: input.RequestSHA256, ManifestSha256: input.Manifest.GetManifestSha256(), AppliedAt: timestamppb.Now(),
		}}
		return proto.MarshalOptions{Deterministic: true}.Marshal(response)
	})}
	ctx := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "messaging",
		RPC: messagingv1.MessagingService_ApplySpaceLifecycleFence_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash,
	})
	response, err := service.ApplySpaceLifecycleFence(ctx, request)
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, response.GetReceipt().GetParticipantId())
	require.EqualValues(t, 8, response.GetReceipt().GetGeneration())

	called = false
	wrongIssuer := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging",
		RPC: messagingv1.MessagingService_ApplySpaceLifecycleFence_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash,
	})
	_, err = service.ApplySpaceLifecycleFence(wrongIssuer, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.False(t, called, "untrusted callers cannot mutate a participant fence")
}

func TestValidateSpacePurgeManifestImportPageRequiresSealOnlyAtManifestEnd(t *testing.T) {
	request := validSpacePurgeManifestPageRequest()
	request.Page.Manifest.ItemCount = 1001
	request.Page.NextPageToken = "next-page"
	request.Page.ItemIds = make([]string, spacePurgeManifestPageSize)
	for i := range request.Page.ItemIds {
		request.Page.ItemIds[i] = fmt.Sprintf("20000000-0000-4000-8000-%012x", i+1)
	}
	request.SealsManifest = true
	require.NoError(t, sealSpacePurgeManifestPage(request.GetPage()))
	require.Error(t, validateSpacePurgeManifestPageRequest(request), "a non-final page cannot seal the manifest")

	request.SealsManifest = false
	require.NoError(t, validateSpacePurgeManifestPageRequest(request))

	request.Page.PageIndex = 1
	request.Page.ItemIds = []string{"20000000-0000-4000-8000-0000000003e9"}
	request.Page.NextPageToken = ""
	require.NoError(t, sealSpacePurgeManifestPage(request.GetPage()))
	request.SealsManifest = false
	require.Error(t, validateSpacePurgeManifestPageRequest(request), "a final page must seal the manifest")
}

func validSpacePurgeManifestPageRequest() *messagingv1.ImportSpacePurgeManifestPageRequest {
	return &messagingv1.ImportSpacePurgeManifestPageRequest{
		ProtocolVersion:     1,
		SpaceId:             "20000000-0000-4000-8000-000000000001",
		DeletionOperationId: "20000000-0000-4000-8000-000000000002",
		ScheduleGeneration:  7,
		SealsManifest:       true,
		Page: &chatv1.SpacePurgeManifestPage{
			ProtocolVersion: 1,
			Manifest: &commonv1.ManifestBinding{
				ManifestId:     "20000000-0000-4000-8000-000000000003",
				ManifestSha256: make([]byte, sha256.Size),
				ItemCount:      1,
			},
			PageIndex: 0,
			ItemIds:   []string{"20000000-0000-4000-8000-000000000010"},
		},
	}
}

func sealSpacePurgeManifestPage(page *chatv1.SpacePurgeManifestPage) error {
	h := sha256.New()
	h.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
	h.Write(page.GetManifest().GetManifestSha256())
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], page.GetPageIndex())
	h.Write(number[:])
	binary.BigEndian.PutUint64(number[:], uint64(len(page.GetItemIds())))
	h.Write(number[:])
	for _, value := range page.GetItemIds() {
		id, err := uuid.Parse(value)
		if err != nil {
			return err
		}
		h.Write(id[:])
	}
	page.PageSha256 = h.Sum(nil)
	return nil
}
