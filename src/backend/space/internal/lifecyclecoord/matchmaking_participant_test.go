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
	commonv1 "voice.app/voice/common/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

type matchmakingLifecycleClientStub struct {
	fenceContext context.Context
	fenceRequest *matchmakingv1.ApplySpaceLifecycleFenceRequest
	fenceResult  *matchmakingv1.ApplySpaceLifecycleFenceResponse
	purgeContext context.Context
	purgeRequest *matchmakingv1.PurgeSpaceRequest
	purgeResult  *matchmakingv1.PurgeSpaceResponse
}

func (s *matchmakingLifecycleClientStub) ApplySpaceLifecycleFence(ctx context.Context, req *matchmakingv1.ApplySpaceLifecycleFenceRequest, _ ...grpc.CallOption) (*matchmakingv1.ApplySpaceLifecycleFenceResponse, error) {
	s.fenceContext, s.fenceRequest = ctx, req
	return s.fenceResult, nil
}

func (s *matchmakingLifecycleClientStub) PurgeSpace(ctx context.Context, req *matchmakingv1.PurgeSpaceRequest, _ ...grpc.CallOption) (*matchmakingv1.PurgeSpaceResponse, error) {
	s.purgeContext, s.purgeRequest = ctx, req
	return s.purgeResult, nil
}

func TestMatchmakingParticipantSignsExactRequestsForProtectedAudience(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clock := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return clock }})
	require.NoError(t, err)
	operationID := uuid.NewString()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: operationID, Generation: 4, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest}
	fenceReceipt := &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING}
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: fence.SpaceId, DeletionOperationId: operationID, Generation: 5, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING, Manifest: manifest}
	purgeReceipt := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING}
	client := &matchmakingLifecycleClientStub{
		fenceResult: &matchmakingv1.ApplySpaceLifecycleFenceResponse{Receipt: fenceReceipt},
		purgeResult: &matchmakingv1.PurgeSpaceResponse{Receipt: purgeReceipt},
	}
	participant := &MatchmakingParticipant{Issuer: issuer, Client: client}
	gotFence, err := participant.ApplySpaceLifecycleFence(context.Background(), fence)
	require.NoError(t, err)
	require.Equal(t, fenceReceipt, gotFence)
	require.Equal(t, fence, client.fenceRequest.GetFence())
	assertMatchmakingLifecyclePrincipal(t, client.fenceContext, matchmakingv1.MatchmakingService_ApplySpaceLifecycleFence_FullMethodName, client.fenceRequest)
	gotPurge, err := participant.PurgeSpace(context.Background(), purge)
	require.NoError(t, err)
	require.Equal(t, purgeReceipt, gotPurge)
	require.Equal(t, purge, client.purgeRequest.GetPurge())
	assertMatchmakingLifecyclePrincipal(t, client.purgeContext, matchmakingv1.MatchmakingService_PurgeSpace_FullMethodName, client.purgeRequest)
	require.NotEqual(t, lifecycleTestRequestID(t, client.fenceContext), lifecycleTestRequestID(t, client.purgeContext))
}

func TestMatchmakingParticipantRejectsInvalidOperationBeforeCall(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	client := &matchmakingLifecycleClientStub{}
	participant := &MatchmakingParticipant{Issuer: issuer, Client: client}
	_, err = participant.ApplySpaceLifecycleFence(context.Background(), &commonv1.SpaceLifecycleFenceRequest{DeletionOperationId: strings.ToUpper(uuid.NewString()), Generation: 1})
	require.Error(t, err)
	require.Nil(t, client.fenceRequest)
	_, err = participant.PurgeSpace(context.Background(), &commonv1.SpacePurgeRequest{DeletionOperationId: uuid.NewString(), Generation: 0})
	require.Error(t, err)
	require.Nil(t, client.purgeRequest)
}

func assertMatchmakingLifecyclePrincipal(t *testing.T, ctx context.Context, method string, request proto.Message) {
	t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	values := md.Get("authorization")
	require.Len(t, values, 1)
	parts := strings.Split(strings.TrimPrefix(values[0], "Bearer "), ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	require.Equal(t, "space", claims["iss"])
	require.Equal(t, "matchmaking", claims["aud"])
	require.Equal(t, method, claims["rpc"])
	require.Equal(t, lifecycleTestRequestID(t, ctx), claims["request_id"])
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	require.Equal(t, hash, claims["request_hash"])
}

func lifecycleTestRequestID(t *testing.T, ctx context.Context) string {
	t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	ids := md.Get("x-request-id")
	require.Len(t, ids, 1)
	return ids[0]
}
