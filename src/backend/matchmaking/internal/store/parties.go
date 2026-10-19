package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/matchmaking/internal/mmadmission"
)

var ErrPartyNotFound = errors.New("party not found")

// Party is a row in parties (group MM / LFP Social Discovery).
type Party struct {
	ID               uuid.UUID
	LeaderProfileID  uuid.UUID
	MemberProfileIDs []uuid.UUID
	GameID           uuid.UUID
	Mode             string
	Criteria         string
	CreatedAt        time.Time
	DisbandedAt      *time.Time
}

// PartyStore persists matchmaking parties.
type PartyStore struct {
	Pool            *pgxpool.Pool
	NativeAdmission *mmadmission.Binding
}

// CreatePartyParams inserts a new party.
type CreatePartyParams struct {
	LeaderProfileID  uuid.UUID
	MemberProfileIDs []uuid.UUID
	GameID           uuid.UUID
	Mode             string
	Criteria         string
}

// Create inserts a pre-cut legacy party. NativeAdmission does not qualify this
// profile-only path; native callers must use CreateSourceParty instead.
func (s *PartyStore) Create(ctx context.Context, p CreatePartyParams) (Party, error) {
	if s == nil || s.Pool == nil {
		return Party{}, errors.New("party store unavailable")
	}
	members := p.MemberProfileIDs
	if len(members) == 0 {
		members = []uuid.UUID{p.LeaderProfileID}
	}
	ids := make([]string, 0, len(members))
	seen := map[uuid.UUID]struct{}{}
	for _, id := range members {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id.String())
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return Party{}, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Party{}, err
	}
	defer tx.Rollback(ctx)
	if err := mmadmission.Acquire(ctx, tx, nil); err != nil {
		return Party{}, err
	}
	id := uuid.New()
	row := tx.QueryRow(ctx, `
		INSERT INTO parties (id, leader_profile_id, member_profile_ids, game_id, mode, criteria)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6::jsonb)
		RETURNING id, leader_profile_id, member_profile_ids, game_id, mode, criteria::text, created_at, disbanded_at
	`, id, p.LeaderProfileID, string(raw), p.GameID, p.Mode, p.Criteria)
	party, err := scanParty(row)
	if err != nil {
		return Party{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Party{}, err
	}
	return party, nil
}

// Get loads a party by id.
func (s *PartyStore) Get(ctx context.Context, id uuid.UUID) (Party, error) {
	if s == nil || s.Pool == nil {
		return Party{}, errors.New("party store unavailable")
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT id, leader_profile_id, member_profile_ids, game_id, mode, criteria::text, created_at, disbanded_at
		FROM parties WHERE id = $1
	`, id)
	party, err := scanParty(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Party{}, ErrPartyNotFound
	}
	return party, err
}

func scanParty(row pgx.Row) (Party, error) {
	var (
		p       Party
		members []byte
	)
	err := row.Scan(
		&p.ID, &p.LeaderProfileID, &members, &p.GameID, &p.Mode, &p.Criteria, &p.CreatedAt, &p.DisbandedAt,
	)
	if err != nil {
		return Party{}, err
	}
	var rawIDs []string
	if err := json.Unmarshal(members, &rawIDs); err != nil {
		return Party{}, err
	}
	p.MemberProfileIDs = make([]uuid.UUID, 0, len(rawIDs))
	for _, s := range rawIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			continue
		}
		p.MemberProfileIDs = append(p.MemberProfileIDs, id)
	}
	return p, nil
}
