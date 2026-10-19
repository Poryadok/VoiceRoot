//go:build role_space_native

package main

import (
	"context"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/principalgrpc"
	"voice/backend/role/internal/principalruntime"
	"voice/backend/role/internal/store"
	"voice/backend/role/permissions"
)

func exerciseNativeReportPermissions(t *testing.T, client rolev1.RoleServiceClient, key *rsa.PrivateKey, space, owner, member, chat uuid.UUID, roles *store.RoleStore, memberRole uuid.UUID) {
	t.Helper()
	for _, tc := range []struct{ issuer, permission string }{
		{"messaging", "TEXT_CHAT_VIEW"},
		{"messaging", "TEXT_CHAT_READ_HISTORY"},
		{"moderation", "MODERATION_MANAGE_REPORTS"},
	} {
		req := &rolev1.CheckPermissionRequest{SpaceId: space.String(), ProfileId: owner.String(), PermissionName: tc.permission, Chat: &chatv1.ChatRef{Id: chat.String(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
		signed := nativeWorkloadContext(t, key, tc.issuer, rolev1.RoleService_CheckPermission_FullMethodName, req)
		response, err := client.CheckPermission(signed, req)
		require.NoError(t, err, "actual protected %s %s needs a qualified exact chat decision", tc.issuer, tc.permission)
		require.True(t, response.GetAllowed())
		_, err = client.CheckPermission(signed, req)
		require.Equal(t, codes.Unauthenticated, status.Code(err), "same signed transport cannot replay")
	}
	request := &rolev1.CheckPermissionRequest{SpaceId: space.String(), ProfileId: member.String(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: chat.String(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
	call := func(req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		return client.CheckPermission(nativeWorkloadContext(t, key, "messaging", rolev1.RoleService_CheckPermission_FullMethodName, req), req)
	}
	// Actual qualified Member plus explicit per-chat restrictions; HISTORY is a
	// separate decision and its deny must not remove current VIEW permission.
	view, err := permissions.MaskFor("TEXT_CHAT_VIEW")
	require.NoError(t, err)
	history, err := permissions.MaskFor("TEXT_CHAT_READ_HISTORY")
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, roles.SetChatOverride(ctx, chat, memberRole, view, history))
	response, err := call(request)
	require.NoError(t, err)
	require.True(t, response.GetAllowed())
	request.PermissionName = "TEXT_CHAT_READ_HISTORY"
	response, err = call(request)
	require.NoError(t, err)
	require.False(t, response.GetAllowed(), "entry-time fallback remains Messaging's separate obligation")
	require.NoError(t, roles.SetChatOverride(ctx, chat, memberRole, history, view))
	response, err = call(request)
	require.NoError(t, err)
	require.True(t, response.GetAllowed())
	request.PermissionName = "TEXT_CHAT_VIEW"
	response, err = call(request)
	require.NoError(t, err)
	require.False(t, response.GetAllowed())
	require.NoError(t, roles.SetChatOverride(ctx, chat, memberRole, 0, 0))
	moderator := proto.Clone(request).(*rolev1.CheckPermissionRequest)
	moderator.PermissionName = "MODERATION_MANAGE_REPORTS"
	response, err = client.CheckPermission(nativeWorkloadContext(t, key, "moderation", rolev1.RoleService_CheckPermission_FullMethodName, moderator), moderator)
	require.NoError(t, err)
	require.False(t, response.GetAllowed(), "ordinary Member gets no report moderation authority")
	for _, tc := range []struct {
		name   string
		mutate func(*rolev1.CheckPermissionRequest)
		code   codes.Code
	}{
		{"missing_chat", func(r *rolev1.CheckPermissionRequest) { r.Chat = nil }, codes.InvalidArgument},
		{"missing_type", func(r *rolev1.CheckPermissionRequest) { r.Chat.Type = nil }, codes.InvalidArgument},
		{"dm", func(r *rolev1.CheckPermissionRequest) { r.Chat.Type = chatv1.ChatType_CHAT_TYPE_DM.Enum() }, codes.InvalidArgument},
		{"nested_unknown", func(r *rolev1.CheckPermissionRequest) { r.Chat.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01}) }, codes.InvalidArgument},
		{"two_nodes", func(r *rolev1.CheckPermissionRequest) { room := uuid.NewString(); r.VoiceRoomId = &room }, codes.InvalidArgument},
		{"cross_purpose", func(r *rolev1.CheckPermissionRequest) { r.PermissionName = "MODERATION_MANAGE_REPORTS" }, codes.Unauthenticated},
		{"foreign_chat", func(r *rolev1.CheckPermissionRequest) { r.Chat.Id = uuid.NewString() }, codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := proto.Clone(request).(*rolev1.CheckPermissionRequest)
			tc.mutate(req)
			_, err := call(req)
			require.Equal(t, tc.code, status.Code(err))
		})
	}
	signed := nativeWorkloadContext(t, key, "messaging", rolev1.RoleService_CheckPermission_FullMethodName, request)
	changed := proto.Clone(request).(*rolev1.CheckPermissionRequest)
	changed.ProfileId = owner.String()
	_, err = client.CheckPermission(signed, changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "full decision body is signature bound")
	called := false
	_, err = principalgrpc.OwnershipUnaryInterceptor(nil)(ctx, request, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_CheckPermission_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, called, "ordinary listener rejects before handler")
}

func exerciseNativeReportExpiry(t *testing.T, ctx context.Context, runtime *principalruntime.Runtime, authority *nativeTrackedObserver, key *rsa.PrivateKey, roles *store.RoleStore, space, profile, chat uuid.UUID) {
	t.Helper()
	delay := &nativeChatCommitExpiry{space: space.String(), profile: profile.String()}
	config := roles.Pool.Config().Copy()
	config.ConnConfig.Tracer = delay
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer pool.Close()
	handler := &grpcsvc.RoleGRPC{Store: &store.RoleStore{Pool: pool}, MemberAuthority: authority}
	req := &rolev1.CheckPermissionRequest{SpaceId: space.String(), ProfileId: profile.String(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: chat.String(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
	signed := nativeWorkloadContext(t, key, "messaging", rolev1.RoleService_CheckPermission_FullMethodName, req)
	md, ok := metadata.FromOutgoingContext(signed)
	require.True(t, ok)
	response, err := principalgrpc.StrictUnaryInterceptor(runtime)(metadata.NewIncomingContext(ctx, md), req, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_CheckPermission_FullMethodName}, func(callCtx context.Context, request any) (any, error) {
		return handler.CheckPermission(callCtx, request.(*rolev1.CheckPermissionRequest))
	})
	require.EqualValues(t, 1, delay.calls.Load())
	require.True(t, delay.fresh.Load(), "original report vector was fresh at owning COMMIT")
	require.Equal(t, codes.Unavailable, status.Code(err), "expiry after owning COMMIT must not return an allow")
	if response, ok := response.(*rolev1.CheckPermissionResponse); ok {
		require.Nil(t, response)
	}
	fresh := &grpcsvc.RoleGRPC{Store: roles, MemberAuthority: authority}
	signed = nativeWorkloadContext(t, key, "messaging", rolev1.RoleService_CheckPermission_FullMethodName, req)
	md, ok = metadata.FromOutgoingContext(signed)
	require.True(t, ok)
	response, err = principalgrpc.StrictUnaryInterceptor(runtime)(metadata.NewIncomingContext(ctx, md), req, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_CheckPermission_FullMethodName}, func(callCtx context.Context, request any) (any, error) {
		return fresh.CheckPermission(callCtx, request.(*rolev1.CheckPermissionRequest))
	})
	require.NoError(t, err, "new observation after an expired read remains independently usable")
	require.True(t, response.(*rolev1.CheckPermissionResponse).GetAllowed())
}
