package partysearch

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/matchmaking/internal/store"
)

// Only synthetic SQL equality/serialization: not installed controller authority.
func admittedProvisionFixture(t *testing.T) (*pgxpool.Pool, *store.MatchStore, store.CreateSourcePartyParams, store.MatchSquadProvisioningIntent) {
	t.Helper()
	ctx := context.Background()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool, NativeAdmission: &binding}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	st := &store.MatchStore{Pool: pool, NativeAdmission: &binding}
	sessions := make([]store.ProposalSession, 0, len(party.Sessions))
	for _, s := range party.Sessions {
		sessions = append(sessions, store.ProposalSession{SessionID: s.ID, ProfileID: s.ProfileID, PartyID: s.PartyID})
	}
	proposal, err := createProvisionProposalFixture(t, pool, st, store.CreateProposalParams{GameID: p.GameID, Mode: p.Mode, Region: "eu", Sessions: sessions})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	manifest := []byte{}
	participants := []*chatv1.MatchSquadParticipant{}
	for _, profile := range p.MemberProfileIDs {
		manifest = append(manifest, profile[:]...)
		participants = append(participants, &chatv1.MatchSquadParticipant{ProfileId: profile.String()})
	}
	chatOp := uuid.New()
	request, err := proto.MarshalOptions{Deterministic: true}.Marshal(&chatv1.CreateMatchSquadChatRequest{ProtocolVersion: 1, OperationId: chatOp.String(), MatchId: proposal.Match.ID.String(), Participants: participants, ParticipantManifestSha256: dispatchDigest(manifest)})
	require.NoError(t, err)
	return pool, st, p, store.MatchSquadProvisioningIntent{MatchID: proposal.Match.ID, OperationID: uuid.New(), ParticipantManifestBytes: manifest, ParticipantManifestHash: dispatchDigest(manifest), ChatOperationID: chatOp, ChatRequestBytes: request, ChatRequestHash: dispatchDigest(request), VoiceOperationID: uuid.New()}
}

func TestProvisionAdmissionWaitsForCloseBeforeAccounts(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	closer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer closer.Rollback(ctx)
	require.NoError(t, mmadmission.AcquireCloseGate(ctx, closer))
	_, err = closer.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	finished := make(chan error, 1)
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := st.SaveValidatedMatchSquadProvisioningIntent(bounded, candidate, proof, intent)
		finished <- err
	}()
	var pid int
	require.Eventually(t, func() bool {
		select {
		case err := <-finished:
			t.Fatalf("forward transition bypassed held FIRST admission gate: %v", err)
		default:
		}
		err := pool.QueryRow(ctx, `SELECT pid FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%pg_catalog.pg_advisory_xact_lock_shared%' LIMIT 1`).Scan(&pid)
		return err == nil
	}, 3*time.Second, 10*time.Millisecond)
	var granted int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND granted`, pid).Scan(&granted))
	require.Zero(t, granted, "account advisory locks cannot precede waiting admission")
	require.NoError(t, closer.Commit(ctx))
	require.ErrorIs(t, <-finished, mmadmission.ErrUnavailable, "head must be freshly read after the actual closer wait")
	var intents int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&intents))
	require.Zero(t, intents)
}

func TestProvisionAdmissionHeldThroughActualCommit(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err := pool.Exec(ctx, `CREATE FUNCTION wait_provision_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(147003); RETURN NEW; END $$;
 CREATE CONSTRAINT TRIGGER wait_provision_commit AFTER INSERT ON matchmaking_match_squad_operations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION wait_provision_commit()`)
	require.NoError(t, err)
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(ctx)
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(147003)`)
	require.NoError(t, err)
	finished := make(chan error, 1)
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := st.SaveValidatedMatchSquadProvisioningIntent(bounded, candidate, proof, intent)
		finished <- err
	}()
	require.Eventually(t, func() bool {
		var n int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND lower(query)='commit'`).Scan(&n)
		return err == nil && n == 1
	}, 3*time.Second, 10*time.Millisecond)
	closed := make(chan error, 1)
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		tx, err := pool.Begin(bounded)
		if err != nil {
			closed <- err
			return
		}
		defer tx.Rollback(context.Background())
		if err = mmadmission.AcquireCloseGate(bounded, tx); err == nil {
			_, err = tx.Exec(bounded, `UPDATE mm_native_admission_head SET state='CLOSED'`)
		}
		if err == nil {
			err = tx.Commit(bounded)
		}
		closed <- err
	}()
	require.Eventually(t, func() bool {
		select {
		case err := <-closed:
			t.Fatalf("closer crossed uncommitted forward transition: %v", err)
		default:
		}
		var n int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%pg_catalog.pg_advisory_xact_lock(%'`).Scan(&n)
		return err == nil && n == 1
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-finished)
	require.NoError(t, <-closed)
}

