package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/partyauthority"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../../..")
	require.NoError(t, err)
	return root
}

// The protocol-upgrade cases deliberately apply 021 themselves. Private source
// authority is freshly captured under 022, never backfilled into a legacy row.
// This fixture does not certify a chronological pre-022 production upgrade.
func rosterResetDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BE011_TEST_PG_DSN")
	var pool *pgxpool.Pool
	if dsn == "" {
		pool = store.StartMatchmakingDBForStoreTest(t, ctx)
	} else {
		admin, err := pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		database := "mm_native_fixture_" + uuid.New().String()[:8]
		_, err = admin.Exec(ctx, `CREATE DATABASE `+database)
		require.NoError(t, err)
		config, err := pgxpool.ParseConfig(dsn)
		require.NoError(t, err)
		config.ConnConfig.Database = database
		config.MaxConns = 4
		pool, err = pgxpool.NewWithConfig(ctx, config)
		require.NoError(t, err)
		t.Cleanup(func() {
			pool.Close()
			_, err := admin.Exec(context.Background(), `DROP DATABASE `+database)
			require.NoError(t, err)
			admin.Close()
		})
	}
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/*.up.sql"))
	require.NoError(t, err)
	sort.Strings(files)
	applied := 0
	for _, file := range files {
		prefix := filepath.Base(file)[:6]
		if prefix > "000020" && prefix != "000022" {
			continue
		}
		body, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, file)
		applied++
	}
	require.Equal(t, 21, applied, "001..020 plus fresh private authority022;021 belongs to the tested upgrade")
	return pool
}

func capturedRosterParty(t *testing.T, pool *pgxpool.Pool, room *uuid.UUID, version int64, profiles ...uuid.UUID) (uuid.UUID, []store.SearchSession) {
	t.Helper()
	ctx := context.Background()
	sort.Slice(profiles, func(i, j int) bool { return bytes.Compare(profiles[i][:], profiles[j][:]) < 0 })
	var game uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM games WHERE status='active' ORDER BY id LIMIT 1`).Scan(&game))
	authorities := make([]partyauthority.Member, len(profiles))
	for i, profile := range profiles {
		authorities[i] = partyauthority.Member{ProfileID: profile, AccountID: uuid.New(), SessionEpoch: 7}
	}
	kind := "VOICE_ROSTER"
	if room == nil {
		kind = "SOLO"
	}
	result, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, store.CreateSourcePartyParams{
		SearchOperationID: uuid.New(), SearchRequestHash: [32]byte{1}, InitiatorAccountID: authorities[0].AccountID,
		InitiatorProfileID: profiles[0], InitiatorSessionEpoch: 7, ProtocolVersion: 1, SourceKind: kind,
		SourceRoomID: room, SourceRosterVersion: uint64(version), MemberProfileIDs: profiles,
		MemberAuthorityVersion: 1, MemberAuthorities: authorities, SourceAuthorityHash: [32]byte{2},
		GameID: game, Mode: "test", Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().UTC().Add(30 * time.Minute),
	})
	require.NoError(t, err)
	return result.Party.ID, result.Sessions
}

type sourceSnapshotFunc func(context.Context, partysource.Subject) (partysource.Snapshot, error)

func (f sourceSnapshotFunc) Snapshot(ctx context.Context, s partysource.Subject) (partysource.Snapshot, error) {
	return f(ctx, s)
}

func qualifiedProposal(t *testing.T, pool *pgxpool.Pool, st *store.MatchStore, params store.CreateProposalParams) (store.CreateProposalResult, error) {
	t.Helper()
	ctx := context.Background()
	candidate, err := st.CaptureProposalCandidate(ctx, params)
	require.NoError(t, err)
	parties := candidate.Parties()
	validator := &partysource.Revalidator{Snapshots: sourceSnapshotFunc(func(_ context.Context, subject partysource.Subject) (partysource.Snapshot, error) {
		require.Zero(t, pool.Stat().AcquiredConns(), "source IO must follow the released capture transaction")
		for _, party := range parties {
			if subject.ProfileID != party.LeaderProfileID {
				continue
			}
			require.Equal(t, party.InitiatorAccountID, subject.AccountID)
			require.Equal(t, party.InitiatorSessionEpoch, subject.SessionEpoch)
			require.Equal(t, party.SearchOperationID, subject.SearchOperationID)
			return partysource.Snapshot{ProtocolVersion: party.ProtocolVersion, Kind: party.SourceKind, RoomID: party.SourceRoomID, RosterVersion: party.SourceRosterVersion, MemberProfileIDs: party.MemberProfileIDs, MemberAuthorityVersion: party.MemberAuthorityVersion, MemberAuthorities: party.MemberAuthorities, SourceAuthorityHash: party.SourceAuthorityHash}, nil
		}
		return partysource.Snapshot{}, partysource.ErrUnknown
	})}
	validation, err := validator.ValidateProposalCandidate(ctx, candidate)
	require.NoError(t, err)
	require.NotNil(t, validation.Attempt)
	return st.CreateValidatedProposal(ctx, candidate, validation.Attempt)
}

func sumSHA256(value []byte) []byte { sum := sha256.Sum256(value); return sum[:] }
