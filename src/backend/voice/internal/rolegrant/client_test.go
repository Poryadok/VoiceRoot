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
	request *rolev1.CheckGameSessionGrantRequest
	allowed bool
	err     error
	ctx     context.Context
}

func (c *recordingRoleClient) CheckGameSessionGrant(ctx context.Context, request *rolev1.CheckGameSessionGrantRequest, _ ...grpc.CallOption) (*rolev1.CheckGameSessionGrantResponse, error) {
	c.ctx, c.request = ctx, request
	return &rolev1.CheckGameSessionGrantResponse{Allowed: c.allowed}, c.err
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
