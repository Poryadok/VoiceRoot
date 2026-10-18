package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

func rosterResetDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	for _, name := range []string{"000019_a3_party_search.up.sql", "000020_voice_roster_inbox.up.sql"} {
		b, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(b))
		require.NoError(t, err)
	}
	return pool
}

func TestVoiceRosterInboxDowngradePreservesSettledEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	room, profile := uuid.New(), uuid.New()
	capturedRosterParty(t, pool, &room, 1, profile)
	_, err := (&MatchStore{Pool: pool}).ApplyVoiceRosterEvent(ctx, rosterBytes(t, uuid.New(), room, profile, 2, false))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE matchmaking_voice_roster_cancellation_outbox SET delivered_at=clock_timestamp()`)
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000020_voice_roster_inbox.down.sql"))
	require.NoError(t, err)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, string(down))
	require.ErrorContains(t, err, "cannot remove Voice roster inbox or cancellation evidence")
	_, err = conn.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	var inbox, outbox int64
	require.NoError(t, conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM matchmaking_voice_roster_inbox),(SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox)`).Scan(&inbox, &outbox))
	require.EqualValues(t, 1, inbox)
	require.EqualValues(t, 1, outbox)
}

func capturedRosterParty(t *testing.T, pool *pgxpool.Pool, room *uuid.UUID, version int64, profiles ...uuid.UUID) (uuid.UUID, []SearchSession) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	var game uuid.UUID
	require.NoError(t, tx.QueryRow(ctx, `SELECT id FROM games WHERE status='active' ORDER BY id LIMIT 1`).Scan(&game))
	ids := make([]string, len(profiles))
	for i, p := range profiles {
		ids[i] = p.String()
	}
	sort.Strings(ids)
	manifest, err := json.Marshal(ids)
	require.NoError(t, err)
	hash := sha256.Sum256(manifest)
	party, op := uuid.New(), uuid.New()
	kind := "VOICE_ROSTER"
	if room == nil {
		kind = "SOLO"
	}
	_, err = tx.Exec(ctx, `INSERT INTO parties(id,leader_profile_id,member_profile_ids,game_id,mode,criteria,search_operation_id,search_request_sha256,initiator_account_id,initiator_session_epoch,voice_snapshot_protocol_version,voice_source_kind,voice_source_room_id,voice_source_roster_version,voice_member_manifest_sha256)
VALUES($1,$2,$3::jsonb,$4,'test','{"region":"eu"}',$5,$6,$7,7,1,$8,$9,$10,$6)`, party, profiles[0], string(manifest), game, op, hash[:], uuid.New(), kind, room, version)
	require.NoError(t, err)
	sessions := make([]SearchSession, 0, len(profiles))
	for _, profile := range profiles {
		sess, err := scanSession(tx.QueryRow(ctx, `INSERT INTO search_sessions(id,profile_id,party_id,game_id,mode,criteria,status,timeout_at) VALUES($1,$2,$3,$4,'test','{"region":"eu"}','searching',clock_timestamp()+interval '30 minutes') RETURNING `+sessionSelectCols, uuid.New(), profile, party, game))
		require.NoError(t, err)
		sessions = append(sessions, sess)
	}
	_, err = tx.Exec(ctx, `INSERT INTO matchmaking_party_search_queue_intents(party_id,action) VALUES($1,'ENQUEUE')`, party)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	return party, sessions
}

func rosterBytes(t *testing.T, event, room, profile uuid.UUID, version uint64, joined bool) []byte {
	t.Helper()
	change := eventsv1.VoiceRosterMembershipChanged_CHANGE_LEFT
	if joined {
		change = eventsv1.VoiceRosterMembershipChanged_CHANGE_JOINED
	}
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(&eventsv1.VoiceStreamEvent{EventId: event.String(), OccurredAt: timestamppb.New(time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)), Payload: &eventsv1.VoiceStreamEvent_VoiceRosterMembershipChanged{VoiceRosterMembershipChanged: &eventsv1.VoiceRosterMembershipChanged{ProtocolVersion: 1, RoomId: room.String(), RosterVersion: version, Change: change, ProfileId: profile.String()}}})
	require.NoError(t, err)
	return b
}

