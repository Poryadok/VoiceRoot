package grpcsvc

import (
	"context"
	"testing"

	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSpaceLifecycleGRPCRequiresVerifiedSpacePrincipalBeforeStore(t *testing.T) {
	service := &SpaceLifecycleGRPC{}
	method := chatv1.ChatService_PrepareSpaceDeletionManifest_FullMethodName
	_, err := service.PrepareSpaceDeletionManifest(context.Background(), &chatv1.PrepareSpaceDeletionManifestRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	verified := principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "chat", RPC: method}
	ctx := principal.WithVerified(context.Background(), verified)
	_, err = service.PrepareSpaceDeletionManifest(ctx, &chatv1.PrepareSpaceDeletionManifestRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err), "valid identity may reach dependency readiness check")

	wrongAudience := verified
	wrongAudience.Audience = "voice"
	_, err = service.PrepareSpaceDeletionManifest(principal.WithVerified(context.Background(), wrongAudience), &chatv1.PrepareSpaceDeletionManifestRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	wrongMethod := verified
	wrongMethod.RPC = chatv1.ChatService_CreateChat_FullMethodName
	_, err = service.PrepareSpaceDeletionManifest(principal.WithVerified(context.Background(), wrongMethod), &chatv1.PrepareSpaceDeletionManifestRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestSpaceLifecyclePurgeDoesNotDeleteWithoutProofClients(t *testing.T) {
	method := chatv1.ChatService_PurgeSpace_FullMethodName
	verified := principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "chat", RPC: method}
	service := &SpaceLifecycleGRPC{}
	_, err := service.PurgeSpace(principal.WithVerified(context.Background(), verified), &chatv1.PurgeSpaceRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSearchManifestPrincipalCannotMutateChatLifecycle(t *testing.T) {
	service := &SpaceLifecycleGRPC{}
	page := chatv1.ChatService_GetSpacePurgeManifestPage_FullMethodName
	for _, issuer := range []string{"search", "space", "messaging", "gameintegration"} {
		verified := principal.Principal{Kind: "service", Issuer: issuer, Subject: "service:" + issuer, Audience: "chat", RPC: page}
		_, err := service.GetSpacePurgeManifestPage(principal.WithVerified(context.Background(), verified), &chatv1.GetSpacePurgeManifestPageRequest{})
		if issuer == "search" || issuer == "space" {
			require.Equal(t, codes.Unavailable, status.Code(err), "authorized reader reaches the store readiness check")
		} else {
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		}
	}
	for _, method := range []string{chatv1.ChatService_PrepareSpaceDeletionManifest_FullMethodName, chatv1.ChatService_PurgeSpace_FullMethodName} {
		ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "search", Subject: "service:search", Audience: "chat", RPC: method})
		require.Equal(t, codes.Unauthenticated, status.Code(requireSpaceLifecyclePrincipal(ctx, method)))
	}
}