func TestProvisionAdmissionRejectsEveryForwardBoundary(t *testing.T) {
	for _, boundary := range []string{"intent", "dispatch", "activation"} {
		t.Run(boundary, func(t *testing.T) {
			ctx := context.Background()
			pool, st, p, intent := admittedProvisionFixture(t)
			if boundary == "dispatch" {
				candidate, proof := dispatchProof(t, st, intent.MatchID, p)
				_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
				require.NoError(t, err)
			}
			if boundary == "activation" {
				dispatchBothReceipts(t, st, p, intent)
			}
			candidate, proof := dispatchProof(t, st, intent.MatchID, p)
			var before string
			require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE((SELECT to_jsonb(o)::text FROM matchmaking_match_squad_operations o WHERE match_id=$1),'absent')`, intent.MatchID).Scan(&before))
			invoke := func() error {
				switch boundary {
				case "intent":
					_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
					return err
				case "dispatch":
					_, err := st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
					return err
				default:
					_, err := st.ActivateValidatedProvisionedMatch(ctx, candidate, proof, intent.OperationID)
					return err
				}
			}
			binding := *st.NativeAdmission
			wrong := binding
			wrong.FreshRouteManifestHash[0]++
			st.NativeAdmission = &wrong
			require.ErrorIs(t, invoke(), mmadmission.ErrUnavailable)
			st.NativeAdmission = nil
			require.ErrorIs(t, invoke(), mmadmission.ErrUnavailable, "begun ADMITTED head is never a nil compatibility path")
			st.NativeAdmission = &binding
			_, err := pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
			require.NoError(t, err)
			require.ErrorIs(t, invoke(), mmadmission.ErrUnavailable)
			var after string
			require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE((SELECT to_jsonb(o)::text FROM matchmaking_match_squad_operations o WHERE match_id=$1),'absent')`, intent.MatchID).Scan(&after))
			require.Equal(t, before, after)
			match, err := st.Get(ctx, intent.MatchID)
			require.NoError(t, err)
			require.Equal(t, store.MatchStatusPendingAccept, match.Status)
		})
	}
}

func TestProvisionAdmissionClosedStillPersistsAbandonedLateReceipt(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.NoError(t, err)
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	var partyID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM parties WHERE search_operation_id=$1`, p.SearchOperationID).Scan(&partyID))
	_, err = st.ResetVoiceParties(ctx, []uuid.UUID{partyID})
	require.NoError(t, err)
	receipt := &chatv1.MatchSquadChatReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: intent.ChatOperationID.String(), MatchId: intent.MatchID.String(), ChatId: uuid.NewString(), ParticipantManifestSha256: intent.ParticipantManifestHash, RequestSha256: intent.ChatRequestHash, CreatedAt: timestamppb.Now()}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	require.NoError(t, err)
	participants := []*chatv1.MatchSquadParticipant{}
	for _, profile := range p.MemberProfileIDs {
		participants = append(participants, &chatv1.MatchSquadParticipant{ProfileId: profile.String()})
	}
	voice, err := proto.MarshalOptions{Deterministic: true}.Marshal(&callsv1.CreateMatchSquadRoomRequest{ProtocolVersion: 1, OperationId: intent.VoiceOperationID.String(), MatchId: intent.MatchID.String(), Participants: participants, ParticipantManifestSha256: intent.ParticipantManifestHash, ChatCreationReceipt: receipt})
	require.NoError(t, err)
	_, err = st.RecordMatchSquadChatReceipt(ctx, intent.MatchID, uuid.MustParse(receipt.ReceiptId), uuid.MustParse(receipt.ChatId), raw, dispatchDigest(voice), voice)
	require.NoError(t, err)
	var disposition, chat, room, state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT provision_disposition,chat_dispatch_state,voice_dispatch_state,state FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&disposition, &chat, &room, &state))
	require.Equal(t, "ABANDONED", disposition)
	require.Equal(t, "RECEIPT_KNOWN", chat)
	require.Equal(t, "NOT_ISSUED", room)
	require.Equal(t, "compensating", state)
	var ready int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_teardown_participants p JOIN matchmaking_match_squad_teardowns a USING(aggregate_id) WHERE a.match_id=$1 AND p.provider='chat' AND p.request_bytes IS NOT NULL`, intent.MatchID).Scan(&ready))
	require.Equal(t, 1, ready)
}