func TestVoiceRosterInboxResetsWholeRoomPartyAndSoloExactlyOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	st := &MatchStore{Pool: pool}
	room, foreignRoom, newMember := uuid.New(), uuid.New(), uuid.New()
	party, members := capturedRosterParty(t, pool, &room, 5, uuid.New(), uuid.New())
	_, unrelated := capturedRosterParty(t, pool, &foreignRoom, 5, uuid.New())
	solo, soloMembers := capturedRosterParty(t, pool, nil, 0, newMember)
	raw := rosterBytes(t, uuid.New(), room, newMember, 6, true)
	result, err := st.ApplyVoiceRosterEvent(ctx, raw)
	require.NoError(t, err)
	require.False(t, result.ContractMismatch)
	require.Equal(t, 3, result.CancelledSessions)
	for _, member := range append(members, soloMembers...) {
		sess, err := (&SessionStore{Pool: pool}).Get(ctx, member.ID)
		require.NoError(t, err)
		require.Equal(t, SessionStatusCancelled, sess.Status)
		require.EqualValues(t, 1, sess.RecoveryGeneration)
	}
	sess, err := (&SessionStore{Pool: pool}).Get(ctx, unrelated[0].ID)
	require.NoError(t, err)
	require.Equal(t, SessionStatusSearching, sess.Status)
	for _, id := range []uuid.UUID{party, solo} {
		var action string
		var revision int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT action,revision FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, id).Scan(&action, &revision))
		require.Equal(t, "RELEASE", action)
		require.EqualValues(t, 2, revision)
	}
	result, err = st.ApplyVoiceRosterEvent(ctx, raw)
	require.NoError(t, err)
	require.Zero(t, result.CancelledSessions)
	old := rosterBytes(t, uuid.New(), foreignRoom, uuid.New(), 4, false)
	result, err = st.ApplyVoiceRosterEvent(ctx, old)
	require.NoError(t, err)
	require.Zero(t, result.CancelledSessions)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox`).Scan(&count))
	require.Equal(t, 3, count)
}

func TestVoiceRosterInboxCancelsAcceptedPartyAndRecoversUnrelatedPendingParty(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	st := &MatchStore{Pool: pool}
	room := uuid.New()
	target, members := capturedRosterParty(t, pool, &room, 5, uuid.New(), uuid.New())
	other, unrelated := capturedRosterParty(t, pool, nil, 0, uuid.New())
	all := append(append([]SearchSession{}, members...), unrelated...)
	params := CreateProposalParams{GameID: members[0].GameID, Mode: "test", Region: "eu"}
	for _, sess := range all {
		params.Sessions = append(params.Sessions, ProposalSession{SessionID: sess.ID, ProfileID: sess.ProfileID, PartyID: sess.PartyID})
	}
	proposal, err := st.CreateProposal(ctx, params)
	require.NoError(t, err)
	_, err = st.RecordAcceptance(ctx, proposal.Match.ID, members[0].ProfileID)
	require.NoError(t, err)
	result, err := st.ApplyVoiceRosterEvent(ctx, rosterBytes(t, uuid.New(), room, uuid.New(), 6, false))
	require.NoError(t, err)
	require.Equal(t, 2, result.CancelledSessions)
	for _, member := range members {
		sess, err := (&SessionStore{Pool: pool}).Get(ctx, member.ID)
		require.NoError(t, err)
		require.Equal(t, SessionStatusCancelled, sess.Status)
	}
	sess, err := (&SessionStore{Pool: pool}).Get(ctx, unrelated[0].ID)
	require.NoError(t, err)
	require.Equal(t, SessionStatusSearching, sess.Status)
	require.Nil(t, sess.MatchID)
	for _, want := range []struct {
		id     uuid.UUID
		action string
	}{{target, "RELEASE"}, {other, "ENQUEUE"}} {
		var action string
		require.NoError(t, pool.QueryRow(ctx, `SELECT action FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, want.id).Scan(&action))
		require.Equal(t, want.action, action)
	}
}

