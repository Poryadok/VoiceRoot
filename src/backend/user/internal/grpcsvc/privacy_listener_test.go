package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
)

func TestProtectedPrivacyDomainRequiresVerifiedSocial(t *testing.T) {
	req := &userv1.GetPrivacySettingsRequest{ProfileId: "77e391a9-6fb2-48b8-ad2e-cbc6d33a8eaf"}
	svc := &SocialPrivacyGRPC{User: &UserGRPC{}}
	_, err := svc.GetPrivacySettings(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "social", Subject: "service:social", Audience: "user", RPC: socialprincipal.Method("user"), RequestHash: hash}
	_, err = svc.GetPrivacySettings(principal.WithVerified(context.Background(), p), req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "authenticated request reaches missing store")
	p.Issuer = "chat"
	p.Subject = "service:chat"
	_, err = svc.GetPrivacySettings(principal.WithVerified(context.Background(), p), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestNotificationRoutingPresenceRequiresDedicatedPrincipal(t *testing.T) {
	req := &userv1.GetNotificationRoutingPresenceRequest{ProfileId: "77e391a9-6fb2-48b8-ad2e-cbc6d33a8eaf"}
	svc := &NotificationPresenceGRPC{User: &UserGRPC{}}
	_, err := svc.GetNotificationRoutingPresence(context.Background(), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	valid := principal.Principal{Kind: "service", Issuer: "notification", Subject: "service:notification", Audience: "user", RPC: userv1.UserService_GetNotificationRoutingPresence_FullMethodName, RequestHash: hash}
	_, err = svc.GetNotificationRoutingPresence(principal.WithVerified(context.Background(), valid), req)
	require.Equal(t, codes.Unavailable, status.Code(err), "a correctly bound internal principal reaches the service dependency")

	wrong := valid
	wrong.Issuer, wrong.Subject = "social", "service:social"
	_, err = svc.GetNotificationRoutingPresence(principal.WithVerified(context.Background(), wrong), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = (&UserGRPC{}).GetNotificationRoutingPresence(context.Background(), req)
	require.Equal(t, codes.Unimplemented, status.Code(err), "the public User registration does not expose this internal method")
}

type serviceDescriptorRecorder struct {
	desc *grpc.ServiceDesc
	impl any
}

func (r *serviceDescriptorRecorder) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.desc, r.impl = desc, impl
}

func TestRegisterNotificationPresenceServerExposesOnlyRoutingRPC(t *testing.T) {
	var registrar serviceDescriptorRecorder
	RegisterNotificationPresenceServer(&registrar, &UserGRPC{})
	require.NotNil(t, registrar.desc)
	require.Len(t, registrar.desc.Methods, 1)
	require.Equal(t, "GetNotificationRoutingPresence", registrar.desc.Methods[0].MethodName)
	require.Empty(t, registrar.desc.Streams)
}

func TestMessagingScheduledPresenceRequiresDedicatedPrincipal(t *testing.T) {
	req := &userv1.GetScheduledMessageDispatchPresenceRequest{
		ScheduledMessageId: uuid.NewString(), SenderAccountId: uuid.NewString(), SenderProfileId: uuid.NewString(),
		ChatId: uuid.NewString(), ScheduleGeneration: 1,
		Mode:     userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_AT,
		ChatType: chatv1.ChatType_CHAT_TYPE_GROUP,
	}
	svc := &MessagingScheduledPresenceGRPC{User: &UserGRPC{}}
	_, err := svc.GetScheduledMessageDispatchPresence(context.Background(), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	valid := principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "user", RPC: userv1.UserService_GetScheduledMessageDispatchPresence_FullMethodName, RequestHash: hash}
	_, err = svc.GetScheduledMessageDispatchPresence(principal.WithVerified(context.Background(), valid), req)
	require.Equal(t, codes.Unavailable, status.Code(err), "a valid principal and body reach the service dependency")

	invalidBody := *req
	invalidBody.Mode = userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_UNSPECIFIED
	invalidHash, err := principal.RequestHash(&invalidBody)
	require.NoError(t, err)
	valid.RequestHash = invalidHash
	_, err = svc.GetScheduledMessageDispatchPresence(principal.WithVerified(context.Background(), valid), &invalidBody)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "a valid principal cannot make an open-ended mode valid")

	valid.Issuer, valid.Subject = "notification", "service:notification"
	valid.RequestHash = hash
	_, err = svc.GetScheduledMessageDispatchPresence(principal.WithVerified(context.Background(), valid), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = (&UserGRPC{}).GetScheduledMessageDispatchPresence(context.Background(), req)
	require.Equal(t, codes.Unimplemented, status.Code(err), "the ordinary User listener must not implement this RPC")
}

func TestRegisterMessagingScheduledPresenceExposesOnlyScheduledDecision(t *testing.T) {
	var registrar serviceDescriptorRecorder
	RegisterMessagingScheduledPresenceServer(&registrar, &UserGRPC{})
	require.NotNil(t, registrar.desc)
	require.Len(t, registrar.desc.Methods, 1)
	require.Equal(t, "GetScheduledMessageDispatchPresence", registrar.desc.Methods[0].MethodName)
	require.Empty(t, registrar.desc.Streams)
}

func TestProtectedSocialGetProfilesIsRequestBoundAndCapped(t *testing.T) {
	svc := &SocialPrivacyGRPC{User: &UserGRPC{}}
	emptyRequest := &userv1.GetProfilesRequest{}
	_, err := svc.GetProfiles(context.Background(), emptyRequest)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	hash, err := principal.RequestHash(emptyRequest)
	require.NoError(t, err)
	identity := principal.Principal{
		Kind: "service", Issuer: "social", Subject: "service:social",
		Audience: "user", RPC: userv1.UserService_GetProfiles_FullMethodName, RequestHash: hash,
	}
	resp, err := svc.GetProfiles(principal.WithVerified(context.Background(), identity), emptyRequest)
	require.NoError(t, err, "a correctly request-bound Social principal can use the existing batch profile lookup")
	require.Empty(t, resp.GetProfileList().GetProfiles())

	tooMany := &userv1.GetProfilesRequest{ProfileIds: make([]string, socialGetProfilesMaxProfiles+1)}
	for i := range tooMany.ProfileIds {
		tooMany.ProfileIds[i] = uuid.NewString()
	}
	hash, err = principal.RequestHash(tooMany)
	require.NoError(t, err)
	identity.RequestHash = hash
	_, err = svc.GetProfiles(principal.WithVerified(context.Background(), identity), tooMany)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "the protected batch is bounded before User store access")
}
