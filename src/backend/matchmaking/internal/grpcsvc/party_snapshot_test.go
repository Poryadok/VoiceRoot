package grpcsvc

import (
	"bytes"
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
	roomID := uuid.New().String()
	members := []uuid.UUID{initiator, member}
	if bytes.Compare(members[0][:], members[1][:]) > 0 {
		members[0], members[1] = members[1], members[0]
	}

	t.Run("solo is exactly the initiating profile", func(t *testing.T) {
		err := validatePartySnapshot(PartySnapshot{
			ProtocolVersion:  1,
			Kind:             PartySnapshotKindSolo,
			MemberProfileIDs: []uuid.UUID{initiator},
		}, initiator)
		require.NoError(t, err)
	})

	t.Run("voice roster has canonical members", func(t *testing.T) {
		err := validatePartySnapshot(PartySnapshot{
			ProtocolVersion:  1,
			Kind:             PartySnapshotKindVoiceRoster,
			RoomID:           roomID,
			RosterVersion:    1,
			MemberProfileIDs: members,
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
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{initiator, initiator},
			},
		},
		{
			name: "duplicate non initiator",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{initiator, member, member},
			},
		},
		{
			name: "initiator absent",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{member},
			},
		},
		{
			name: "non positive roster version",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				MemberProfileIDs: []uuid.UUID{initiator, member},
			},
		},
		{
			name: "empty room id",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindVoiceRoster,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{initiator, member},
			},
		},
		{
			name: "malformed room id",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           "not-a-uuid",
				RosterVersion:    1,
				MemberProfileIDs: members,
			},
		},
		{
			name: "nil room id",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           uuid.Nil.String(),
				RosterVersion:    1,
				MemberProfileIDs: members,
			},
		},
		{
			name: "unsorted members",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{members[1], members[0]},
			},
		},
		{
			name: "nil member",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{uuid.Nil, initiator},
			},
		},
		{
			name: "unsupported protocol version",
			snapshot: PartySnapshot{
				ProtocolVersion:  2,
				Kind:             PartySnapshotKindVoiceRoster,
				RoomID:           roomID,
				RosterVersion:    1,
				MemberProfileIDs: members,
			},
		},
		{
			name: "unknown kind",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             "OTHER",
				MemberProfileIDs: []uuid.UUID{initiator},
			},
		},
		{
			name: "solo has room",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindSolo,
				RoomID:           roomID,
				MemberProfileIDs: []uuid.UUID{initiator},
			},
		},
		{
			name: "solo has roster version",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindSolo,
				RosterVersion:    1,
				MemberProfileIDs: []uuid.UUID{initiator},
			},
		},
		{
			name: "solo has extra member",
			snapshot: PartySnapshot{
				ProtocolVersion:  1,
				Kind:             PartySnapshotKindSolo,
				MemberProfileIDs: members,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePartySnapshot(tc.snapshot, initiator)
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
		})
	}
}

// Voice owns membership eligibility, room activity and atomic snapshot versus
// mutation behaviour. Those cross-service cases require its protected RPC and
// belong to the Voice integration suite; this MM seam validates only the
// already returned contract shape before any persistence is attempted.

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
