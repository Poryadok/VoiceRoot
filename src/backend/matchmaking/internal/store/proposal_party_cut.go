package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sort"
	"time"
)

var ErrProposalCandidateChanged = errors.New("persisted proposal candidate changed")

// ProposalCandidate is the immutable local cut for one immediate proposal
// reservation. It contains no match ID because reservation has not occurred.
type ProposalCandidate struct {
	fingerprint [32]byte
	params      CreateProposalParams
	parties     []SourceParty
}

func (c ProposalCandidate) Hash() [32]byte { return c.fingerprint }
func (c ProposalCandidate) Accounts() []uuid.UUID {
	if c.fingerprint == [32]byte{} || len(c.parties) == 0 {
		return nil
	}
	accounts := map[uuid.UUID]bool{}
	for _, p := range c.parties {
		_, hash, err := encodePrivateSourceAuthority(CreateSourcePartyParams{InitiatorAccountID: p.InitiatorAccountID, InitiatorProfileID: p.LeaderProfileID, InitiatorSessionEpoch: p.InitiatorSessionEpoch, SourceKind: p.SourceKind, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash})
		if err != nil || hash != p.MemberAuthorityHash {
			return nil
		}
		for _, member := range p.MemberAuthorities {
			accounts[member.AccountID] = true
		}
	}
	ids := make([]uuid.UUID, 0, len(accounts))
	for id := range accounts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	return ids
}
func (c ProposalCandidate) Parties() []SourceParty {
	return (ProvisionCandidate{parties: c.parties}).Parties()
}

