package grpcsvc

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/pkg/principal"
)

type notificationLifecycleFake struct {
	fence *commonv1.SpaceLifecycleFenceRequest
	purge *commonv1.SpacePurgeRequest
}

func notificationTestRequestDigest(t *testing.T, message proto.Message) []byte {
	t.Helper()
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
	require.NoError(t, err)
	digest := sha256.Sum256(append(append([]byte(message.ProtoReflect().Descriptor().FullName()), 0), encoded...))
	return digest[:]
}

func TestNotificationLifecycleReceiptRejectsDifferentRequestDigest(t *testing.T) {
	backend := &notificationLifecycleFake{}
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: "2f85e125-4227-4d5d-a82a-852b019c0df8", DeletionOperationId: "6a6e83cf-748f-4cf9-b708-1eb0e909b3ad", Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: "manifest", ManifestSha256: make([]byte, 32)}}
	receipt, err := backend.ApplySpaceLifecycleFence(context.Background(), fence)
	require.NoError(t, err)
	receipt.RequestSha256 = notificationTestRequestDigest(t, &notificationv1.ApplySpaceLifecycleFenceRequest{Fence: fence})
	require.True(t, validNotificationFenceReceipt(receipt, fence))
	receipt.RequestSha256[0] ^= 1
	require.False(t, validNotificationFenceReceipt(receipt, fence), "a different body must not complete the participant barrier")
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: fence.SpaceId, DeletionOperationId: fence.DeletionOperationId, Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION, Manifest: fence.Manifest, PurgeDecidedAt: timestamppb.Now()}
	purgeReceipt, err := backend.PurgeSpace(context.Background(), purge)
	require.NoError(t, err)
	purgeReceipt.RequestSha256 = notificationTestRequestDigest(t, &notificationv1.PurgeSpaceRequest{Purge: purge})
	require.True(t, validNotificationPurgeReceipt(purgeReceipt, purge))
	purgeReceipt.RequestSha256[0] ^= 1
	require.False(t, validNotificationPurgeReceipt(purgeReceipt, purge), "a different purge body must not complete the participant barrier")
}

func (f *notificationLifecycleFake) ApplySpaceLifecycleFence(_ context.Context, request *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	f.fence = request
	wrapper := &notificationv1.ApplySpaceLifecycleFenceRequest{Fence: request}
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(wrapper)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append(append([]byte(wrapper.ProtoReflect().Descriptor().FullName()), 0), encoded...))
	return &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion:     1,
		ReceiptId:           "notification-fence-receipt",
		SpaceId:             request.GetSpaceId(),
		DeletionOperationId: request.GetDeletionOperationId(),
		Generation:          request.GetGeneration(),
		ParticipantId:       commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION,
		AppliedState:        request.GetDesiredState(),
		RequestSha256:       digest[:],
		ManifestSha256:      append([]byte(nil), request.GetManifest().GetManifestSha256()...),
		AppliedAt:           timestamppb.Now(),
	}, nil
}

func (f *notificationLifecycleFake) PurgeSpace(_ context.Context, request *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	f.purge = request
	wrapper := &notificationv1.PurgeSpaceRequest{Purge: request}
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(wrapper)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append(append([]byte(wrapper.ProtoReflect().Descriptor().FullName()), 0), encoded...))
	return &commonv1.SpacePurgeReceipt{
		ProtocolVersion:     1,
		ReceiptId:           "notification-purge-receipt",
		SpaceId:             request.GetSpaceId(),
		DeletionOperationId: request.GetDeletionOperationId(),
		Generation:          request.GetGeneration(),
		ParticipantId:       commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION,
		State:               commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED,
		RequestSha256:       digest[:],
		CompletedAt:         timestamppb.Now(),
	}, nil
}

func TestNotificationLifecycleRequiresExactSpacePrincipal(t *testing.T) {
	service := &NotificationGRPC{SpaceLifecycle: &notificationLifecycleFake{}}
	request := &notificationv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             "2f85e125-4227-4d5d-a82a-852b019c0df8",
		DeletionOperationId: "6a6e83cf-748f-4cf9-b708-1eb0e909b3ad",
		Generation:          1,
		DesiredState:        commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:            &commonv1.ManifestBinding{ManifestId: "93af8297-b917-4051-9628-ef3e66817460", ManifestSha256: make([]byte, 32)},
	}}
	_, err := service.ApplySpaceLifecycleFence(context.Background(), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestNotificationLifecycleForwardsVerifiedFence(t *testing.T) {
	backend := &notificationLifecycleFake{}
	service := &NotificationGRPC{SpaceLifecycle: backend}
	request := &notificationv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             "2f85e125-4227-4d5d-a82a-852b019c0df8",
		DeletionOperationId: "6a6e83cf-748f-4cf9-b708-1eb0e909b3ad",
		Generation:          1,
		DesiredState:        commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:            &commonv1.ManifestBinding{ManifestId: "93af8297-b917-4051-9628-ef3e66817460", ManifestSha256: make([]byte, 32)},
	}}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "notification",
		RPC:       notificationv1.NotificationService_ApplySpaceLifecycleFence_FullMethodName,
		RequestID: "t40-notification-fence", RequestHash: hash,
	})
	response, err := service.ApplySpaceLifecycleFence(ctx, request)
	require.NoError(t, err)
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION, response.GetReceipt().GetParticipantId())
	require.Equal(t, request.GetFence(), backend.fence)
}
