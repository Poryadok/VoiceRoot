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
	"voice/backend/pkg/partyauthority"
)

var ErrProvisionCandidateChanged = errors.New("persisted provisioning candidate changed")

type ProvisionCandidate struct {
	MatchID     uuid.UUID
	fingerprint [32]byte
	parties     []SourceParty
}

func (c ProvisionCandidate) Hash() [32]byte { return c.fingerprint }

// Accounts returns a defensive canonical set for the outer platform ban guard.
func (c ProvisionCandidate) Accounts() []uuid.UUID {
	if c.MatchID == uuid.Nil || c.fingerprint == [32]byte{} || len(c.parties) == 0 {
		return nil
	}
	unique := map[uuid.UUID]struct{}{}
	for _, p := range c.parties {
		_, hash, err := encodePrivateSourceAuthority(CreateSourcePartyParams{InitiatorAccountID: p.InitiatorAccountID, InitiatorProfileID: p.LeaderProfileID, InitiatorSessionEpoch: p.InitiatorSessionEpoch, SourceKind: p.SourceKind, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash})
		if err != nil || hash != p.MemberAuthorityHash {
			return nil
		}
		for _, member := range p.MemberAuthorities {
			unique[member.AccountID] = struct{}{}
		}
	}
	accounts := make([]uuid.UUID, 0, len(unique))
	for account := range unique {
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool { return bytes.Compare(accounts[i][:], accounts[j][:]) < 0 })
	return accounts
}
func (c ProvisionCandidate) Parties() []SourceParty {
	parties := append([]SourceParty(nil), c.parties...)
	for i := range parties {
		parties[i].MemberProfileIDs = append([]uuid.UUID(nil), parties[i].MemberProfileIDs...)
		parties[i].MemberAuthorities = append([]partyauthority.Member(nil), parties[i].MemberAuthorities...)
		if parties[i].SourceRoomID != nil {
			id := *parties[i].SourceRoomID
			parties[i].SourceRoomID = &id
		}
		if parties[i].DisbandedAt != nil {
			at := *parties[i].DisbandedAt
			parties[i].DisbandedAt = &at
		}
	}
	return parties
}
func (s *MatchStore) CaptureProvisionCandidate(ctx context.Context, id uuid.UUID) (ProvisionCandidate, error) {
	if s == nil || s.Pool == nil || id == uuid.Nil {
		return ProvisionCandidate{}, ErrSourcePartyUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ProvisionCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	defer rollbackPartySearch(tx)
	candidate, err := readProvisionCandidate(ctx, tx, id)
	if err != nil {
		return ProvisionCandidate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProvisionCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	return candidate, nil
}

// VerifyProvisionCandidateTx performs no locking, network IO or commit. The
// transition owner must acquire/check the outer all-account PlatformBanGuard,
// then hold match -> sorted complete parties -> sorted complete
// sessions -> proposals/operation locks, then write and COMMIT in this transaction.
// Full operation JSON includes lease bookkeeping: capture after lease acquisition;
// renewal requires fresh capture and external validation.
func VerifyProvisionCandidateTx(ctx context.Context, tx pgx.Tx, c ProvisionCandidate) error {
	if tx == nil || c.MatchID == uuid.Nil || c.fingerprint == [32]byte{} {
		return ErrProvisionCandidateChanged
	}
	current, err := readProvisionCandidate(ctx, tx, c.MatchID)
	if err != nil {
		return errors.Join(ErrProvisionCandidateChanged, err)
	}
	if current.fingerprint != c.fingerprint {
		return ErrProvisionCandidateChanged
	}
	return nil
}

const provisionCutSQL = `WITH match_sessions AS (SELECT * FROM search_sessions WHERE match_id=$1),
party_ids AS (SELECT party_id id FROM match_sessions WHERE party_id IS NOT NULL UNION SELECT party_id FROM match_proposals WHERE match_id=$1 AND party_id IS NOT NULL),
all_sessions AS (SELECT * FROM search_sessions WHERE match_id=$1 OR party_id IN(SELECT id FROM party_ids))
SELECT jsonb_build_object(
'match',(SELECT to_jsonb(m) FROM matches m WHERE id=$1),
'proposals',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM match_proposals p WHERE match_id=$1),'[]'::jsonb),
'sessions',COALESCE((SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM all_sessions s),'[]'::jsonb),
'parties',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM parties p WHERE id IN(SELECT id FROM party_ids)),'[]'::jsonb),
'operation',(SELECT to_jsonb(o) FROM matchmaking_match_squad_operations o WHERE match_id=$1))::text,clock_timestamp()`

type provisionSession struct {
	ID        uuid.UUID  `json:"id"`
	ProfileID uuid.UUID  `json:"profile_id"`
	PartyID   *uuid.UUID `json:"party_id"`
	MatchID   *uuid.UUID `json:"match_id"`
	Status    string     `json:"status"`
	TimeoutAt *time.Time `json:"timeout_at"`
}
type provisionProposal struct {
	ID        uuid.UUID  `json:"id"`
	ProfileID uuid.UUID  `json:"profile_id"`
	SessionID uuid.UUID  `json:"search_session_id"`
	PartyID   *uuid.UUID `json:"party_id"`
	Response  string     `json:"response"`
}

func readProvisionCandidate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (ProvisionCandidate, error) {
	var raw string
	var now time.Time
	if err := tx.QueryRow(ctx, provisionCutSQL, id).Scan(&raw, &now); err != nil {
		return ProvisionCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	var view struct {
		Match *struct {
			ID           uuid.UUID          `json:"id"`
			GameID       uuid.UUID          `json:"game_id"`
			Mode         string             `json:"mode"`
			Status       string             `json:"status"`
			Participants []MatchParticipant `json:"participants"`
		} `json:"match"`
		Proposals []provisionProposal `json:"proposals"`
		Sessions  []provisionSession  `json:"sessions"`
		Parties   []struct {
			ID uuid.UUID `json:"id"`
		} `json:"parties"`
	}
	if json.Unmarshal([]byte(raw), &view) != nil || view.Match == nil || view.Match.ID != id || view.Match.Status != MatchStatusPendingAccept || len(view.Proposals) == 0 || len(view.Proposals) != len(view.Sessions) || len(view.Sessions) != len(view.Match.Participants) {
		return ProvisionCandidate{}, ErrSourcePartyUnavailable
	}
	sessions := map[uuid.UUID]provisionSession{}
	profiles := map[uuid.UUID]bool{}
	partyMembers := map[uuid.UUID]map[uuid.UUID]bool{}
	for _, s := range view.Sessions {
		if s.ID == uuid.Nil || s.ProfileID == uuid.Nil || s.PartyID == nil || *s.PartyID == uuid.Nil || s.MatchID == nil || *s.MatchID != id || s.Status != SessionStatusPendingAccept || profiles[s.ProfileID] || s.TimeoutAt == nil || !s.TimeoutAt.After(now) {
			return ProvisionCandidate{}, ErrSourcePartyUnavailable
		}
		sessions[s.ID] = s
		profiles[s.ProfileID] = true
		if partyMembers[*s.PartyID] == nil {
			partyMembers[*s.PartyID] = map[uuid.UUID]bool{}
		}
		partyMembers[*s.PartyID][s.ProfileID] = true
	}
	seen := map[uuid.UUID]bool{}
	for _, p := range view.Proposals {
		s, ok := sessions[p.SessionID]
		if !ok || p.ID == uuid.Nil || seen[p.SessionID] || p.Response != ProposalResponseAccepted || p.ProfileID != s.ProfileID || !samePartyUUID(p.PartyID, s.PartyID) {
			return ProvisionCandidate{}, ErrSourcePartyUnavailable
		}
		seen[p.SessionID] = true
	}
	seen = map[uuid.UUID]bool{}
	for _, p := range view.Match.Participants {
		sid, err := uuid.Parse(p.SessionID)
		s, ok := sessions[sid]
		if err != nil || p.SessionID != sid.String() || !ok || seen[sid] || p.ProfileID != s.ProfileID.String() {
			return ProvisionCandidate{}, ErrSourcePartyUnavailable
		}
		seen[sid] = true
	}
	if len(view.Parties) != len(partyMembers) {
		return ProvisionCandidate{}, ErrSourcePartyUnavailable
	}
	cut := ProvisionCandidate{MatchID: id, fingerprint: sha256.Sum256([]byte(raw))}
	for _, row := range view.Parties {
		p, err := scanSourceParty(tx.QueryRow(ctx, `SELECT `+sourcePartyCols+` FROM parties WHERE id=$1 AND search_operation_id IS NOT NULL`, row.ID))
		if err != nil || p.DisbandedAt != nil || p.GameID != view.Match.GameID || p.Mode != view.Match.Mode {
			return ProvisionCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		var coherent bool
		if err := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM search_sessions WHERE party_id=$1 AND (game_id<>$2 OR mode<>$3 OR criteria<>$4::jsonb))`, p.ID, p.GameID, p.Mode, p.Criteria).Scan(&coherent); err != nil || !coherent {
			return ProvisionCandidate{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		members := partyMembers[p.ID]
		if len(members) != len(p.MemberProfileIDs) {
			return ProvisionCandidate{}, ErrSourcePartyUnavailable
		}
		for _, id := range p.MemberProfileIDs {
			if !members[id] {
				return ProvisionCandidate{}, ErrSourcePartyUnavailable
			}
		}
		cut.parties = append(cut.parties, p)
	}
	if len(cut.parties) == 0 {
		return ProvisionCandidate{}, ErrSourcePartyUnavailable
	}
	return cut, nil
}
