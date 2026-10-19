package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// lockProposalParties precedes every session lock. Captured rosters are immutable;
// a partial queue projection must not turn a whole party into separate matches.
func lockProposalParties(ctx context.Context, tx pgx.Tx, sessions []ProposalSession) ([]uuid.UUID, error) {
	requested := make(map[uuid.UUID]map[uuid.UUID]bool)
	seenProfiles := make(map[uuid.UUID]bool, len(sessions))
	for _, session := range sessions {
		if session.SessionID == uuid.Nil || session.ProfileID == uuid.Nil || seenProfiles[session.ProfileID] {
			return nil, ErrSessionNotSearchable
		}
		seenProfiles[session.ProfileID] = true
		if session.PartyID == nil {
			continue
		}
		if *session.PartyID == uuid.Nil {
			return nil, ErrSessionNotSearchable
		}
		if requested[*session.PartyID] == nil {
			requested[*session.PartyID] = make(map[uuid.UUID]bool)
		}
		requested[*session.PartyID][session.ProfileID] = true
	}
	ids := make([]uuid.UUID, 0, len(requested))
	for id := range requested {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return ids, nil
	}
	rows, err := tx.Query(ctx, `SELECT id,member_profile_ids,disbanded_at FROM parties
 WHERE id=ANY($1::uuid[]) ORDER BY id FOR UPDATE`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id uuid.UUID
		var raw []byte
		var disbanded *time.Time
		if err := rows.Scan(&id, &raw, &disbanded); err != nil {
			return nil, err
		}
		var members []uuid.UUID
		if disbanded != nil || json.Unmarshal(raw, &members) != nil || len(members) == 0 || len(members) != len(requested[id]) {
			return nil, ErrSessionNotSearchable
		}
		seen := make(map[uuid.UUID]bool, len(members))
		for _, member := range members {
			if member == uuid.Nil || seen[member] || !requested[id][member] {
				return nil, ErrSessionNotSearchable
			}
			seen[member] = true
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if count != len(ids) {
		return nil, ErrSessionNotSearchable
	}
	return ids, nil
}

func proposalSessionScopeMatches(session SearchSession, p CreateProposalParams, now time.Time) bool {
	if session.GameID != p.GameID || session.Mode != p.Mode || session.MatchID != nil || session.TimeoutAt == nil || !session.TimeoutAt.After(now) {
		return false
	}
	var scope struct {
		Region string `json:"region"`
	}
	return json.Unmarshal([]byte(session.Criteria), &scope) == nil && scope.Region == p.Region && scope.Region != ""
}
