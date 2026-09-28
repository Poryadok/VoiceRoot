package store

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func applyRoleGrantMigrations(t *testing.T, ctx context.Context, st *RoleStore) {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "src", "backend", "migrations", "role_db")
	for _, name := range []string{
		"000011_voice_policy_epoch.up.sql",
		"000012_space_retirement.up.sql",
		"000013_game_session_grants.up.sql",
	} {
		sql, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		_, err = st.Pool.Exec(ctx, string(sql))
		require.NoError(t, err)
	}
}

func startRoleGrantStore(t *testing.T) (*RoleStore, context.Context) {
	t.Helper()
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	st := &RoleStore{Pool: pool}
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	applyRoleGrantMigrations(t, ctx, st)
	t.Cleanup(pool.Close)
	return st, ctx
}

func roleGrantRequest(app, env, session, room, operation uuid.UUID, revision int64, profiles []uuid.UUID, requestHash [32]byte) GameSessionGrantApply {
	return GameSessionGrantApply{
		ApplicationID: app, EnvironmentID: env, SessionID: session, VoiceRoomID: room,
		OperationID: operation, RosterRevision: revision, ProfileIDs: profiles, RequestSHA256: requestHash,
	}
}

func sortedRoleGrantProfiles(profiles ...uuid.UUID) []uuid.UUID {
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].String() < profiles[j].String() })
	return profiles
}

func TestGameSessionGrantApplyReplacesCompleteRosterAndReplaysAfterStoreRestart(t *testing.T) {
	st, ctx := startRoleGrantStore(t)
	app, env, session, room := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	p1, p2, p3 := uuid.New(), uuid.New(), uuid.New()
	profiles := sortedRoleGrantProfiles(p1, p2)
	requestHash := sha256.Sum256([]byte("apply revision 1"))
	first := roleGrantRequest(app, env, session, room, uuid.New(), 1, profiles, requestHash)
	receipt, err := st.ApplyGameSessionGrants(ctx, first)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, receipt.ReceiptID)
	require.Equal(t, requestHash, receipt.RequestSHA256)
	require.Equal(t, int64(1), receipt.RosterRevision)
	require.Equal(t, GameSessionGrantApplied, receipt.Outcome)
	require.True(t, receipt.AppliedProfileSetSHA256 == gameSessionProfileSetHash(profiles))

	for _, profile := range profiles {
		allowed, err := st.CheckGameSessionGrant(ctx, app, env, session, room, profile)
		require.NoError(t, err)
		require.True(t, allowed)
	}
	allowed, err := st.CheckGameSessionGrant(ctx, app, env, session, room, p3)
	require.NoError(t, err)
	require.False(t, allowed)
	for _, mismatched := range [][5]uuid.UUID{
		{uuid.New(), env, session, room, p1},
		{app, uuid.New(), session, room, p1},
		{app, env, uuid.New(), room, p1},
		{app, env, session, uuid.New(), p1},
	} {
		allowed, err = st.CheckGameSessionGrant(ctx, mismatched[0], mismatched[1], mismatched[2], mismatched[3], mismatched[4])
		require.NoError(t, err)
		require.False(t, allowed, "each scope component must match the committed grant")
	}

	// An equal revision with the same complete set is a no-mutation receipt.
	equal := roleGrantRequest(app, env, session, room, uuid.New(), 1, profiles, sha256.Sum256([]byte("equal revision")))
	equalReceipt, err := st.ApplyGameSessionGrants(ctx, equal)
	require.NoError(t, err)
	require.Equal(t, GameSessionGrantReplay, equalReceipt.Outcome)
	require.Equal(t, receipt.AppliedProfileSetSHA256, equalReceipt.AppliedProfileSetSHA256)

	// A higher revision atomically replaces the set; omitted members immediately lose VOICE_JOIN.
	higherProfiles := sortedRoleGrantProfiles(p2, p3)
	higher := roleGrantRequest(app, env, session, room, uuid.New(), 2, higherProfiles, sha256.Sum256([]byte("apply revision 2")))
	_, err = st.ApplyGameSessionGrants(ctx, higher)
	require.NoError(t, err)
	allowed, err = st.CheckGameSessionGrant(ctx, app, env, session, room, p1)
	require.NoError(t, err)
	require.False(t, allowed)
	for _, profile := range []uuid.UUID{p2, p3} {
		allowed, err = st.CheckGameSessionGrant(ctx, app, env, session, room, profile)
		require.NoError(t, err)
		require.True(t, allowed)
	}

	// A lower revision is a durable stale no-op and cannot overwrite the set.
	stale := roleGrantRequest(app, env, session, room, uuid.New(), 1, []uuid.UUID{}, sha256.Sum256([]byte("stale")))
	staleReceipt, err := st.ApplyGameSessionGrants(ctx, stale)
	require.NoError(t, err)
	require.Equal(t, GameSessionGrantStale, staleReceipt.Outcome)
	allowed, err = st.CheckGameSessionGrant(ctx, app, env, session, room, p3)
	require.NoError(t, err)
	require.True(t, allowed)

	// An explicit higher-revision empty snapshot removes every grant.
	empty := roleGrantRequest(app, env, session, room, uuid.New(), 3, []uuid.UUID{}, sha256.Sum256([]byte("complete empty roster")))
	emptyReceipt, err := st.ApplyGameSessionGrants(ctx, empty)
	require.NoError(t, err)
	require.Equal(t, GameSessionGrantApplied, emptyReceipt.Outcome)
	require.Equal(t, gameSessionProfileSetHash(nil), emptyReceipt.AppliedProfileSetSHA256)
	for _, profile := range []uuid.UUID{p1, p2, p3} {
		allowed, err = st.CheckGameSessionGrant(ctx, app, env, session, room, profile)
		require.NoError(t, err)
		require.False(t, allowed)
	}

	// The durable operation receipt is identical after constructing a fresh store, as after process restart.
	restarted := &RoleStore{Pool: st.Pool}
	replayed, err := restarted.ApplyGameSessionGrants(ctx, first)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
}

