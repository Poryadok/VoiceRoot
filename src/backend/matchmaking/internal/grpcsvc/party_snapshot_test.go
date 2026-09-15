package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// TestStartSearch_ExternalPartyIDRejectedBeforeState verifies that the public
// field is not an authority boundary. A Voice-owned snapshot is the only
// source of pre-match party membership (docs/todo/backend.md A3).
func TestStartSearch_ExternalPartyIDRejectedBeforeState(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startDB(t, ctx)
	srv := searchTestServer(t, pool)
	profileID := uuid.New()
	ctx = ctxWithProfileAccount(profileID, uuid.New())
	gameID := dotaGameID(t, srv, ctx)
	partyID := uuid.New().String()

	_, err := srv.StartSearch(ctx, &matchmakingv1.StartSearchRequest{
		GameId:       gameID,
		Mode:         "5v5 Ranked",
		CriteriaJson: validDotaCriteriaJSON(),
		PartyId:      &partyID,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	assertNoPartyStartState(t, ctx, pool, srv, profileID, gameID)
}

// These tests freeze the minimal pure-MM validation seam to be implemented by
// the A3 owner. Transport authentication and the Voice RPC are deliberately
// outside this package-level test: StartSearch cannot consume the snapshot
// until the Gateway-to-MM verified-principal path exists.
func TestValidatePartySnapshot(t *testing.T) {
	initiator := uuid.New()
	member := uuid.New()
	roomID := uuid.New()

	t.Run("solo is exactly the initiating profile", func(t *testing.T) {
		err := validatePartySnapshot(PartySnapshot{
			Kind:             PartySnapshotKindSolo,
			MemberProfileIDs: []uuid.UUID{initiator},
		}, initiator)
		require.NoError(t, err)
	})

	for _, tc := range []struct {
		name     string
		snapshot PartySnapshot
	}{
		{
			name: "duplicate initiator",
			snapshot: PartySnapshot{
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{initiator, initiator},
			},
		},
		{
			name: "initiator absent",
			snapshot: PartySnapshot{
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{member},
			},
		},
		{
			name: "non positive roster version",
			snapshot: PartySnapshot{
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				MemberProfileIDs: []uuid.UUID{initiator, member},
			},
		},
		{
			name: "empty room id",
			snapshot: PartySnapshot{
				Kind:             PartySnapshotKindVoiceRoster,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{initiator, member},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePartySnapshot(tc.snapshot, initiator)
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
		})
	}
}

func assertNoPartyStartState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, srv *MatchmakingGRPC, profileID uuid.UUID, gameID string) {
	t.Helper()
	var sessionCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE profile_id = $1`, profileID).Scan(&sessionCount))
	require.Zero(t, sessionCount, "rejected party input must not create a search session")

	parsedGameID, err := uuid.Parse(gameID)
	require.NoError(t, err)
	queued, err := srv.Queue.ListSessionIDs(ctx, parsedGameID, "5v5 Ranked", "eu", 0)
	require.NoError(t, err)
	require.Empty(t, queued, "rejected party input must not enqueue a search")
}
