package lifecyclecoord

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type lifecycleInvokeFunc func(context.Context, string, any, any, ...grpc.CallOption) error

func (f lifecycleInvokeFunc) Invoke(ctx context.Context, method string, request, response any, options ...grpc.CallOption) error {
	return f(ctx, method, request, response, options...)
}

func TestGRPCParticipantBindsDynamicChatRequestAndReturnsOwnerReceipt(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clock := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return clock }})
	require.NoError(t, err)
	fence := &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 4,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:     &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)},
	}
	wantReceipt := &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: fence.GetSpaceId(),
		DeletionOperationId: fence.GetDeletionOperationId(), Generation: fence.GetGeneration(),
		ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
	}
	var seenContext context.Context
	var seenRequest proto.Message
	invoker := lifecycleInvokeFunc(func(ctx context.Context, method string, request, response any, _ ...grpc.CallOption) error {
		seenContext = ctx
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Equal(t, []string{fence.GetDeletionOperationId()}, md.Get("x-request-id"), "Chat's protected listener binds the transport request ID to the deletion operation")
		require.Equal(t, chatv1.ChatService_ApplySpaceLifecycleFence_FullMethodName, method)
		dynamicRequest, ok := request.(*dynamicpb.Message)
		require.True(t, ok)
		seenRequest = dynamicRequest
		fenceField := dynamicRequest.Descriptor().Fields().ByName("fence")
		require.NotNil(t, fenceField)
		gotFence := &commonv1.SpaceLifecycleFenceRequest{}
		encoded, marshalErr := proto.Marshal(dynamicRequest.Get(fenceField).Message().Interface())
		require.NoError(t, marshalErr)
		require.NoError(t, proto.Unmarshal(encoded, gotFence))
		require.True(t, proto.Equal(fence, gotFence))
		dynamicResponse, ok := response.(*dynamicpb.Message)
		require.True(t, ok)
		receiptField := dynamicResponse.Descriptor().Fields().ByName("receipt")
		require.NotNil(t, receiptField)
		encoded, marshalErr = proto.Marshal(wantReceipt)
		require.NoError(t, marshalErr)
		dynamicReceipt := dynamicpb.NewMessage(receiptField.Message())
		require.NoError(t, proto.Unmarshal(encoded, dynamicReceipt))
		dynamicResponse.Set(receiptField, protoreflect.ValueOfMessage(dynamicReceipt))
		return nil
	})
	participant := &GRPCParticipant{
		Issuer: issuer, Client: invoker, ParticipantID: commonv1.ParticipantId_PARTICIPANT_ID_CHAT, Audience: "chat",
		FenceMethod: chatv1.ChatService_ApplySpaceLifecycleFence_FullMethodName,
		PurgeMethod: chatv1.ChatService_PurgeSpace_FullMethodName,
	}
	got, err := participant.ApplySpaceLifecycleFence(context.Background(), fence)
	require.NoError(t, err)
	require.True(t, proto.Equal(wantReceipt, got))
	assertDynamicLifecycleToken(t, seenContext, "chat", chatv1.ChatService_ApplySpaceLifecycleFence_FullMethodName, seenRequest)
}

func TestGRPCParticipantRejectsAudienceAndPurgeParticipantMismatchBeforeInvoke(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	calls := 0
	invoker := lifecycleInvokeFunc(func(context.Context, string, any, any, ...grpc.CallOption) error { calls++; return nil })
	participant := &GRPCParticipant{
		Issuer: issuer, Client: invoker, ParticipantID: commonv1.ParticipantId_PARTICIPANT_ID_CHAT, Audience: "search",
		FenceMethod: chatv1.ChatService_ApplySpaceLifecycleFence_FullMethodName,
		PurgeMethod: chatv1.ChatService_PurgeSpace_FullMethodName,
	}
	_, err = participant.ApplySpaceLifecycleFence(context.Background(), &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 1,
	})
	require.Error(t, err)
	_, err = participant.PurgeSpace(context.Background(), &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 1,
		ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_FILE,
	})
	require.Error(t, err)
	require.Zero(t, calls)
}

