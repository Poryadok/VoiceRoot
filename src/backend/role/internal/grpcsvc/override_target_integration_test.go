package grpcsvc

import (
	"context"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
)

type ownerTargetsForTest struct {
	chat  func(context.Context, string, string) error
	voice func(context.Context, string, string) error
}

func (f ownerTargetsForTest) ValidateChat(ctx context.Context, s, c string) error {
	if f.chat != nil {
		return f.chat(ctx, s, c)
	}
	return nil
}
func (f ownerTargetsForTest) ValidateVoice(ctx context.Context, s, r string) error {
	if f.voice != nil {
		return f.voice(ctx, s, r)
	}
	return nil
}

// Existing hierarchy/permission tests use synthetic resource IDs and explicitly
// fake the independent owner existence seam. Production never defaults to this.
func withSyntheticOverrideTargets(s *RoleGRPC) { s.OverrideTargets = ownerTargetsForTest{} }

func TestRoleOverrideOwnerFailureMatrixAndNoWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	st, cleanup := startRoleStoreTest(t)
	defer cleanup()
	for _, voice := range []bool{false, true} {
		for _, ownerCode := range []codes.Code{codes.NotFound, codes.InvalidArgument, codes.Unavailable, codes.Unauthenticated, codes.PermissionDenied, codes.Unimplemented, codes.DeadlineExceeded} {
			f := newOrdinaryHandlerFixture(t, ctx, st.Pool)
			before := ordinaryHandlerSnapshot(t, ctx, st.Pool)
			calls := 0
			deny := func(context.Context, string, string) error { calls++; return status.Error(ownerCode, "owner denied") }
			f.svc.OverrideTargets = ownerTargetsForTest{chat: deny, voice: deny}
			var err error
			if voice {
				_, err = f.svc.SetVoiceRoomOverride(f.ctx, &rolev1.SetVoiceRoomOverrideRequest{SpaceId: f.space.String(), VoiceRoomId: f.voice.String(), RoleId: f.custom.String(), AllowMask: 16})
			} else {
				_, err = f.svc.SetChatOverride(f.ctx, &rolev1.SetChatOverrideRequest{SpaceId: f.space.String(), Chat: &chatv1.ChatRef{Id: f.chat.String()}, RoleId: f.custom.String(), AllowMask: 16})
			}
			want := ownerCode
			switch ownerCode {
			case codes.Unauthenticated, codes.PermissionDenied, codes.Unimplemented:
				want = codes.Unavailable
			}
			require.Equal(t, want, status.Code(err))
			require.Equal(t, 1, calls)
			require.Equal(t, before, ordinaryHandlerSnapshot(t, ctx, st.Pool))
			count, _ := f.events.snapshot()
			require.Zero(t, count)
		}
	}
}

