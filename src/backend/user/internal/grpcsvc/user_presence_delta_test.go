package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	eventsv1 "voice.app/voice/events/v1"
	userv1 "voice.app/voice/user/v1"
)

type presenceTransitionEvent struct {
	profileID string
	oldStatus string
	newStatus string
}

// presenceDeltaEventsRecorder is deliberately private to these presence
// transition tests, so lifecycle-test fakes remain unrelated to this contract.
type presenceDeltaEventsRecorder struct {
	events []presenceTransitionEvent
}

func (r *presenceDeltaEventsRecorder) PublishProfileCreated(context.Context, string, string) error {
	return nil
}

func (r *presenceDeltaEventsRecorder) PublishProfileUpdated(context.Context, string, string, string) error {
	return nil
}

func (r *presenceDeltaEventsRecorder) PublishProfileSwitched(context.Context, string, string, string) error {
	return nil
}

func (r *presenceDeltaEventsRecorder) PublishVerified(context.Context, string, string, string) error {
	return nil
}

func (r *presenceDeltaEventsRecorder) PublishPresenceChanged(_ context.Context, profileID, oldStatus, newStatus string) error {
	r.events = append(r.events, presenceTransitionEvent{
		profileID: profileID,
		oldStatus: oldStatus,
		newStatus: newStatus,
	})
	return nil
}

func TestUpdatePresence_PublishesOnlyCanonicalEnumTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)

	accountID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	profileID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'presence_delta', '0065', 'Presence Delta', true)`,
		profileID, accountID)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	events := &presenceDeltaEventsRecorder{}
	svc := &UserGRPC{
		Profiles: store.NewProfileStore(pool),
		Presence: store.NewPresenceStore(rdb),
		Events:   events,
	}
	authed := presenceDeltaAuthContext(ctx, accountID, profileID)

	// The first live observation has no predecessor and must publish canonical
	// values even when the legacy string input uses a different case.
	_, err = svc.UpdatePresence(authed, &userv1.UpdatePresenceRequest{
		Status:    "ONLINE",
		GameTitle: proto.String("Dota 2"),
	})
	require.NoError(t, err)
	require.Equal(t, []presenceTransitionEvent{{
		profileID: profileID.String(),
		oldStatus: "",
		newStatus: "online",
	}}, events.events)

	// A same-enum heartbeat changes activity fields and refreshes Redis TTLs but
	// must not become a duplicate transition event.
	mr.FastForward(4 * time.Minute)
	_, err = svc.UpdatePresence(authed, &userv1.UpdatePresenceRequest{
		Status:       "online",
		GameTitle:    proto.String("Counter-Strike 2"),
		CustomStatus: proto.String("queueing"),
		CallInfoJson: proto.String(`{"room":"delta"}`),
	})
	require.NoError(t, err)
	require.Len(t, events.events, 1, "same status enum heartbeat must not publish user.presence_changed")

	sessionTTL, err := rdb.TTL(ctx, "voice:user:presence:"+profileID.String()).Result()
	require.NoError(t, err)
	require.Greater(t, sessionTTL, 4*time.Minute, "heartbeat must refresh the five-minute live session TTL")
	lastSeenTTL, err := rdb.TTL(ctx, "voice:user:last_seen:"+profileID.String()).Result()
	require.NoError(t, err)
	require.Greater(t, lastSeenTTL, 29*24*time.Hour, "heartbeat must refresh the thirty-day interim last_seen TTL")

	// The enum is canonical: DND wins over the conflicting legacy string and
	// reports the prior online enum as the old side of the transition.
	_, err = svc.UpdatePresence(authed, &userv1.UpdatePresenceRequest{
		Status:     "ONLINE",
		StatusEnum: userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND.Enum(),
	})
	require.NoError(t, err)
	require.Equal(t, []presenceTransitionEvent{
		{profileID: profileID.String(), oldStatus: "", newStatus: "online"},
		{profileID: profileID.String(), oldStatus: "online", newStatus: "dnd"},
	}, events.events)

	snap, err := svc.Presence.Get(ctx, profileID)
	require.NoError(t, err)
	require.Equal(t, "dnd", snap.Status)
	require.Equal(t, int32(userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND), snap.StatusEnum)
}

func TestPresenceChangeProto_DeltaFieldsRoundTripWithLegacyCurrentStatus(t *testing.T) {
	want := &eventsv1.PresenceChange{
		ProfileId: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Status:    "dnd", // legacy consumers continue to see current/new status.
		OldStatus: "online",
		NewStatus: "dnd",
	}

	b, err := proto.Marshal(want)
	require.NoError(t, err)
	var got eventsv1.PresenceChange
	require.NoError(t, proto.Unmarshal(b, &got))
	require.Equal(t, want.GetProfileId(), got.GetProfileId())
	require.Equal(t, "dnd", got.GetStatus())
	require.Equal(t, "online", got.GetOldStatus())
	require.Equal(t, "dnd", got.GetNewStatus())
}

func presenceDeltaAuthContext(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.Pairs(
		authctx.HeaderUserID, accountID.String(),
		authctx.HeaderProfileID, profileID.String(),
	))
}
