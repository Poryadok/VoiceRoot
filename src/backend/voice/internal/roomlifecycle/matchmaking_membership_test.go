package roomlifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMatchmakingMembershipIdentityValidation(t *testing.T) {
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	base := MatchmakingMembershipIdentity{ProfileID: uuid.New(), AccountID: uuid.New(), RoomID: uuid.New(), MediaEpoch: uuid.New(), SessionEpoch: 2, State: MembershipJoined}
	for _, state := range []MembershipState{MembershipJoining, MembershipJoined, MembershipLeaving, MembershipLeft, MembershipEjected} {
		row := base
		row.State = state
		require.NoError(t, validateMatchmakingMembership(row))
	}
	for _, seconds := range []time.Duration{1, 30} {
		row := base
		row.State, row.ReconnectStartedAt = MembershipReconnecting, &now
		deadline := now.Add(seconds * time.Second)
		row.ReconnectDeadline = &deadline
		require.NoError(t, validateMatchmakingMembership(row))
	}
	for name, mutate := range map[string]func(*MatchmakingMembershipIdentity){
		"reconnect only start": func(row *MatchmakingMembershipIdentity) {
			row.State, row.ReconnectStartedAt = MembershipReconnecting, &now
		},
		"reconnect only deadline": func(row *MatchmakingMembershipIdentity) {
			row.State, row.ReconnectDeadline = MembershipReconnecting, &now
		},
		"joined with deadline": func(row *MatchmakingMembershipIdentity) { row.ReconnectDeadline = &now },
		"negative interval": func(row *MatchmakingMembershipIdentity) {
			end := now.Add(-time.Second)
			row.State, row.ReconnectStartedAt, row.ReconnectDeadline = MembershipReconnecting, &now, &end
		},
		"unknown legacy":        func(row *MatchmakingMembershipIdentity) { row.AccountID, row.SessionEpoch, row.State = uuid.Nil, 0, "" },
		"nil account":           func(row *MatchmakingMembershipIdentity) { row.AccountID = uuid.Nil },
		"nil profile":           func(row *MatchmakingMembershipIdentity) { row.ProfileID = uuid.Nil },
		"nil room":              func(row *MatchmakingMembershipIdentity) { row.RoomID = uuid.Nil },
		"nil media":             func(row *MatchmakingMembershipIdentity) { row.MediaEpoch = uuid.Nil },
		"zero epoch":            func(row *MatchmakingMembershipIdentity) { row.SessionEpoch = 0 },
		"negative epoch":        func(row *MatchmakingMembershipIdentity) { row.SessionEpoch = -1 },
		"unknown state":         func(row *MatchmakingMembershipIdentity) { row.State = "ACTIVE" },
		"reconnect no times":    func(row *MatchmakingMembershipIdentity) { row.State = MembershipReconnecting },
		"joined with reconnect": func(row *MatchmakingMembershipIdentity) { row.ReconnectStartedAt = &now },
		"deadline equal": func(row *MatchmakingMembershipIdentity) {
			row.State, row.ReconnectStartedAt, row.ReconnectDeadline = MembershipReconnecting, &now, &now
		},
		"deadline too long": func(row *MatchmakingMembershipIdentity) {
			end := now.Add(30*time.Second + time.Microsecond)
			row.State, row.ReconnectStartedAt, row.ReconnectDeadline = MembershipReconnecting, &now, &end
		},
	} {
		t.Run(name, func(t *testing.T) {
			row := base
			mutate(&row)
			require.ErrorIs(t, validateMatchmakingMembership(row), ErrInvariant)
		})
	}
}

func TestMatchmakingMembershipStoreMissingPool(t *testing.T) {
	for _, store := range []*PostgresLifecycleStore{nil, NewPostgresLifecycleStore(nil)} {
		require.ErrorIs(t, store.CheckMatchmakingMembershipSchema(context.Background()), ErrUnavailable)
		_, found, err := store.LoadMembershipIdentity(context.Background(), uuid.New())
		require.False(t, found)
		require.ErrorIs(t, err, ErrUnavailable)
		_, found, err = store.LoadMatchmakingRoom(context.Background(), uuid.New())
		require.False(t, found)
		require.ErrorIs(t, err, ErrUnavailable)
	}
}