func TestVoiceRosterInboxConflictingBytesCancelNewerCaptureAndPersistAlert(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	st := &MatchStore{Pool: pool}
	room, event := uuid.New(), uuid.New()
	_, err := st.ApplyVoiceRosterEvent(ctx, rosterBytes(t, event, room, uuid.New(), 6, true))
	require.NoError(t, err)
	_, members := capturedRosterParty(t, pool, &room, 9, uuid.New())
	result, err := st.ApplyVoiceRosterEvent(ctx, rosterBytes(t, event, room, uuid.New(), 6, true))
	require.NoError(t, err)
	require.True(t, result.ContractMismatch)
	require.Equal(t, 1, result.CancelledSessions)
	sess, err := (&SessionStore{Pool: pool}).Get(ctx, members[0].ID)
	require.NoError(t, err)
	require.Equal(t, SessionStatusCancelled, sess.Status)
	var conflicts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_contract_conflicts`).Scan(&conflicts))
	require.Equal(t, 1, conflicts)
}

func TestVoiceRosterChangedCandidateResetsAllPartiesAtomically(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	st := &MatchStore{Pool: pool}
	a, am := capturedRosterParty(t, pool, nil, 0, uuid.New())
	b, bm := capturedRosterParty(t, pool, nil, 0, uuid.New())
	params := CreateProposalParams{GameID: am[0].GameID, Mode: "test", Region: "eu"}
	for _, sess := range append(am, bm...) {
		params.Sessions = append(params.Sessions, ProposalSession{SessionID: sess.ID, ProfileID: sess.ProfileID, PartyID: sess.PartyID})
	}
	proposal, err := st.CreateProposal(ctx, params)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE matchmaking_voice_roster_cancellation_outbox ADD CONSTRAINT reject_candidate_cancel CHECK(false)`)
	require.NoError(t, err)
	_, err = st.ResetVoiceParties(ctx, []uuid.UUID{b, a, b})
	require.Error(t, err, "one failed cancellation must roll back every party")
	for _, member := range append(am, bm...) {
		sess, err := (&SessionStore{Pool: pool}).Get(ctx, member.ID)
		require.NoError(t, err)
		require.Equal(t, SessionStatusPendingAccept, sess.Status)
	}
	_, err = pool.Exec(ctx, `ALTER TABLE matchmaking_voice_roster_cancellation_outbox DROP CONSTRAINT reject_candidate_cancel`)
	require.NoError(t, err)
	result, err := st.ResetVoiceParties(ctx, []uuid.UUID{b, a, b})
	require.NoError(t, err)
	require.Equal(t, 2, result.CancelledSessions)
	match, err := st.Get(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.Equal(t, MatchStatusAbandoned, match.Status)
	for _, member := range append(am, bm...) {
		sess, err := (&SessionStore{Pool: pool}).Get(ctx, member.ID)
		require.NoError(t, err)
		require.Equal(t, SessionStatusCancelled, sess.Status)
	}
	result, err = st.ResetVoiceParties(ctx, []uuid.UUID{a, b})
	require.NoError(t, err)
	require.Zero(t, result.CancelledSessions)
}

