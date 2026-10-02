package grpcsvc

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/pkg/principal"
)

type notificationManifestFake struct {
	notificationLifecycleFake
	calls   int
	corrupt bool
}

func (f *notificationManifestFake) ImportSpacePurgeManifestPage(_ context.Context, request *notificationv1.ImportSpacePurgeManifestPageRequest) (*notificationv1.ImportSpacePurgeManifestPageReceipt, error) {
	f.calls++
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append(append([]byte(request.ProtoReflect().Descriptor().FullName()), 0), wire...))
	hash := digest[:]
	if f.corrupt {
		hash[0] ^= 1
	}
	return &notificationv1.ImportSpacePurgeManifestPageReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.SpaceId, DeletionOperationId: request.DeletionOperationId, Generation: request.ScheduleGeneration, Manifest: request.Page.Manifest, PageIndex: request.Page.PageIndex, AcceptedCount: uint64(len(request.Page.ItemIds)), PageSha256: request.Page.PageSha256, ManifestSealed: request.SealsManifest, RequestSha256: hash, CompletedAt: timestamppb.Now()}, nil
}

func TestNotificationManifestPrincipalUnknownPolicyAndExactReceipt(t *testing.T) {
	request := &notificationv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), ScheduleGeneration: 1, SealsManifest: true, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}, PageSha256: make([]byte, 32)}}
	backend := &notificationManifestFake{}
	service := &NotificationGRPC{SpaceLifecycle: backend}
	verified := func(request *notificationv1.ImportSpacePurgeManifestPageRequest, issuer string) context.Context {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: issuer, Subject: "service:" + issuer, Audience: "notification", RPC: notificationv1.NotificationService_ImportSpacePurgeManifestPage_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash})
	}
	_, err := service.ImportSpacePurgeManifestPage(context.Background(), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = service.ImportSpacePurgeManifestPage(verified(request, "bot"), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, backend.calls)
	unknown := protowire.AppendVarint(protowire.AppendTag(nil, 100, protowire.VarintType), 7)
	request.Page.ProtoReflect().SetUnknown(unknown)
	response, err := service.ImportSpacePurgeManifestPage(verified(request, "space"), request)
	require.NoError(t, err)
	require.NotNil(t, response.Receipt)
	require.Equal(t, unknown, []byte(request.Page.ProtoReflect().GetUnknown()), "explicit Chat page compatibility policy preserves unknown wire fields")
	backend.corrupt = true
	_, err = service.ImportSpacePurgeManifestPage(verified(request, "space"), request)
	require.Equal(t, codes.DataLoss, status.Code(err))
	backend.corrupt = false
	for _, target := range []proto.Message{request, request.Page.Manifest} {
		target.ProtoReflect().SetUnknown(unknown)
		prior := backend.calls
		_, err = service.ImportSpacePurgeManifestPage(verified(request, "space"), request)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Equal(t, prior, backend.calls)
		target.ProtoReflect().SetUnknown(nil)
	}
	ctx := verified(request, "space")
	request.ScheduleGeneration++
	_, err = service.ImportSpacePurgeManifestPage(ctx, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
