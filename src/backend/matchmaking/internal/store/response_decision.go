package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
	admission "voice/backend/matchmaking/internal/mmadmission"
)

type ResponseValidationProof interface{ Matches(ResponseCandidate) bool }

func (s *MatchStore) RecordValidatedResponse(ctx context.Context, candidate ResponseCandidate, proof ResponseValidationProof) (AcceptanceResult, error) {
	params := candidate.params
	if replay, found, err := s.ReadResponseReceipt(ctx, params); err != nil || found {
		return replay, err
	}
	if s == nil || s.Pool == nil || s.NativeAdmission == nil || candidate.hash == ([32]byte{}) || (params.Accept && (proof == nil || !proof.Matches(candidate))) {
		return AcceptanceResult{}, ErrSourcePartyUnavailable
	}
	accounts := candidate.Accounts()
	parties := candidate.Parties()
	if len(accounts) == 0 || len(parties) == 0 {
		return AcceptanceResult{}, ErrSourcePartyUnavailable
	}
	partyIDs := make([]uuid.UUID, 0, len(parties))
	for _, p := range parties {
		partyIDs = append(partyIDs, p.ID)
	}
	binding := copyNativeBinding(s.NativeAdmission)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return AcceptanceResult{}, err
	}
	defer rollbackPartySearch(tx)
	if params.Accept {
		err = admission.Acquire(ctx, tx, binding)
	} else {
		err = admission.AcquireTerminal(ctx, tx, binding)
	}
	if err != nil {
		return AcceptanceResult{}, err
	}
	guard, err := AcquirePlatformBanGuard(ctx, tx, accounts)
	if err != nil {
		return AcceptanceResult{}, err
	}
	if err = lockMatchSquadReceiptMatchTx(ctx, tx, params.MatchID); err != nil {
		return AcceptanceResult{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM parties WHERE id=ANY($1::uuid[]) ORDER BY id FOR UPDATE`, partyIDs); err != nil {
		return AcceptanceResult{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM search_sessions WHERE match_id=$1 OR party_id=ANY($2::uuid[]) ORDER BY id FOR UPDATE`, params.MatchID, partyIDs); err != nil {
		return AcceptanceResult{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM match_proposals WHERE match_id=$1 ORDER BY id FOR UPDATE`, params.MatchID); err != nil {
		return AcceptanceResult{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT match_id FROM matchmaking_match_squad_operations WHERE match_id=$1 FOR UPDATE`, params.MatchID); err != nil {
		return AcceptanceResult{}, err
	}
	// Account locks serialize operations by this actor even across different
	// matches. A concurrent committed receipt wins before any business mutation.
	if replay, found, err := readResponseReceiptTx(ctx, tx, params); err != nil || found {
		return replay, err
	}
	if params.Accept {
		if err = guard.Check(ctx); err != nil {
			return AcceptanceResult{}, err
		}
	}
	if err = VerifyResponseCandidateTx(ctx, tx, candidate); err != nil {
		return AcceptanceResult{}, err
	}
	match, err := scanMatch(tx.QueryRow(ctx, responseMatchSQL, params.MatchID))
	if err != nil {
		return AcceptanceResult{}, err
	}
	proposals, err := loadMatchProposalsForUpdate(ctx, tx, params.MatchID)
	if err != nil {
		return AcceptanceResult{}, err
	}
	sessions, err := loadMatchSessionsForUpdate(ctx, tx, params.MatchID)
	if err != nil {
		return AcceptanceResult{}, err
	}
	result := AcceptanceResult{Match: match}
	for _, p := range proposals {
		if p.ProfileID == params.ProfileID {
			result.Proposal = p
			break
		}
	}
	if result.Proposal.ID == uuid.Nil {
		return AcceptanceResult{}, ErrProposalNotFound
	}
	var generation int64
	foundSession := false
	for _, session := range sessions {
		if session.ID == result.Proposal.SearchSessionID && session.ProfileID == params.ProfileID {
			generation = session.RecoveryGeneration
			foundSession = true
		}
	}
	if !foundSession {
		return AcceptanceResult{}, ErrSourcePartyUnavailable
	}
	origins, err := readCandidateOriginsMode(ctx, tx, binding, candidate.cut.parties, !params.Accept)
	if err != nil {
		return AcceptanceResult{}, err
	}
	originBytes, err := json.Marshal(origins)
	if err != nil || len(originBytes) > 1048576 {
		return AcceptanceResult{}, ErrSourcePartyUnavailable
	}
	originHash := responseOriginHash(originBytes)
	var manifest [32]byte
	for _, party := range candidate.cut.parties {
		for _, member := range party.MemberAuthorities {
			if member.ProfileID == params.ProfileID && member.AccountID == params.AccountID {
				manifest = party.MemberAuthorityHash
			}
		}
	}
	if manifest == ([32]byte{}) {
		return AcceptanceResult{}, ErrSourcePartyUnavailable
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return AcceptanceResult{}, err
	}
	outcome := "ALREADY_RESPONDED"
	allAccepted := true
	for _, p := range proposals {
		if p.Response != ProposalResponseAccepted {
			allAccepted = false
		}
	}
	switch {
	case params.Accept && !allAccepted && !now.Before(match.CreatedAt.Add(MatchAcceptWindow)):
		result.ChangedSessions, err = expirePendingMatchLocked(ctx, tx, match, proposals, sessions, now)
		result.Match.Status = MatchStatusAbandoned
		result.Match.CompletedAt = &now
		result.Expired = true
		outcome = "EXPIRED"
		if result.Proposal.Response == ProposalResponsePending {
			result.Proposal.Response = ProposalResponseDeclined
			result.Proposal.UpdatedAt = now
		}
	case result.Proposal.Response != ProposalResponsePending:
		result.Replayed = true
	default:
		response := ProposalResponseDeclined
		if params.Accept {
			response = ProposalResponseAccepted
		}
		var tagCount int64
		tag, e := tx.Exec(ctx, `UPDATE match_proposals SET response=$2,updated_at=$3 WHERE id=$1 AND response='pending'`, result.Proposal.ID, response, now)
		err = e
		tagCount = tag.RowsAffected()
		if err == nil && tagCount != 1 {
			err = ErrProvisionCandidateChanged
		}
		if err == nil {
			result.Proposal.Response = response
			result.Proposal.UpdatedAt = now
			if params.Accept {
				outcome = "ACCEPTED"
				result.AllAccepted = true
				for _, p := range proposals {
					if p.ProfileID != params.ProfileID && p.Response != ProposalResponseAccepted {
						result.AllAccepted = false
					}
				}
			} else {
				outcome = "DECLINED"
				result.ChangedSessions, err = recoverDeclinedMatchLocked(ctx, tx, match, proposals, sessions, responsePartyKey(result.Proposal), now, "PROPOSAL_DECLINED")
				result.Match.Status = MatchStatusAbandoned
				result.Match.CompletedAt = &now
				result.Expired = true
			}
		}
	}
	if err != nil {
		return AcceptanceResult{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO matchmaking_match_response_receipts(actor_account_id,actor_profile_id,operation_id,request_sha256,match_id,proposal_id,search_session_id,session_generation,actor_session_epoch,accept,source_cut_sha256,source_manifest_sha256,source_origin_bytes,source_origin_sha256,outcome)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, params.AccountID, params.ProfileID, params.OperationID, params.RequestHash[:], params.MatchID, result.Proposal.ID, result.Proposal.SearchSessionID, generation, params.SessionEpoch, params.Accept, candidate.hash[:], manifest[:], originBytes, originHash[:], outcome)
	if err != nil {
		return AcceptanceResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AcceptanceResult{}, err
	}
	return result, nil
}

const responseMatchSQL = `SELECT id,game_id,mode,region,participants,left_profile_ids,voice_room_id,chat_id,status,created_at,completed_at FROM matches WHERE id=$1`

func validResponseParams(p ResponseParams) bool {
	return p.MatchID != uuid.Nil && p.AccountID != uuid.Nil && p.ProfileID != uuid.Nil && p.OperationID != uuid.Nil && p.SessionEpoch > 0 && p.RequestHash != ([32]byte{})
}
func responseOriginHash(raw []byte) [32]byte {
	return sha256.Sum256(append([]byte("voice.mm.response-origin-evidence.v1\x00"), raw...))
}

// ReadResponseReceipt is an authenticated caller's historical outcome path.
// It neither acquires forward admission nor sets AllAccepted. A fresh JWT may
// retry one operation while CLOSED, but can never re-dispatch a provider plan.
func (s *MatchStore) ReadResponseReceipt(ctx context.Context, params ResponseParams) (AcceptanceResult, bool, error) {
	if s == nil || s.Pool == nil || ctx == nil || !validResponseParams(params) {
		return AcceptanceResult{}, false, ErrSourcePartyUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return AcceptanceResult{}, false, err
	}
	defer rollbackPartySearch(tx)
	result, found, err := readResponseReceiptTx(ctx, tx, params)
	if err != nil {
		return AcceptanceResult{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AcceptanceResult{}, false, err
	}
	return result, found, nil
}

func readResponseReceiptTx(ctx context.Context, tx pgx.Tx, params ResponseParams) (AcceptanceResult, bool, error) {
	var request, cut, manifest, originBytes, originDigest []byte
	var matchID, proposalID, sessionID uuid.UUID
	var epoch, generation int64
	var accept bool
	var outcome string
	err := tx.QueryRow(ctx, `SELECT request_sha256,match_id,proposal_id,search_session_id,session_generation,actor_session_epoch,accept,source_cut_sha256,source_manifest_sha256,source_origin_bytes,source_origin_sha256,outcome FROM matchmaking_match_response_receipts WHERE actor_account_id=$1 AND actor_profile_id=$2 AND operation_id=$3`, params.AccountID, params.ProfileID, params.OperationID).
		Scan(&request, &matchID, &proposalID, &sessionID, &generation, &epoch, &accept, &cut, &manifest, &originBytes, &originDigest, &outcome)
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptanceResult{}, false, nil
	}
	if err != nil {
		return AcceptanceResult{}, false, err
	}
	if !bytes.Equal(request, params.RequestHash[:]) || matchID != params.MatchID || accept != params.Accept || epoch != params.SessionEpoch {
		return AcceptanceResult{}, false, ErrMatchOperationConflict
	}
	if len(cut) != 32 || bytes.Equal(cut, make([]byte, 32)) || len(manifest) != 32 || bytes.Equal(manifest, make([]byte, 32)) || len(originBytes) == 0 || len(originBytes) > 1048576 || generation < 0 {
		return AcceptanceResult{}, false, ErrSourcePartyUnavailable
	}
	hash := responseOriginHash(originBytes)
	if !bytes.Equal(originDigest, hash[:]) {
		return AcceptanceResult{}, false, ErrSourcePartyUnavailable
	}
	var origins candidateOriginCut
	if json.Unmarshal(originBytes, &origins) != nil || origins.Version != 2 || origins.Stamp.Validate() != nil || origins.HistoryHash == ([32]byte{}) || len(origins.Origins) == 0 {
		return AcceptanceResult{}, false, ErrSourcePartyUnavailable
	}
	canonical, err := json.Marshal(origins)
	if err != nil || !bytes.Equal(canonical, originBytes) {
		return AcceptanceResult{}, false, ErrSourcePartyUnavailable
	}
	for _, origin := range origins.Origins {
		if origin.Validate() != nil || origin.SourceKind != "NATIVE_VOICE" {
			return AcceptanceResult{}, false, ErrSourcePartyUnavailable
		}
	}
	if outcome != "ACCEPTED" && outcome != "DECLINED" && outcome != "EXPIRED" && outcome != "ALREADY_RESPONDED" {
		return AcceptanceResult{}, false, ErrSourcePartyUnavailable
	}
	match, err := scanMatch(tx.QueryRow(ctx, responseMatchSQL, matchID))
	if err != nil {
		return AcceptanceResult{}, false, err
	}
	var p MatchProposal
	err = tx.QueryRow(ctx, `SELECT id,match_id,search_session_id,profile_id,party_id,response,created_at,updated_at FROM match_proposals WHERE id=$1`, proposalID).Scan(&p.ID, &p.MatchID, &p.SearchSessionID, &p.ProfileID, &p.PartyID, &p.Response, &p.CreatedAt, &p.UpdatedAt)
	if err != nil || p.MatchID != matchID || p.ProfileID != params.ProfileID || p.SearchSessionID != sessionID {
		return AcceptanceResult{}, false, ErrSourcePartyUnavailable
	}
	var currentProfile uuid.UUID
	var currentGeneration int64
	if tx.QueryRow(ctx, `SELECT profile_id,recovery_generation FROM search_sessions WHERE id=$1`, sessionID).Scan(&currentProfile, &currentGeneration) != nil || currentProfile != params.ProfileID || currentGeneration < generation {
		return AcceptanceResult{}, false, ErrSourcePartyUnavailable
	}
	return AcceptanceResult{Match: match, Proposal: p, Replayed: true, Expired: outcome == "EXPIRED" || outcome == "DECLINED"}, true, nil
}
