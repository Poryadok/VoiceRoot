package rolegrant

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestLoadFromEnvRequiresCompleteVoiceRoleMTLSTuple(t *testing.T) {
	values := map[string]string{}
	get := func(key string) string { return values[key] }
	_, enabled, err := LoadFromEnv(get)
	require.NoError(t, err)
	require.False(t, enabled)
	values["VOICE_ROLE_GRPC_ADDR"] = "role:9091"
	_, enabled, err = LoadFromEnv(get)
	require.Error(t, err)
	require.True(t, enabled)
	for key, value := range map[string]string{
		"VOICE_ROLE_TLS_CA_FILE": "ca.pem", "VOICE_ROLE_CLIENT_CERT_FILE": "voice.crt", "VOICE_ROLE_CLIENT_KEY_FILE": "voice.key",
	} {
		values[key] = value
	}
	config, enabled, err := LoadFromEnv(get)
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "role:9091", config.Address)
}

type recordingRoleClient struct {
	rolev1.RoleServiceClient
	request       *rolev1.CheckGameSessionGrantRequest
	voiceRequest  *rolev1.ResolveVoiceRoomGrantsRequest
	voiceResponse *rolev1.ResolveVoiceRoomGrantsResponse
	allowed       bool
	err           error
	ctx           context.Context
}

func (c *recordingRoleClient) CheckGameSessionGrant(ctx context.Context, request *rolev1.CheckGameSessionGrantRequest, _ ...grpc.CallOption) (*rolev1.CheckGameSessionGrantResponse, error) {
	c.ctx, c.request = ctx, request
	return &rolev1.CheckGameSessionGrantResponse{Allowed: c.allowed}, c.err
}

func (c *recordingRoleClient) ResolveVoiceRoomGrants(ctx context.Context, request *rolev1.ResolveVoiceRoomGrantsRequest, _ ...grpc.CallOption) (*rolev1.ResolveVoiceRoomGrantsResponse, error) {
	c.ctx, c.voiceRequest = ctx, request
	return c.voiceResponse, c.err
}

func TestCheckerBindsExactVoicePrincipalAndFailsClosed(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "voice", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	client := &recordingRoleClient{allowed: true}
	checker, err := NewChecker(client, issuer)
	require.NoError(t, err)
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	require.NoError(t, checker.CheckGameSessionGrant(context.Background(), ids[0], ids[1], ids[2], ids[3], ids[4]))
	require.True(t, proto.Equal(&rolev1.CheckGameSessionGrantRequest{ApplicationId: ids[0], EnvironmentId: ids[1], SessionId: ids[2], VoiceRoomId: ids[3], ProfileId: ids[4]}, client.request))
	md, ok := metadata.FromOutgoingContext(client.ctx)
	require.True(t, ok)
	auth := md.Get("authorization")
	requestIDs := md.Get("x-request-id")
	require.Len(t, auth, 1)
	require.Len(t, requestIDs, 1)
	requestHash, err := principal.RequestHash(client.request)
	require.NoError(t, err)
	verified, err := principal.VerifyService(context.Background(), auth[0][len("Bearer "):], principal.VerifyConfig{
		ExpectedIssuer: "voice", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckGameSessionGrant_FullMethodName,
		ExpectedRequestID: requestIDs[0], ExpectedRequestHash: requestHash,
		KeyResolver: func(_ context.Context, issuerID, keyID string) (*rsa.PublicKey, error) {
			require.Equal(t, "voice", issuerID)
			require.Equal(t, "test", keyID)
			return &key.PublicKey, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "service:voice", verified.Subject)
	require.NoError(t, uuid.Validate(requestIDs[0]))

	client.allowed = false
	err = checker.CheckGameSessionGrant(context.Background(), ids[0], ids[1], ids[2], ids[3], ids[4])
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCheckerResolvesVoiceGrantsWithFreshPrivateAssertionAndReplacesOutgoingMetadata(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "voice", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	request := &rolev1.ResolveVoiceRoomGrantsRequest{SpaceId: uuid.NewString(), VoiceRoomId: uuid.NewString(), ProfileId: uuid.NewString()}
	client := &recordingRoleClient{voiceResponse: &rolev1.ResolveVoiceRoomGrantsResponse{
		PolicyEpoch: 23, Grants: &rolev1.VoiceRoomGrants{CanJoin: true, CanPublishAudio: true, CanSubscribe: true},
	}}
	checker, err := NewChecker(client, issuer)
	require.NoError(t, err)
	forged := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer forged-user-token", "x-request-id", "caller-request", "x-user-profile", uuid.NewString(),
	))
	grants, err := checker.ResolveVoiceRoomGrants(forged, request.SpaceId, request.VoiceRoomId, request.ProfileId)
	require.NoError(t, err)
	require.Equal(t, uint64(23), grants.PolicyEpoch)
	require.True(t, grants.CanJoin)
	require.True(t, grants.CanPublishAudio)
	require.True(t, grants.CanSubscribe)
	require.True(t, proto.Equal(request, client.voiceRequest))
	md, ok := metadata.FromOutgoingContext(client.ctx)
	require.True(t, ok)
	require.Len(t, md.Get("authorization"), 1)
	require.Len(t, md.Get("x-request-id"), 1)
	require.Empty(t, md.Get("x-user-profile"))
	require.NotEqual(t, "Bearer forged-user-token", md.Get("authorization")[0])
	require.NotEqual(t, "caller-request", md.Get("x-request-id")[0])
	require.NoError(t, uuid.Validate(md.Get("x-request-id")[0]))

	hash, err := principal.RequestHash(client.voiceRequest)
	require.NoError(t, err)
	verified, err := principal.VerifyService(context.Background(), md.Get("authorization")[0][len("Bearer "):], principal.VerifyConfig{
		ExpectedIssuer: "voice", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_ResolveVoiceRoomGrants_FullMethodName,
		ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash,
		KeyResolver: func(_ context.Context, issuerID, keyID string) (*rsa.PublicKey, error) {
			require.Equal(t, "voice", issuerID)
			require.Equal(t, "test", keyID)
			return &key.PublicKey, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "service:voice", verified.Subject)
	firstRequestID := md.Get("x-request-id")[0]
	_, err = checker.ResolveVoiceRoomGrants(forged, request.SpaceId, request.VoiceRoomId, request.ProfileId)
	require.NoError(t, err)
	secondMD, ok := metadata.FromOutgoingContext(client.ctx)
	require.True(t, ok)
	require.NotEqual(t, firstRequestID, secondMD.Get("x-request-id")[0], "every private lookup needs a fresh request ID/JTI")
}

func TestCheckerRejectsIncompleteVoiceGrantDecision(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "voice", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	checker, err := NewChecker(&recordingRoleClient{}, issuer)
	require.NoError(t, err)
	_, err = checker.ResolveVoiceRoomGrants(context.Background(), uuid.NewString(), uuid.NewString(), uuid.NewString())
	require.Equal(t, codes.Unavailable, status.Code(err))
}
