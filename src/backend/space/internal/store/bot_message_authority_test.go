package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

func TestBotMessageAuthorityNativeInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real Native51 PostgreSQL")
	}
	f := newBotInstallationScopeFixture(t)
	require.NoError(t, f.call(t, f.add))
	ctx := context.Background()
	chat := uuid.NewString()
	_, err := f.store.Pool.Exec(ctx, `INSERT INTO space_tree_nodes(space_id,kind,chat_id) VALUES($1,'text_chat',$2)`, f.add.SpaceId, chat)
	require.NoError(t, err)
	in := f.add.InstallationIntent
	binding := &commonv1.BotMessageSendBinding{ProtocolVersion: 1, BotId: in.BotId, ActorProfileId: in.ActorProfileId, SpaceId: in.SpaceId, InstallationId: in.InstallationId, InstallOperationId: in.OperationId, InstallationGeneration: in.Generation, ChatId: chat}
	req := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: in.SpaceId, ProfileId: in.ActorProfileId, ChatId: &chat, BotMessageSendBinding: binding}
	response, err := f.store.ReadMemberAdmissionAuthority(ctx, req)
	require.NoError(t, err)
	require.True(t, proto.Equal(binding, response.GetBotMessageSendBinding()), "exact current native Bot binding must be echoed")
	ordinary := proto.Clone(req).(*spacev1.GetMemberAdmissionAuthorityRequest)
	ordinary.BotMessageSendBinding = nil
	ordinaryResponse, err := f.store.ReadMemberAdmissionAuthority(ctx, ordinary)
	require.NoError(t, err)
	require.Nil(t, ordinaryResponse.BotMessageSendBinding, "Role observation remains unchanged")
	var owner string
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT owner_profile_id::text FROM spaces WHERE id=$1`, in.SpaceId).Scan(&owner))
	for _, mutate := range []func(*spacev1.GetMemberAdmissionAuthorityRequest){
		func(r *spacev1.GetMemberAdmissionAuthorityRequest) { r.BotMessageSendBinding.InstallationGeneration++ },
		func(r *spacev1.GetMemberAdmissionAuthorityRequest) {
			r.BotMessageSendBinding.InstallationId = uuid.NewString()
		},
		func(r *spacev1.GetMemberAdmissionAuthorityRequest) {
			r.BotMessageSendBinding.InstallOperationId = uuid.NewString()
		},
		func(r *spacev1.GetMemberAdmissionAuthorityRequest) { r.BotMessageSendBinding.BotId = uuid.NewString() },
		func(r *spacev1.GetMemberAdmissionAuthorityRequest) { r.ProfileId = uuid.NewString() },
		func(r *spacev1.GetMemberAdmissionAuthorityRequest) {
			r.ProfileId = owner
			r.BotMessageSendBinding.ActorProfileId = owner
		},
		func(r *spacev1.GetMemberAdmissionAuthorityRequest) {
			r.BotMessageSendBinding.ProtoReflect().SetUnknown([]byte{0x48, 1})
		},
		func(r *spacev1.GetMemberAdmissionAuthorityRequest) {
			other := uuid.NewString()
			r.ChatId = &other
			r.BotMessageSendBinding.ChatId = other
		},
		func(r *spacev1.GetMemberAdmissionAuthorityRequest) {
			generation := uint64(1)
			operation := in.OperationId
			r.ExpectedGeneration = &generation
			r.ExpectedAdmissionOperationId = &operation
		},
	} {
		changed := proto.Clone(req).(*spacev1.GetMemberAdmissionAuthorityRequest)
		mutate(changed)
		_, err = f.store.ReadMemberAdmissionAuthority(ctx, changed)
		require.Error(t, err)
	}
	require.NoError(t, f.call(t, botScopeCancel(f.add)))
	_, err = f.store.ReadMemberAdmissionAuthority(ctx, req)
	require.Error(t, err)
	next := proto.Clone(f.add).(*spacev1.AddBotMemberRequest)
	next.InstallationIntent.Generation++
	next.InstallationIntent.OperationId = uuid.NewString()
	next.InstallationIntent.InstallationId = uuid.NewString()
	require.NoError(t, f.call(t, next))
	_, err = f.store.ReadMemberAdmissionAuthority(ctx, req)
	require.Error(t, err, "reinstall cannot revive old tuple")
	req.BotMessageSendBinding.InstallationGeneration = next.InstallationIntent.Generation
	req.BotMessageSendBinding.InstallOperationId = next.InstallationIntent.OperationId
	req.BotMessageSendBinding.InstallationId = next.InstallationIntent.InstallationId
	response, err = f.store.ReadMemberAdmissionAuthority(ctx, req)
	require.NoError(t, err)
	require.True(t, proto.Equal(req.BotMessageSendBinding, response.GetBotMessageSendBinding()))
	blocker, err := f.store.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, lockLifecycleSpace(ctx, blocker, uuid.MustParse(in.SpaceId)))
	bounded, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, err = f.store.ReadMemberAdmissionAuthority(bounded, req)
	require.Error(t, err, "waiting for the fence cannot renew original call budget")
	require.NoError(t, blocker.Rollback(ctx))
}
