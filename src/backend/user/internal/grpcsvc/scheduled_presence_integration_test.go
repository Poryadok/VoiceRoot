package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/privacy"
	"voice/backend/user/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	userv1 "voice.app/voice/user/v1"
)

func TestScheduledMessageDispatchPresenceUsesCurrentDMPrivacyAndExactOnline(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)
	profiles, privacyStore := store.NewProfileStore(pool), store.NewPrivacyStore(pool)
	senderAccount, senderProfile := uuid.New(), uuid.New()
	recipientAccount, recipientProfile := uuid.New(), uuid.New()
	seedScheduledPresenceProfile(t, ctx, pool, senderAccount, senderProfile, true, "schedule-guest")
	seedScheduledPresenceProfile(t, ctx, pool, recipientAccount, recipientProfile, false, "schedule-recipient")

	settings := privacy.SettingsForPreset("gaming")
	settings.AllowDM = privacy.EveryoneWithGuests()
	settings.ShowOnline = privacy.EveryoneWithGuests()
	settings.AllowGuestDM = true
	_, err := privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	presence := store.NewPresenceStore(rdb)
	_, err = presence.UpsertAndGetPrevious(ctx, recipientProfile, store.PresenceUpsert{
		Status: "online", StatusEnum: int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE), Now: time.Now().UTC(),
	})
	require.NoError(t, err)

	svc := &UserGRPC{
		Profiles: profiles, Privacy: privacyStore, Presence: presence,
		Blocks: stubProfileBlocks{}, SocialGraph: alwaysFriendsGraph{},
		SpaceCoMembership: stubSpaceCoMembership{}, DeletedAccounts: &deletedAccountCheckerStub{},
	}
	peerProfileID := recipientProfile.String()
	req := &userv1.GetScheduledMessageDispatchPresenceRequest{
		ScheduledMessageId: uuid.NewString(), SenderAccountId: senderAccount.String(), SenderProfileId: senderProfile.String(),
		ChatId: uuid.NewString(), RecipientProfileId: &peerProfileID, ScheduleGeneration: 1,
		Mode:     userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_WHEN_ONLINE,
		ChatType: chatv1.ChatType_CHAT_TYPE_DM,
	}
	call := func() *userv1.GetScheduledMessageDispatchPresenceResponse {
		t.Helper()
		resp, callErr := svc.scheduledMessageDispatchPresence(ctx, req)
		require.NoError(t, callErr)
		return resp
	}

	require.True(t, call().GetUserPolicyAllowsDispatch(), "a guest allowed by the current DM policy may dispatch")
	require.True(t, call().GetSenderIsGuest(), "the User result includes current persisted sender account type")
	require.True(t, call().GetRecipientOnline(), "the online mode reports exact visible ONLINE")

	t.Run("soft-deleted peer is not eligible", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE profiles SET deleted_at = now() WHERE id = $1`, recipientProfile)
		require.NoError(t, err)
		decision := call()
		require.False(t, decision.GetUserPolicyAllowsDispatch())
		require.False(t, decision.GetRecipientOnline())
		_, err = pool.Exec(ctx, `UPDATE profiles SET deleted_at = NULL WHERE id = $1`, recipientProfile)
		require.NoError(t, err)
	})

	settings.AllowGuestDM = false
	_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
	require.NoError(t, err)
	decision := call()
	require.False(t, decision.GetUserPolicyAllowsDispatch(), "the current guest DM restriction suppresses delivery")
	require.False(t, decision.GetRecipientOnline())

	settings.AllowGuestDM = true
	settings.ShowOnline = privacy.Nobody()
	_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
	require.NoError(t, err)
	decision = call()
	require.True(t, decision.GetUserPolicyAllowsDispatch(), "presence visibility does not change current DM policy eligibility")
	require.False(t, decision.GetRecipientOnline(), "hidden presence is never reported online")

	t.Run("at mode ignores presence visibility and availability", func(t *testing.T) {
		oldMode, oldPresence, oldSocial, oldBlocks := req.Mode, svc.Presence, svc.SocialGraph, svc.Blocks
		t.Cleanup(func() {
			req.Mode, svc.Presence, svc.SocialGraph, svc.Blocks = oldMode, oldPresence, oldSocial, oldBlocks
		})
		req.Mode = userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_AT
		svc.Presence = nil
		settings.ShowOnline = privacy.FriendsOnly()
		_, err := privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
		require.NoError(t, err)
		svc.SocialGraph = failingScheduledPresenceSocialGraph{}
		resp, callErr := svc.scheduledMessageDispatchPresence(ctx, req)
		require.NoError(t, callErr, "at mode does not query presence visibility or status")
		require.True(t, resp.GetUserPolicyAllowsDispatch())
		require.False(t, resp.GetRecipientOnline())

		settings.AllowGuestDM = false
		_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
		require.NoError(t, err)
		resp, callErr = svc.scheduledMessageDispatchPresence(ctx, req)
		require.NoError(t, callErr)
		require.False(t, resp.GetUserPolicyAllowsDispatch(), "at mode still enforces the current guest-DM restriction")

		settings.AllowGuestDM = true
		settings.AllowDM = privacy.Nobody()
		_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
		require.NoError(t, err)
		resp, callErr = svc.scheduledMessageDispatchPresence(ctx, req)
		require.NoError(t, callErr)
		require.False(t, resp.GetUserPolicyAllowsDispatch(), "at mode still enforces the current DM audience")

		settings.AllowDM = privacy.EveryoneWithGuests()
		_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
		require.NoError(t, err)
		svc.Blocks = stubProfileBlocks{blocked: map[string]bool{senderAccount.String() + ":" + recipientAccount.String(): true}}
		resp, callErr = svc.scheduledMessageDispatchPresence(ctx, req)
		require.NoError(t, callErr)
		require.False(t, resp.GetUserPolicyAllowsDispatch(), "at mode still enforces current block state")
	})

	t.Run("non-DM at mode validates only the persisted sender", func(t *testing.T) {
		oldRequest, oldBlocks, oldPresence, oldPrivacy, oldSocial := req, svc.Blocks, svc.Presence, svc.Privacy, svc.SocialGraph
		t.Cleanup(func() {
			req, svc.Blocks, svc.Presence, svc.Privacy, svc.SocialGraph = oldRequest, oldBlocks, oldPresence, oldPrivacy, oldSocial
		})
		requestCopy := *req
		requestCopy.ChatType = chatv1.ChatType_CHAT_TYPE_GROUP
		requestCopy.Mode = userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_AT
		requestCopy.RecipientProfileId = nil
		req = &requestCopy
		svc.Blocks, svc.Presence, svc.Privacy, svc.SocialGraph = nil, nil, nil, nil
		resp, callErr := svc.scheduledMessageDispatchPresence(ctx, req)
		require.NoError(t, callErr)
		require.True(t, resp.GetUserPolicyAllowsDispatch(), "User's non-DM result is limited to sender ownership and active-profile validation")
		require.True(t, resp.GetSenderIsGuest(), "Messaging receives the current persisted guest fact")
		require.False(t, resp.GetRecipientOnline())
	})

	settings.ShowOnline = privacy.EveryoneWithGuests()
	settings.AllowDM = privacy.Nobody()
	_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
	require.NoError(t, err)
	decision = call()
	require.False(t, decision.GetUserPolicyAllowsDispatch(), "the current DM audience blocks dispatch")
	require.False(t, decision.GetRecipientOnline())

	settings.AllowDM = privacy.EveryoneWithGuests()
	_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
	require.NoError(t, err)
	svc.Blocks = stubProfileBlocks{blocked: map[string]bool{senderAccount.String() + ":" + recipientAccount.String(): true}}
	decision = call()
	require.False(t, decision.GetUserPolicyAllowsDispatch(), "a current block suppresses dispatch")
	require.False(t, decision.GetRecipientOnline())
	svc.Blocks = stubProfileBlocks{}

	for _, test := range []struct {
		name   string
		status string
		enum   userv1.PresenceOnlineStatus
		want   bool
	}{
		{name: "exact online", status: "online", enum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE, want: true},
		{name: "idle", status: "idle", enum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_IDLE},
		{name: "do not disturb", status: "dnd", enum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND},
		{name: "invisible", status: "invisible", enum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_INVISIBLE},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := presence.UpsertAndGetPrevious(ctx, recipientProfile, store.PresenceUpsert{Status: test.status, StatusEnum: int32(test.enum), Now: time.Now().UTC()})
			require.NoError(t, err)
			decision := call()
			require.True(t, decision.GetUserPolicyAllowsDispatch())
			require.Equal(t, test.want, decision.GetRecipientOnline())
		})
	}

	t.Run("missing block authority fails retryably", func(t *testing.T) {
		svc.Blocks = nil
		_, callErr := svc.scheduledMessageDispatchPresence(ctx, req)
		require.Equal(t, codes.Unavailable, status.Code(callErr))
	})

	t.Run("privacy dependency failure remains retryable", func(t *testing.T) {
		svc.Blocks = stubProfileBlocks{}
		_, err := pool.Exec(ctx, `UPDATE profiles SET is_guest_account = false WHERE id = $1`, senderProfile)
		require.NoError(t, err)
		settings.ShowOnline = privacy.FriendsOnly()
		_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(recipientProfile, settings))
		require.NoError(t, err)
		svc.SocialGraph = failingScheduledPresenceSocialGraph{}
		_, callErr := svc.scheduledMessageDispatchPresence(ctx, req)
		require.Equal(t, codes.Unavailable, status.Code(callErr))
	})

	t.Run("changed sender binding is rejected", func(t *testing.T) {
		wrong := *req
		wrong.SenderAccountId = uuid.NewString()
		_, callErr := svc.scheduledMessageDispatchPresence(ctx, &wrong)
		require.Equal(t, codes.PermissionDenied, status.Code(callErr))
	})
}

type failingScheduledPresenceSocialGraph struct{}

func (failingScheduledPresenceSocialGraph) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, context.DeadlineExceeded
}

func (failingScheduledPresenceSocialGraph) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, context.DeadlineExceeded
}

func seedScheduledPresenceProfile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, accountID, profileID uuid.UUID, guest bool, username string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, is_guest_account)
VALUES ($1, $2, $3, '4242', $3, true, $4)`, profileID, accountID, username, guest)
	require.NoError(t, err)
}