func TestRoleOverrideFinalFreezeAndCancellationDenyWithoutEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	st, cleanup := startRoleStoreTest(t)
	defer cleanup()
	for _, voice := range []bool{false, true} {
		for _, cancelParent := range []bool{false, true} {
			f := newOrdinaryHandlerFixture(t, ctx, st.Pool)
			callCtx, cancel := context.WithCancel(f.ctx)
			target := func(context.Context, string, string) error {
				if cancelParent {
					cancel()
				} else {
					prepareOrdinaryHandlerFixture(t, ctx, st.Pool, f)
				}
				return nil
			}
			f.svc.OverrideTargets = ownerTargetsForTest{chat: target, voice: target}
			var err error
			if voice {
				_, err = f.svc.SetVoiceRoomOverride(callCtx, &rolev1.SetVoiceRoomOverrideRequest{SpaceId: f.space.String(), VoiceRoomId: f.voice.String(), RoleId: f.custom.String(), AllowMask: 16})
			} else {
				_, err = f.svc.SetChatOverride(callCtx, &rolev1.SetChatOverrideRequest{SpaceId: f.space.String(), Chat: &chatv1.ChatRef{Id: f.chat.String()}, RoleId: f.custom.String(), AllowMask: 16})
			}
			cancel()
			want := codes.Unavailable
			if cancelParent {
				want = codes.Canceled
			}
			require.Equal(t, want, status.Code(err))
			var allow int64
			table, column, node := "chat_overrides", "chat_id", f.chat
			if voice {
				table, column, node = "voice_room_overrides", "voice_room_id", f.voice
			}
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT allow FROM `+table+` WHERE `+column+`=$1 AND role_id=$2`, node, f.custom).Scan(&allow))
			require.EqualValues(t, 1, allow)
			count, _ := f.events.snapshot()
			require.Zero(t, count)
		}
	}
}

func TestRoleOverrideSnapshotSuccessAndRemovalDoesNotRevalidate(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	st, cleanup := startRoleStoreTest(t)
	defer cleanup()
	f := newOrdinaryHandlerFixture(t, ctx, st.Pool)
	calls := 0
	allow := func(context.Context, string, string) error { calls++; return nil }
	f.svc.OverrideTargets = ownerTargetsForTest{chat: allow, voice: allow}
	_, err := f.svc.SetChatOverride(f.ctx, &rolev1.SetChatOverrideRequest{SpaceId: f.space.String(), Chat: &chatv1.ChatRef{Id: f.chat.String()}, RoleId: f.custom.String(), AllowMask: 16})
	require.NoError(t, err)
	_, err = f.svc.SetVoiceRoomOverride(f.ctx, &rolev1.SetVoiceRoomOverrideRequest{SpaceId: f.space.String(), VoiceRoomId: f.voice.String(), RoleId: f.custom.String(), AllowMask: 16})
	require.NoError(t, err)
	count, _ := f.events.snapshot()
	require.Equal(t, 2, count)
	require.Equal(t, 2, calls)
	deny := func(context.Context, string, string) error {
		t.Fatal("removal must not call owner validation")
		return nil
	}
	f.svc.OverrideTargets = ownerTargetsForTest{chat: deny, voice: deny}
	_, err = f.svc.RemoveChatOverride(f.ctx, &rolev1.RemoveChatOverrideRequest{SpaceId: f.space.String(), Chat: &chatv1.ChatRef{Id: f.chat.String()}, RoleId: f.custom.String()})
	require.NoError(t, err)
	_, err = f.svc.RemoveVoiceRoomOverride(f.ctx, &rolev1.RemoveVoiceRoomOverrideRequest{SpaceId: f.space.String(), VoiceRoomId: f.voice.String(), RoleId: f.custom.String()})
	require.NoError(t, err)
}

func TestRoleOverrideOwnerReadIsUnlockedAndFinalPermissionIsFresh(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	st, cleanup := startRoleStoreTest(t)
	defer cleanup()
	for _, voice := range []bool{false, true} {
		f := newOrdinaryHandlerFixture(t, ctx, st.Pool)
		calls := 0
		read := func(parent context.Context, spaceID, nodeID string) error {
			calls++
			require.Equal(t, f.space.String(), spaceID)
			bounded, cancel := context.WithTimeout(parent, 2*time.Second)
			defer cancel()
			// This independent Store invocation acquires the same Space advisory lock.
			// It would time out if a Role preflight transaction still held that lock.
			_, err := f.svc.Store.GetEffectiveMask(bounded, f.space, f.owner, nil, nil)
			require.NoError(t, err, "owner RPC must execute without a Role lock held")
			// Revoke all current actor authority between preflight and final commit.
			_, err = st.Pool.Exec(bounded, `DELETE FROM member_roles WHERE space_id=$1 AND profile_id=$2`, f.space, f.owner)
			require.NoError(t, err)
			return nil
		}
		f.svc.OverrideTargets = ownerTargetsForTest{chat: read, voice: read}
		var err error
		if voice {
			_, err = f.svc.SetVoiceRoomOverride(f.ctx, &rolev1.SetVoiceRoomOverrideRequest{SpaceId: f.space.String(), VoiceRoomId: f.voice.String(), RoleId: f.custom.String(), AllowMask: 16})
		} else {
			_, err = f.svc.SetChatOverride(f.ctx, &rolev1.SetChatOverrideRequest{SpaceId: f.space.String(), Chat: &chatv1.ChatRef{Id: f.chat.String()}, RoleId: f.custom.String(), AllowMask: 16})
		}
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.Equal(t, 1, calls)
		var allow, deny int64
		table, column, node := "chat_overrides", "chat_id", f.chat
		if voice {
			table, column, node = "voice_room_overrides", "voice_room_id", f.voice
		}
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT allow,deny FROM `+table+` WHERE `+column+`=$1 AND role_id=$2`, node, f.custom).Scan(&allow, &deny))
		require.EqualValues(t, 1, allow)
		require.EqualValues(t, 2, deny)
		count, _ := f.events.snapshot()
		require.Zero(t, count)
	}
}

func TestRoleOverridesRequireOwnerValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	st, cleanup := startRoleStoreTest(t)
	defer cleanup()
	f := newOrdinaryHandlerFixture(t, ctx, st.Pool)
	before := ordinaryHandlerSnapshot(t, ctx, st.Pool)
	_, err := f.svc.SetChatOverride(f.ctx, &rolev1.SetChatOverrideRequest{SpaceId: f.space.String(), Chat: &chatv1.ChatRef{Id: f.chat.String()}, RoleId: f.custom.String(), AllowMask: 16})
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = f.svc.SetVoiceRoomOverride(f.ctx, &rolev1.SetVoiceRoomOverrideRequest{SpaceId: f.space.String(), VoiceRoomId: f.voice.String(), RoleId: f.custom.String(), AllowMask: 16})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, before, ordinaryHandlerSnapshot(t, ctx, st.Pool))
	count, _ := f.events.snapshot()
	require.Zero(t, count)
}
