package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// loadEffectiveHead never inserts or reconstructs a missing head. Its caller
// owns the native sorted Space fence and must use READ COMMITTED. Source facts
// are separately validated in the same transaction before this private helper.
func loadEffectiveHead(ctx context.Context, tx pgx.Tx, space, profile uuid.UUID) (effectiveHead, uuid.UUID, error) {
	var raw []byte
	var cutover uuid.UUID
	err := tx.QueryRow(ctx, `SELECT jsonb_build_object(
 'Generation',generation,'RetiredThrough',retired_through,'Revision',ledger_revision,
 'EpisodeID',COALESCE(episode_id,'00000000-0000-0000-0000-000000000000'::uuid),
 'Active',state='ACTIVE','Known',boundary_state='KNOWN',
 'Since',COALESCE(history_visible_since,'0001-01-01 00:00:00+00'::timestamptz),
 'ObservedAt',observed_at,'Sources',jsonb_build_object('Ordinary',COALESCE(ordinary_witness,''),
 'Legacy',ordinary_legacy,'Community',COALESCE(community_witness,''),
 'CommunityUntil',COALESCE(community_until,'0001-01-01 00:00:00+00'::timestamptz))),cutover_id
 FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 FOR UPDATE`, space, profile).Scan(&raw, &cutover)
	var h effectiveHead
	if err != nil || json.Unmarshal(raw, &h) != nil || !h.valid() || cutover == uuid.Nil {
		return effectiveHead{}, uuid.Nil, ErrEffectiveEpisodeUnavailable
	}
	return h, cutover, nil
}

// saveEffectiveHead is only a CAS persistence primitive. It neither admits a
// caller-supplied source nor proves native writer coverage/cutover by itself.
// No source transaction may commit until its final native revision/time fence
// succeeds. The protected report route is not activated by this primitive.
func saveEffectiveHead(ctx context.Context, tx pgx.Tx, space, profile, cutover uuid.UUID, oldRevision int64, h effectiveHead) error {
	if tx == nil || space == uuid.Nil || profile == uuid.Nil || cutover == uuid.Nil || !h.valid() || oldRevision < 0 || h.Revision != oldRevision+1 {
		return ErrEffectiveEpisodeUnavailable
	}
	var isolation string
	if err := tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&isolation); err != nil || isolation != "read committed" {
		return ErrEffectiveEpisodeUnavailable
	}
	state := "NONE"
	if h.Active {
		state = "ACTIVE"
	} else if h.Generation > 0 {
		state = "RETIRED"
	}
	boundary := "UNKNOWN"
	if h.Known {
		boundary = "KNOWN"
	}
	var episode any
	if h.EpisodeID != uuid.Nil {
		episode = h.EpisodeID
	}
	var since any
	if h.Known {
		since = h.Since
	}
	var ordinary, community, communityUntil, covered any
	if h.Sources.Ordinary != "" {
		ordinary = h.Sources.Ordinary
		covered = pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}
	}
	if h.Sources.Community != "" {
		community = h.Sources.Community
		communityUntil = h.Sources.CommunityUntil
		if covered == nil {
			covered = h.Sources.CommunityUntil
		}
	}
	args := []any{space, profile, h.Generation, h.RetiredThrough, h.Revision, episode, state, boundary, since, covered, h.ObservedAt, cutover, ordinary, h.Sources.Legacy, community, communityUntil}
	var err error
	var count int64
	if oldRevision == 0 {
		tag, e := tx.Exec(ctx, `INSERT INTO space_effective_member_heads(space_id,profile_id,generation,retired_through,ledger_revision,episode_id,state,boundary_state,history_visible_since,covered_until,observed_at,cutover_id,ordinary_witness,ordinary_legacy,community_witness,community_until)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) ON CONFLICT DO NOTHING`, args...)
		err = e
		count = tag.RowsAffected()
	} else {
		args = append(args, oldRevision)
		tag, e := tx.Exec(ctx, `UPDATE space_effective_member_heads SET generation=$3,retired_through=$4,ledger_revision=$5,episode_id=$6,state=$7,boundary_state=$8,history_visible_since=$9,covered_until=$10,observed_at=$11,ordinary_witness=$13,ordinary_legacy=$14,community_witness=$15,community_until=$16 WHERE space_id=$1 AND profile_id=$2 AND cutover_id=$12 AND ledger_revision=$17`, args...)
		err = e
		count = tag.RowsAffected()
	}
	if err != nil || count != 1 {
		return ErrEffectiveEpisodeUnavailable
	}
	// Final time is sampled after the write (which may have waited on a row).
	var finished time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished); err != nil || finished.Before(h.ObservedAt) || !h.Sources.equal(h.Sources.at(finished)) || (!h.bridgeUntil.IsZero() && !finished.Before(h.bridgeUntil)) {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}
