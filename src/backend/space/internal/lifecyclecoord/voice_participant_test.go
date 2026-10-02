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
	callsv1 "voice.app/voice/calls/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/principal"
)

type voiceLifecycleClientStub struct {
	callsv1.VoiceServiceClient
	fenceContext context.Context
	fenceRequest *callsv1.ApplySpaceLifecycleFenceRequest
	fenceResult  *callsv1.ApplySpaceLifecycleFenceResponse
	purgeContext context.Context
	purgeRequest *callsv1.PurgeSpaceRequest
	purgeResult  *callsv1.PurgeSpaceResponse
}

func (s *voiceLifecycleClientStub) ApplySpaceLifecycleFence(ctx context.Context, req *callsv1.ApplySpaceLifecycleFenceRequest, _ ...grpc.CallOption) (*callsv1.ApplySpaceLifecycleFenceResponse, error) {
	s.fenceContext, s.fenceRequest = ctx, req
	return s.fenceResult, nil
}

func (s *voiceLifecycleClientStub) PurgeSpace(ctx context.Context, req *callsv1.PurgeSpaceRequest, _ ...grpc.CallOption) (*callsv1.PurgeSpaceResponse, error) {
	s.purgeContext, s.purgeRequest = ctx, req
	return s.purgeResult, nil
}

func TestVoiceParticipantSignsExactFenceAndPurgeRPCRequests(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clock := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return clock }})
	require.NoError(t, err)
	operationID := uuid.NewString()
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: operationID, Generation: 4, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}}
	fenceReceipt := &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE}
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: fence.SpaceId, DeletionOperationId: operationID, Generation: 5, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE, Manifest: fence.Manifest}
	purgeReceipt := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE}
	client := &voiceLifecycleClientStub{
		fenceResult: &callsv1.ApplySpaceLifecycleFenceResponse{Receipt: fenceReceipt},
		purgeResult: &callsv1.PurgeSpaceResponse{Receipt: purgeReceipt},
	}
	participant := &VoiceParticipant{Issuer: issuer, Client: client}

	gotFence, err := participant.ApplySpaceLifecycleFence(context.Background(), fence)
	require.NoError(t, err)
	require.Equal(t, fenceReceipt, gotFence)
	require.Equal(t, fence, client.fenceRequest.GetFence())
	assertLifecyclePrincipal(t, client.fenceContext, callsv1.VoiceService_ApplySpaceLifecycleFence_FullMethodName, client.fenceRequest)

	gotPurge, err := participant.PurgeSpace(context.Background(), purge)
	require.NoError(t, err)
	require.Equal(t, purgeReceipt, gotPurge)
	require.Equal(t, purge, client.purgeRequest.GetPurge())
	assertLifecyclePrincipal(t, client.purgeContext, callsv1.VoiceService_PurgeSpace_FullMethodName, client.purgeRequest)

	require.NotEqual(t, outgoingRequestID(t, client.fenceContext), outgoingRequestID(t, client.purgeContext))
}

func TestVoiceParticipantFailsClosedWithoutTransportOrCanonicalOperation(t *testing.T) {
	ctx := context.Background()
	_, err := (*VoiceParticipant)(nil).ApplySpaceLifecycleFence(ctx, &commonv1.SpaceLifecycleFenceRequest{})
	require.Error(t, err)
	_, err = (&VoiceParticipant{}).PurgeSpace(ctx, &commonv1.SpacePurgeRequest{})
	require.Error(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	client := &voiceLifecycleClientStub{}
	participant := &VoiceParticipant{Issuer: issuer, Client: client}
	_, err = participant.ApplySpaceLifecycleFence(ctx, &commonv1.SpaceLifecycleFenceRequest{DeletionOperationId: strings.ToUpper(uuid.NewString()), Generation: 1})
	require.Error(t, err)
	require.Nil(t, client.fenceRequest)
}

func assertLifecyclePrincipal(t *testing.T, ctx context.Context, method string, request proto.Message) {
	t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	auth := md.Get("authorization")
	require.Len(t, auth, 1)
	parts := strings.Split(strings.TrimPrefix(auth[0], "Bearer "), ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	claims := map[string]any{}
	require.NoError(t, json.Unmarshal(payload, &claims))
	require.Equal(t, "space", claims["iss"])
	require.Equal(t, "voice", claims["aud"])
	require.Equal(t, method, claims["rpc"])
	require.Equal(t, outgoingRequestID(t, ctx), claims["request_id"])
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	require.Equal(t, hash, claims["request_hash"])
}

func outgoingRequestID(t *testing.T, ctx context.Context) string {
	t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	values := md.Get("x-request-id")
	require.Len(t, values, 1)
	return values[0]
}
