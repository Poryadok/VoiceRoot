package grpcsvc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/ordinarycall"
	voicestore "voice/backend/voice/internal/store"
)

type leaveAccountResolver func(context.Context, uuid.UUID) (uuid.UUID, error)

func (f leaveAccountResolver) AccountIDByProfileID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return f(ctx, id)
}

type leaveRecordingMedia struct {
	ordinaryHandlerMedia
	removed []string
}

func (m *leaveRecordingMedia) RemoveParticipant(_ context.Context, _, identity string) error {
	m.removed = append(m.removed, identity)
	return nil
}

func TestOrdinaryDelayedLeaveCannotRemoveReplacementGeneration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Now().UTC()
	owner := ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New()}
	actor := ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New(), SessionEpoch: 3}
	journal, media := &ordinaryHandlerJournal{}, &leaveRecordingMedia{}
	calls := voicestore.NewMemoryCallStore()
	coordinator := &ordinarycall.Coordinator{Journal: journal, Media: media, Projection: NewOrdinaryProjection(calls, journal), Now: func() time.Time { return now }}
	r, err := coordinator.Start(ctx, ordinarycall.CreateIntent{OperationID: uuid.New(), RoomID: uuid.New(), ChatID: uuid.New(), Group: true, Initiator: owner, MediaKind: 1}, nil)
	require.NoError(t, err)
	op := uuid.New()
	g1, err := coordinator.Admit(ctx, r.Intent.RoomID, op, func(r *ordinarycall.Room) error { return r.BeginJoin(op, actor, now) }, nil)
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	svc := &VoiceGRPC{Calls: calls, OrdinaryCalls: coordinator, AccountVoiceProfiles: leaveAccountResolver(func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
		close(entered)
		select {
		case <-release:
			return actor.AccountID, nil
		case <-ctx.Done():
			return uuid.Nil, ctx.Err()
		}
	}), SessionEpochChecker: currentSessionCheckerFunc(func(context.Context, string, int64) error { return nil })}
	actorCtx := principal.WithVerified(ctx, principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "voice", AccountID: actor.AccountID.String(), ProfileID: actor.ProfileID.String(), SessionEpoch: 3})
	result := make(chan error, 1)
	go func() {
		_, err := svc.terminateOrdinaryCall(actorCtx, g1, actor.ProfileID.String(), "leave")
		result <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("Leave did not reach User barrier")
	}
	_, err = coordinator.Change(ctx, r.Intent.RoomID, func(r *ordinarycall.Room) error { return r.DecideLeave(actor.ProfileID, now) })
	require.NoError(t, err)
	op = uuid.New()
	g2, err := coordinator.Admit(ctx, r.Intent.RoomID, op, func(r *ordinarycall.Room) error { return r.BeginJoin(op, actor, now) }, nil)
	require.NoError(t, err)
	require.NotEqual(t, g1.Members[actor.ProfileID.String()].Generation, g2.Members[actor.ProfileID.String()].Generation)
	once.Do(func() { close(release) })
	select {
	case err = <-result:
		require.Error(t, err, "stale G1 request must be rejected")
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	case <-ctx.Done():
		t.Fatal("stale Leave did not finish")
	}
	after, err := journal.Load(ctx, r.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, g2.Members, after.Members, "G2 membership remains unchanged")
	require.Equal(t, g2.Events, after.Events, "no stale participant-left fact")
	require.Equal(t, g2.Effects, after.Effects, "no G2 cleanup intent")
	require.Equal(t, g2.RosterVersion, after.RosterVersion)
	require.Equal(t, []string{g1.Members[actor.ProfileID.String()].MediaIdentity}, media.removed, "only G1 was removed")
}