func TestGRPCParticipantUsesCallsDescriptorForVoice(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	fence := &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 1,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:     &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)},
	}
	wantReceipt := &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: fence.GetSpaceId(),
		DeletionOperationId: fence.GetDeletionOperationId(), Generation: fence.GetGeneration(),
		ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
	}
	invoker := lifecycleInvokeFunc(func(ctx context.Context, method string, request, response any, _ ...grpc.CallOption) error {
		require.Equal(t, callsv1.VoiceService_ApplySpaceLifecycleFence_FullMethodName, method)
		dynamicResponse, ok := response.(*dynamicpb.Message)
		require.True(t, ok)
		receiptField := dynamicResponse.Descriptor().Fields().ByName("receipt")
		require.NotNil(t, receiptField)
		encoded, marshalErr := proto.Marshal(wantReceipt)
		require.NoError(t, marshalErr)
		dynamicReceipt := dynamicpb.NewMessage(receiptField.Message())
		require.NoError(t, proto.Unmarshal(encoded, dynamicReceipt))
		dynamicResponse.Set(receiptField, protoreflect.ValueOfMessage(dynamicReceipt))
		return nil
	})
	participant := &GRPCParticipant{
		Issuer: issuer, Client: invoker, ParticipantID: commonv1.ParticipantId_PARTICIPANT_ID_VOICE, Audience: "voice",
		FenceMethod: callsv1.VoiceService_ApplySpaceLifecycleFence_FullMethodName,
		PurgeMethod: callsv1.VoiceService_PurgeSpace_FullMethodName,
	}
	got, err := participant.ApplySpaceLifecycleFence(context.Background(), fence)
	require.NoError(t, err)
	require.True(t, proto.Equal(wantReceipt, got))
}

type roleRetirementClientFunc func(context.Context, *rolev1.RetireSpaceRequest, ...grpc.CallOption) (*rolev1.RetireSpaceResponse, error)

func (f roleRetirementClientFunc) RetireSpace(ctx context.Context, request *rolev1.RetireSpaceRequest, options ...grpc.CallOption) (*rolev1.RetireSpaceResponse, error) {
	return f(ctx, request, options...)
}

func TestGRPCRoleRetirementParticipantSignsExactRequestAndReturnsReceipt(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clock := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return clock }})
	require.NoError(t, err)
	request := &rolev1.RetireSpaceRequest{
		ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 4,
		PurgeDecidedAt: timestamppb.New(clock), Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)},
	}
	wantReceipt := &rolev1.RetireSpaceReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.GetSpaceId(),
		DeletionOperationId: request.GetDeletionOperationId(), Generation: request.GetGeneration(),
	}
	var seenContext context.Context
	var seenRequest proto.Message
	client := roleRetirementClientFunc(func(ctx context.Context, got *rolev1.RetireSpaceRequest, _ ...grpc.CallOption) (*rolev1.RetireSpaceResponse, error) {
		seenContext = ctx
		seenRequest = got
		require.True(t, proto.Equal(request, got))
		return &rolev1.RetireSpaceResponse{Receipt: wantReceipt}, nil
	})
	participant := &GRPCRoleRetirementParticipant{Issuer: issuer, Client: client}
	got, err := participant.RetireSpace(context.Background(), request)
	require.NoError(t, err)
	require.True(t, proto.Equal(wantReceipt, got))
	assertDynamicLifecycleToken(t, seenContext, "role", rolev1.RoleService_RetireSpace_FullMethodName, seenRequest)
}

func assertDynamicLifecycleToken(t *testing.T, ctx context.Context, audience, method string, request proto.Message) {
	t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	requestIDs := md.Get("x-request-id")
	require.Len(t, requestIDs, 1)
	authorization := md.Get("authorization")
	require.Len(t, authorization, 1)
	parts := strings.Split(strings.TrimPrefix(authorization[0], "Bearer "), ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	require.Equal(t, audience, claims["aud"])
	require.Equal(t, method, claims["rpc"])
	require.Equal(t, requestIDs[0], claims["request_id"])
	expectedHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	require.Equal(t, expectedHash, claims["request_hash"])
}
