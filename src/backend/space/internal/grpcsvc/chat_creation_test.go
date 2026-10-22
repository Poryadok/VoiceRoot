package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
	"voice/backend/space/internal/store"
)

func TestCompositePublicDisclosureBindsOriginalIdentity(t *testing.T) {
	user := &spacev1.SpaceChatCreationUser{AccountId: uuid.NewString(), SessionEpoch: 7, AccountType: "regular"}
	b := &spacev1.SpaceChatCreationBinding{CreatorProfileId: uuid.NewString(), Initiator: &spacev1.SpaceChatCreationBinding_User{User: user}}
	a := chatCreationAdmission{}
	actor := &spacev1.SpaceChatCreationPublicActor{ProfileId: b.CreatorProfileId, Initiator: &spacev1.SpaceChatCreationPublicActor_User{User: &spacev1.SpaceChatCreationUser{AccountId: user.AccountId, SessionEpoch: 8, AccountType: "guest"}}}
	require.NoError(t, a.CheckPublicOutcomeDisclosure(context.Background(), actor, b), "current guest/epoch must not rewrite historical creation identity")
	actor.GetUser().AccountId = uuid.NewString()
	require.Equal(t, codes.NotFound, status.Code(a.CheckPublicOutcomeDisclosure(context.Background(), actor, b)))
	actor.GetUser().AccountId = user.AccountId
	actor.ProfileId = uuid.NewString()
	require.Equal(t, codes.NotFound, status.Code(a.CheckPublicOutcomeDisclosure(context.Background(), actor, b)))
	bot := &spacev1.SpaceChatCreationBot{BotId: uuid.NewString(), InstallationId: uuid.NewString(), BotScope: "TEXT_CHAT_CREATE_IN_SPACE"}
	b.Initiator = &spacev1.SpaceChatCreationBinding_Bot{Bot: bot}
	actor.ProfileId = b.CreatorProfileId
	actor.Initiator = &spacev1.SpaceChatCreationPublicActor_Bot{Bot: bot}
	require.Equal(t, codes.Unavailable, status.Code(a.CheckPublicOutcomeDisclosure(context.Background(), actor, b)), "missing current Bot owner authority never grants disclosure")
}

type compositeRoleClient struct {
	rolev1.RoleServiceClient
	check func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)
}

func (c compositeRoleClient) CheckPermission(ctx context.Context, req *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return c.check(ctx, req)
}

func TestCompositeRoleAdmissionUsesFreshProtectedBindingAndNoFallback(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	b := &spacev1.SpaceChatCreationBinding{SpaceId: uuid.NewString(), CreatorProfileId: uuid.NewString(), OperationId: uuid.NewString()}
	tokens := []string{}
	protected := compositeRoleClient{check: func(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		require.Equal(t, b.SpaceId, req.SpaceId)
		require.Equal(t, b.CreatorProfileId, req.ProfileId)
		require.Equal(t, permissions.TextChatCreateInSpace, req.PermissionName)
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md, 2)
		require.Equal(t, []string{b.OperationId}, md.Get("x-request-id"))
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		tokens = append(tokens, token)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		p, err := principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckPermission_FullMethodName, ExpectedRequestID: b.OperationId, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		require.NoError(t, err)
		require.Equal(t, "service:space", p.Subject)
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}
	admission := chatCreationAdmission{space: &SpaceGRPC{Store: &store.SpaceStore{}, PrincipalIssuer: issuer, OwnershipRoles: protected}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString(), "authorization", "Bearer attacker"))
	require.NoError(t, admission.CheckChatCreation(ctx, b))
	require.NoError(t, admission.CheckChatCreation(ctx, b))
	require.NotEqual(t, tokens[0], tokens[1])
	// A configured legacy client cannot replace the dedicated protected one.
	admission.space.OwnershipRoles = nil
	admission.space.Roles = protected
	require.Equal(t, codes.Unavailable, status.Code(admission.CheckChatCreation(ctx, b)))
}

func TestCompositeRoleAdmissionOnlyAuthenticatedFalseIsTerminal(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	b := &spacev1.SpaceChatCreationBinding{SpaceId: uuid.NewString(), CreatorProfileId: uuid.NewString(), OperationId: uuid.NewString()}
	for _, tc := range []struct {
		name   string
		result *rolev1.CheckPermissionResponse
		err    error
		code   codes.Code
	}{
		{"allowed", &rolev1.CheckPermissionResponse{Allowed: true}, nil, codes.OK},
		{"domain denial", &rolev1.CheckPermissionResponse{}, nil, codes.PermissionDenied},
		{"nil decision", nil, nil, codes.Unavailable},
		{"transport permission denial", nil, status.Error(codes.PermissionDenied, "wrong listener"), codes.Unavailable},
		{"authentication denial", nil, status.Error(codes.Unauthenticated, "bad credential"), codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := chatCreationAdmission{space: &SpaceGRPC{Store: &store.SpaceStore{}, PrincipalIssuer: issuer, OwnershipRoles: compositeRoleClient{check: func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
				return tc.result, tc.err
			}}}}
			require.Equal(t, tc.code, status.Code(a.CheckChatCreation(context.Background(), b)))
		})
	}
}