func TestGameSessionGrantApplyRejectsConflictsAndInvalidCompleteRoster(t *testing.T) {
	st, ctx := startRoleGrantStore(t)
	app, env, session, room := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	op := uuid.New()
	first := roleGrantRequest(app, env, session, room, op, 3, []uuid.UUID{p1}, sha256.Sum256([]byte("first")))
	_, err := st.ApplyGameSessionGrants(ctx, first)
	require.NoError(t, err)

	changedOperation := first
	changedOperation.RequestSHA256 = sha256.Sum256([]byte("changed body under same operation"))
	changedOperation.ProfileIDs = []uuid.UUID{p2}
	_, err = st.ApplyGameSessionGrants(ctx, changedOperation)
	require.ErrorIs(t, err, ErrGameSessionGrantConflict)

	equalRevisionChangedSet := roleGrantRequest(app, env, session, room, uuid.New(), 3, []uuid.UUID{p2}, sha256.Sum256([]byte("equal changed set")))
	_, err = st.ApplyGameSessionGrants(ctx, equalRevisionChangedSet)
	require.ErrorIs(t, err, ErrGameSessionGrantConflict)

	for name, profiles := range map[string][]uuid.UUID{
		"duplicate profile":    {p1, p1},
		"unsorted profile set": {sortedRoleGrantProfiles(p1, p2)[1], sortedRoleGrantProfiles(p1, p2)[0]},
	} {
		t.Run(name, func(t *testing.T) {
			invalid := roleGrantRequest(app, env, uuid.New(), uuid.New(), uuid.New(), 1, profiles, sha256.Sum256([]byte(name)))
			_, err := st.ApplyGameSessionGrants(ctx, invalid)
			require.ErrorIs(t, err, ErrGameSessionGrantInvalid)
		})
	}
}

