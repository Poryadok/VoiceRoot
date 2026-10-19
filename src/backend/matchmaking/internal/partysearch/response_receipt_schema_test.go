package partysearch

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"voice/backend/matchmaking/internal/store"
)

func TestResponseReceiptDownWaitsForNoncooperatingFirstWriter(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	proposal, err := createProvisionProposalFixture(t, pool, &store.MatchStore{Pool: pool}, partyProposal(p, party.Sessions))
	require.NoError(t, err)
	row := proposal.Proposals[0]
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer writer.Rollback(ctx)
	// Unknown provenance deliberately tests retention, never source authority.
	_, err = writer.Exec(ctx, `INSERT INTO matchmaking_match_response_receipts(actor_account_id,actor_profile_id,operation_id,request_sha256,match_id,proposal_id,search_session_id,session_generation,actor_session_epoch,accept,source_cut_sha256,source_manifest_sha256,source_origin_bytes,source_origin_sha256,outcome)VALUES($1,$2,$3,$4,$5,$6,$7,0,7,true,$4,$4,$8,$4,'ACCEPTED')`, p.InitiatorAccountID, row.ProfileID, uuid.New(), make([]byte, 32), proposal.Match.ID, row.ID, row.SearchSessionID, []byte("unknown pending evidence"))
	require.NoError(t, err)
	reader, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer reader.Release()
	var pid int
	require.NoError(t, reader.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	down := projectionMigration(t, "down")
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		_, e := reader.Exec(bounded, down)
		done <- e
	}()
	require.Eventually(t, func() bool {
		select {
		case e := <-done:
			t.Fatalf("DOWN bypassed receipt writer: %v", e)
		default:
		}
		var waiting bool
		return pool.QueryRow(ctx, `SELECT wait_event_type='Lock' FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&waiting) == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.ErrorContains(t, <-done, "evidence must be retained")
	_, err = reader.Exec(ctx, `ROLLBACK`)
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_response_receipts`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestResponseReceiptSchemaRetainsUnknownEvidenceOnRollback(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.matchmaking_match_response_receipts') IS NOT NULL`).Scan(&exists))
	require.True(t, exists, "same MM026 must implement the durable c117 decision receipt")
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	proposal, err := createProvisionProposalFixture(t, pool, &store.MatchStore{Pool: pool}, partyProposal(p, party.Sessions))
	require.NoError(t, err)
	row := proposal.Proposals[0]
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_match_response_receipts(actor_account_id,actor_profile_id,operation_id,request_sha256,match_id,proposal_id,search_session_id,session_generation,actor_session_epoch,accept,source_cut_sha256,source_manifest_sha256,source_origin_bytes,source_origin_sha256,outcome)VALUES($1,$2,$3,$4,$5,$6,$7,0,7,true,$4,$4,$8,$4,'ACCEPTED')`, p.InitiatorAccountID, row.ProfileID, uuid.New(), make([]byte, 32), proposal.Match.ID, row.ID, row.SearchSessionID, []byte("unknown retained provenance fixture"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, projectionMigration(t, "down"))
	require.Error(t, err, "missing head does not permit destruction of an unknown retained response receipt")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_response_receipts`).Scan(&count))
	require.Equal(t, 1, count)
}