func (s *MatchStore) CaptureProposalCandidate(ctx context.Context, p CreateProposalParams) (ProposalCandidate, error) {
	if s == nil || s.Pool == nil {
		return ProposalCandidate{}, ErrSourcePartyUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ProposalCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	defer rollbackPartySearch(tx)
	c, err := readProposalCandidate(ctx, tx, p)
	if err != nil {
		return ProposalCandidate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProposalCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	return c, nil
}

// VerifyProposalCandidateTx performs no locking, network IO or commit. The
// reservation owner acquires account ban locks, then sorted complete parties
// and sessions, before this fresh statement and the forward transition.
func VerifyProposalCandidateTx(ctx context.Context, tx pgx.Tx, c ProposalCandidate) error {
	if tx == nil || c.fingerprint == [32]byte{} {
		return ErrProposalCandidateChanged
	}
	current, err := readProposalCandidate(ctx, tx, c.params)
	if err != nil || current.fingerprint != c.fingerprint {
		return errors.Join(ErrProposalCandidateChanged, err)
	}
	return nil
}

const proposalCutSQL = `WITH selected AS (SELECT * FROM search_sessions WHERE id=ANY($1::uuid[])),
party_ids AS (SELECT DISTINCT party_id id FROM selected WHERE party_id IS NOT NULL)
SELECT jsonb_build_object(
'sessions',COALESCE((SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM search_sessions s WHERE id=ANY($1::uuid[]) OR party_id IN(SELECT id FROM party_ids)),'[]'::jsonb),
'parties',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM parties p WHERE id IN(SELECT id FROM party_ids)),'[]'::jsonb))::text,clock_timestamp()`

func readProposalCandidate(ctx context.Context, tx pgx.Tx, p CreateProposalParams) (ProposalCandidate, error) {
	if p.GameID == uuid.Nil || p.Mode == "" || p.Region == "" || len(p.Sessions) == 0 {
		return ProposalCandidate{}, ErrSourcePartyUnavailable
	}
	p.Sessions = append([]ProposalSession(nil), p.Sessions...)
	requested := map[uuid.UUID]ProposalSession{}
	profiles := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(p.Sessions))
	partyMembers := map[uuid.UUID]map[uuid.UUID]bool{}
	for i, session := range p.Sessions {
		if session.SessionID == uuid.Nil || session.ProfileID == uuid.Nil || profiles[session.ProfileID] || requested[session.SessionID].SessionID != uuid.Nil || session.PartyID == nil || *session.PartyID == uuid.Nil {
			return ProposalCandidate{}, ErrSourcePartyUnavailable
		}
		id := *session.PartyID
		p.Sessions[i].PartyID = &id
		requested[session.SessionID] = p.Sessions[i]
		profiles[session.ProfileID] = true
		ids = append(ids, session.SessionID)
		if partyMembers[id] == nil {
			partyMembers[id] = map[uuid.UUID]bool{}
		}
		partyMembers[id][session.ProfileID] = true
	}
	sort.Slice(p.Sessions, func(i, j int) bool { return bytes.Compare(p.Sessions[i].SessionID[:], p.Sessions[j].SessionID[:]) < 0 })
	var raw string
	var now time.Time
	if err := tx.QueryRow(ctx, proposalCutSQL, ids).Scan(&raw, &now); err != nil {
		return ProposalCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	var view struct {
		Sessions []struct {
			provisionSession
			GameID   uuid.UUID       `json:"game_id"`
			Mode     string          `json:"mode"`
			Criteria json.RawMessage `json:"criteria"`
			SpaceID  *uuid.UUID      `json:"space_id"`
		} `json:"sessions"`
		Parties []struct {
			ID uuid.UUID `json:"id"`
		} `json:"parties"`
	}
	if json.Unmarshal([]byte(raw), &view) != nil || len(view.Parties) != len(partyMembers) {
		return ProposalCandidate{}, ErrSourcePartyUnavailable
	}
	seen := map[uuid.UUID]bool{}
	var scope *uuid.UUID
	for _, session := range view.Sessions {
		selected, exists := requested[session.ID]
		if !exists {
			if session.Status == SessionStatusSearching || session.Status == SessionStatusPendingAccept || session.Status == "waiting" {
				return ProposalCandidate{}, ErrSourcePartyUnavailable
			}
			continue
		}
		if seen[session.ID] || session.ProfileID != selected.ProfileID || !samePartyUUID(session.PartyID, selected.PartyID) || session.Status != SessionStatusSearching || !proposalSessionScopeMatches(SearchSession{GameID: session.GameID, Mode: session.Mode, MatchID: session.MatchID, TimeoutAt: session.TimeoutAt, Criteria: string(session.Criteria)}, p, now) {
			return ProposalCandidate{}, ErrSourcePartyUnavailable
		}
		if len(seen) == 0 {
			scope = session.SpaceID
		} else if !samePartyUUID(scope, session.SpaceID) {
			return ProposalCandidate{}, ErrSourcePartyUnavailable
		}
		seen[session.ID] = true
	}
	if len(seen) != len(requested) {
		return ProposalCandidate{}, ErrSourcePartyUnavailable
	}
	c := ProposalCandidate{params: p}
	for _, row := range view.Parties {
		party, err := scanSourceParty(tx.QueryRow(ctx, `SELECT `+sourcePartyCols+` FROM parties WHERE id=$1 AND search_operation_id IS NOT NULL`, row.ID))
		if err != nil || party.DisbandedAt != nil || party.GameID != p.GameID || party.Mode != p.Mode || len(party.MemberProfileIDs) != len(partyMembers[row.ID]) {
			return ProposalCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		for _, profile := range party.MemberProfileIDs {
			if !partyMembers[row.ID][profile] {
				return ProposalCandidate{}, ErrSourcePartyUnavailable
			}
		}
		var coherent bool
		if err := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM search_sessions WHERE party_id=$1 AND status IN('searching','waiting','pending_accept') AND (game_id<>$2 OR mode<>$3 OR criteria<>$4::jsonb))`, party.ID, party.GameID, party.Mode, party.Criteria).Scan(&coherent); err != nil || !coherent {
			return ProposalCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		c.parties = append(c.parties, party)
	}
	body, err := json.Marshal(struct {
		Params CreateProposalParams
		Cut    string
	}{p, raw})
	if err != nil {
		return ProposalCandidate{}, ErrSourcePartyUnavailable
	}
	c.fingerprint = sha256.Sum256(append([]byte("voice.mm.proposal-local-cut.v1\x00"), body...))
	if len(c.Accounts()) == 0 {
		return ProposalCandidate{}, ErrSourcePartyUnavailable
	}
	return c, nil
}