func TestVoiceRosterResetOutboxFailureRollsBackInboxAndQueueIntent(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	st := &MatchStore{Pool: pool}
	room := uuid.New()
	party, members := capturedRosterParty(t, pool, &room, 5, uuid.New())
	_, err := pool.Exec(ctx, `ALTER TABLE matchmaking_voice_roster_cancellation_outbox ADD CONSTRAINT reject_fixture_event CHECK(false)`)
	require.NoError(t, err)
	raw := rosterBytes(t, uuid.New(), room, uuid.New(), 6, false)
	_, err = st.ApplyVoiceRosterEvent(ctx, raw)
	require.Error(t, err)
	sess, err := (&SessionStore{Pool: pool}).Get(ctx, members[0].ID)
	require.NoError(t, err)
	require.Equal(t, SessionStatusSearching, sess.Status)
	require.Zero(t, sess.RecoveryGeneration)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_inbox`).Scan(&count))
	require.Zero(t, count)
	var action string
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT action,revision FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party).Scan(&action, &revision))
	require.Equal(t, "ENQUEUE", action)
	require.EqualValues(t, 1, revision)
	_, err = pool.Exec(ctx, `ALTER TABLE matchmaking_voice_roster_cancellation_outbox DROP CONSTRAINT reject_fixture_event`)
	require.NoError(t, err)
	result, err := st.ApplyVoiceRosterEvent(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, 1, result.CancelledSessions)
	first, err := st.ClaimVoiceRosterCancellation(ctx)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NoError(t, st.FinishVoiceRosterCancellation(ctx, *first, false))
	_, err = pool.Exec(ctx, `UPDATE matchmaking_voice_roster_cancellation_outbox SET next_attempt_at=clock_timestamp()`)
	require.NoError(t, err)
	replay, err := st.ClaimVoiceRosterCancellation(ctx)
	require.NoError(t, err)
	require.NotNil(t, replay)
	require.Equal(t, first.EventID, replay.EventID)
	require.Equal(t, first.Payload, replay.Payload)
	require.NotEqual(t, first.LeaseToken, replay.LeaseToken)
	require.ErrorIs(t, st.FinishVoiceRosterCancellation(ctx, *first, true), ErrRosterCancellationLease)
	require.NoError(t, st.FinishVoiceRosterCancellation(ctx, *replay, true))
	env := new(eventsv1.MatchmakingStreamEvent)
	require.NoError(t, proto.Unmarshal(replay.Payload, env))
	require.Equal(t, "VOICE_ROSTER_CHANGED", env.GetSearchCancelled().GetReason())
}

func TestVoiceRosterInboxConcurrentReplayAndOutOfOrderVersionFork(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	st := &MatchStore{Pool: pool}
	room := uuid.New()
	_, members := capturedRosterParty(t, pool, &room, 5, uuid.New(), uuid.New())
	raw := rosterBytes(t, uuid.New(), room, uuid.New(), 7, true)
	type outcome struct {
		result VoiceRosterResetResult
		err    error
	}
	ready := make(chan struct{})
	results := make(chan outcome, 2)
	for range 2 {
		go func() { <-ready; r, e := st.ApplyVoiceRosterEvent(ctx, raw); results <- outcome{r, e} }()
	}
	close(ready)
	total := 0
	for range 2 {
		r := <-results
		require.NoError(t, r.err)
		require.False(t, r.result.ContractMismatch)
		total += r.result.CancelledSessions
	}
	require.Equal(t, len(members), total)
	_, later := capturedRosterParty(t, pool, &room, 9, uuid.New())
	result, err := st.ApplyVoiceRosterEvent(ctx, rosterBytes(t, uuid.New(), room, uuid.New(), 6, false))
	require.NoError(t, err)
	require.Zero(t, result.CancelledSessions)
	result, err = st.ApplyVoiceRosterEvent(ctx, rosterBytes(t, uuid.New(), room, uuid.New(), 7, false))
	require.NoError(t, err)
	require.True(t, result.ContractMismatch)
	require.Equal(t, 1, result.CancelledSessions, "a room/version fork fails closed even for a newer capture")
	sess, err := (&SessionStore{Pool: pool}).Get(ctx, later[0].ID)
	require.NoError(t, err)
	require.Equal(t, SessionStatusCancelled, sess.Status)
}