func TestGameSessionGrantRevokeIsScopedDurableAndTerminal(t *testing.T) {
	st, ctx := startRoleGrantStore(t)
	app, env, session, room, otherSession := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	p1, p2 := uuid.New(), uuid.New()
	_, err := st.ApplyGameSessionGrants(ctx, roleGrantRequest(app, env, session, room, uuid.New(), 1, []uuid.UUID{p1}, sha256.Sum256([]byte("apply one"))))
	require.NoError(t, err)
	_, err = st.ApplyGameSessionGrants(ctx, roleGrantRequest(app, env, otherSession, room, uuid.New(), 1, []uuid.UUID{p2}, sha256.Sum256([]byte("apply two"))))
	require.NoError(t, err)

	operation, requestHash := uuid.New(), sha256.Sum256([]byte("close session"))
	receipt, err := st.RevokeGameSessionGrants(ctx, app, env, session, operation, requestHash)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, receipt.ReceiptID)
	require.Equal(t, requestHash, receipt.RequestSHA256)
	require.Equal(t, GameSessionGrantRevoked, receipt.Outcome)

	allowed, err := st.CheckGameSessionGrant(ctx, app, env, session, room, p1)
	require.NoError(t, err)
	require.False(t, allowed)
	allowed, err = st.CheckGameSessionGrant(ctx, app, env, otherSession, room, p2)
	require.NoError(t, err)
	require.True(t, allowed, "revoking one session cannot remove another session's grant")

	replayed, err := st.RevokeGameSessionGrants(ctx, app, env, session, operation, requestHash)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
	_, err = st.RevokeGameSessionGrants(ctx, app, env, session, operation, sha256.Sum256([]byte("changed close")))
	require.ErrorIs(t, err, ErrGameSessionGrantConflict)

	_, err = st.ApplyGameSessionGrants(ctx, roleGrantRequest(app, env, session, room, uuid.New(), 2, []uuid.UUID{p1}, sha256.Sum256([]byte("late apply"))))
	require.ErrorIs(t, err, ErrGameSessionGrantRevoked)

	restarted := &RoleStore{Pool: st.Pool}
	replayed, err = restarted.RevokeGameSessionGrants(ctx, app, env, session, operation, requestHash)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
}

func TestGameSessionGrantApplyReplacementRollsBackAsOneTransaction(t *testing.T) {
	st, ctx := startRoleGrantStore(t)
	app, env, session, room := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	p1, p2, rejected := uuid.New(), uuid.New(), uuid.New()
	_, err := st.ApplyGameSessionGrants(ctx, roleGrantRequest(app, env, session, room, uuid.New(), 1, []uuid.UUID{p1}, sha256.Sum256([]byte("revision one"))))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `
		CREATE FUNCTION reject_game_session_profile() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.profile_id = '`+rejected.String()+`'::uuid THEN
				RAISE EXCEPTION 'injected grant insert failure';
			END IF;
			RETURN NEW;
		END
		$$;
		CREATE TRIGGER reject_game_session_profile BEFORE INSERT ON game_session_grants
		FOR EACH ROW EXECUTE FUNCTION reject_game_session_profile()`)
	require.NoError(t, err)

	request := roleGrantRequest(app, env, session, room, uuid.New(), 2, sortedRoleGrantProfiles(p2, rejected), sha256.Sum256([]byte("failing revision two")))
	_, err = st.ApplyGameSessionGrants(ctx, request)
	require.Error(t, err)
	allowed, err := st.CheckGameSessionGrant(ctx, app, env, session, room, p1)
	require.NoError(t, err)
	require.True(t, allowed, "the prior complete roster survives a failed replacement")
	allowed, err = st.CheckGameSessionGrant(ctx, app, env, session, room, p2)
	require.NoError(t, err)
	require.False(t, allowed, "a partial replacement must not become visible")
	var revision int64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT roster_revision FROM game_session_grant_sessions WHERE application_id=$1 AND environment_id=$2 AND session_id=$3`, app, env, session).Scan(&revision))
	require.Equal(t, int64(1), revision)
}
