package partysearch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/squad"
	"voice/backend/matchmaking/internal/store"
)

func TestProvisionOutcomesNeverExposeResourcesForAbandonedOrMismatchedWork(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	outcome, err := st.GetMatchSquadProvisioningOutcome(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchSquadOutcomeWaitingValidation, outcome.State)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err = st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.NoError(t, err)
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	require.NoError(t, st.AbandonMatch(ctx, intent.MatchID))
	outcome, err = st.GetMatchSquadProvisioningOutcome(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchSquadOutcomeSettlingAbandoned, outcome.State)
	require.Empty(t, outcome.ChatID)
	require.Empty(t, outcome.VoiceRoomID)
	require.NoError(t, st.RecordMatchSquadCreationMismatch(ctx, intent.MatchID, intent.OperationID, "chat", intent.ChatRequestHash, []byte("contradictory receipt")))
	outcome, err = st.GetMatchSquadProvisioningOutcome(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchSquadOutcomeContractMismatch, outcome.State)
	require.Empty(t, outcome.ChatID)
	require.Empty(t, outcome.VoiceRoomID)
	// A distinct never-issued operation closes locally with no synthetic receipt.
	otherPool, other, otherParams, otherIntent := dispatchFixture(t)
	candidate, proof = dispatchProof(t, other, otherIntent.MatchID, otherParams)
	_, err = other.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, otherIntent)
	require.NoError(t, err)
	require.NoError(t, other.AbandonMatch(ctx, otherIntent.MatchID))
	outcome, err = other.GetMatchSquadProvisioningOutcome(ctx, otherIntent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchSquadOutcomeCompensationClosed, outcome.State)
	require.Empty(t, outcome.ChatID)
	require.Empty(t, outcome.VoiceRoomID)
	var count int
	require.NoError(t, otherPool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_teardowns`).Scan(&count))
	require.Zero(t, count)
	// Public compatibility adapters preserve failure and cannot present successful
	// cleanup as provisioning success, even when forwarding infrastructure is absent.
	adapter := &squad.MatchSquadProviderWorker{Store: other}
	typed, typedErr := adapter.ProvisionWithOutcome(ctx, otherIntent.MatchID, otherParams.MemberProfileIDs)
	require.Error(t, typedErr)
	require.Equal(t, store.MatchSquadOutcomeCompensationClosed, typed.State)
	room, chat, compatErr := adapter.Provision(ctx, otherIntent.MatchID, otherParams.MemberProfileIDs)
	require.Error(t, compatErr)
	require.Empty(t, room)
	require.Empty(t, chat)
	// The result reader requires the evidence schema; missing columns do not
	// silently classify an old operation as free of contradictory evidence.
	_, err = pool.Exec(ctx, `ALTER TABLE matchmaking_match_squad_operations DROP COLUMN creation_evidence_version`)
	require.NoError(t, err)
	_, err = st.GetMatchSquadProvisioningOutcome(ctx, intent.MatchID)
	require.ErrorIs(t, err, store.ErrSourcePartyUnavailable)
}

func TestProvisionOutcomeDistinguishesActiveFromKnownCompensation(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	chatID, roomID := dispatchBothReceipts(t, st, p, intent)
	outcome, err := st.GetMatchSquadProvisioningOutcome(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchSquadOutcomeWaitingValidation, outcome.State)
	require.Empty(t, outcome.ChatID)
	require.Empty(t, outcome.VoiceRoomID)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err = st.ActivateValidatedProvisionedMatch(ctx, candidate, proof, intent.OperationID)
	require.NoError(t, err)
	outcome, err = st.GetMatchSquadProvisioningOutcome(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchSquadOutcomeActive, outcome.State)
	require.Equal(t, chatID.String(), outcome.ChatID)
	require.Equal(t, roomID.String(), outcome.VoiceRoomID)
	require.NoError(t, st.RecordMatchSquadCreationMismatch(ctx, intent.MatchID, intent.OperationID, "chat", intent.ChatRequestHash, []byte("late contradiction")))
	outcome, err = st.GetMatchSquadProvisioningOutcome(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchSquadOutcomeContractMismatch, outcome.State)
	require.Empty(t, outcome.ChatID)
	require.Empty(t, outcome.VoiceRoomID)
	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM matches WHERE id=$1`, intent.MatchID).Scan(&status))
	require.Equal(t, string(store.MatchStatusActive), status, "reading a late contradiction cannot auto-end an active winner")
	_, other, otherParams, otherIntent := dispatchFixture(t)
	dispatchBothReceipts(t, other, otherParams, otherIntent)
	require.NoError(t, other.AbandonMatch(ctx, otherIntent.MatchID))
	outcome, err = other.GetMatchSquadProvisioningOutcome(ctx, otherIntent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchSquadOutcomeCompensationPending, outcome.State)
	require.Empty(t, outcome.ChatID)
	require.Empty(t, outcome.VoiceRoomID)
}
