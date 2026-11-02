package registry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidCommunityRoster = errors.New("invalid community roster snapshot")
	ErrStaleCommunityRoster   = errors.New("stale community roster revision")
)

type CommunityRosterMember struct {
	BindingID                uuid.UUID `json:"binding_id"`
	CharacterKey             string    `json:"character_key"`
	SourceMembershipRevision int64     `json:"source_membership_revision"`
	SourceRank               string    `json:"source_rank"`
	GrantReasons             []string  `json:"grant_reasons"`
}

type CommunityRosterPage struct {
	Index   int32                   `json:"index"`
	Count   int32                   `json:"count"`
	SHA256  []byte                  `json:"-"`
	Members []CommunityRosterMember `json:"members"`
}

type CommunityRosterSnapshotInput struct {
	OperationID     uuid.UUID             `json:"operation_id"`
	CorporationKey  string                `json:"corporation_key"`
	SourceRevision  int64                 `json:"source_revision"`
	OwnerGeneration int64                 `json:"owner_generation"`
	SnapshotSHA256  []byte                `json:"-"`
	Pages           []CommunityRosterPage `json:"pages"`
}

type CommunityRosterResult struct {
	OperationID     uuid.UUID  `json:"operation_id"`
	ApplicationID   uuid.UUID  `json:"application_id"`
	EnvironmentID   uuid.UUID  `json:"environment_id"`
	CorporationKey  string     `json:"corporation_key"`
	SourceRevision  int64      `json:"source_revision"`
	OwnerGeneration int64      `json:"owner_generation"`
	SnapshotSHA256  []byte     `json:"snapshot_sha256"`
	Outcome         string     `json:"outcome"`
	MemberCount     int        `json:"member_count"`
	LeaseExpiresAt  *time.Time `json:"lease_expires_at,omitempty"`
	Replayed        bool       `json:"replayed"`
}

type CommunityRosterGrant struct {
	BindingID    uuid.UUID `json:"binding_id"`
	CharacterKey string    `json:"character_key"`
	SourceRank   string    `json:"source_rank"`
	GrantReasons []string  `json:"grant_reasons"`
}

type CommunityRosterSnapshotView struct {
	ApplicationID   uuid.UUID              `json:"application_id"`
	EnvironmentID   uuid.UUID              `json:"environment_id"`
	CorporationKey  string                 `json:"corporation_key"`
	SourceRevision  int64                  `json:"source_revision"`
	OwnerGeneration int64                  `json:"owner_generation"`
	SnapshotSHA256  []byte                 `json:"snapshot_sha256"`
	LeaseExpiresAt  time.Time              `json:"lease_expires_at"`
	Members         []CommunityRosterGrant `json:"members"`
}

func (v CommunityRosterSnapshotView) SnapshotSHA256String() string {
	return hex.EncodeToString(v.SnapshotSHA256)
}

func (o *SessionOrchestrator) ApplyCommunityRosterSnapshot(ctx context.Context, p SessionPrincipal, in CommunityRosterSnapshotInput) (CommunityRosterResult, error) {
	if o == nil || o.Store == nil {
		return CommunityRosterResult{}, ErrRegistryUnavailable
	}
	result, err := o.Store.ApplyCommunityRosterSnapshot(ctx, p, in)
	if err != nil {
		return CommunityRosterResult{}, err
	}
	if o.CommunityRosterProjector == nil {
		return CommunityRosterResult{}, ErrRegistryUnavailable
	}
	projection, err := o.Store.BuildCommunityRosterProjection(ctx, p, in)
	if err != nil {
		return CommunityRosterResult{}, err
	}
	receipt, err := o.CommunityRosterProjector.ProjectCommunityRoster(ctx, projection)
	if err != nil {
		return CommunityRosterResult{}, err
	}
	if err = o.Store.RecordCommunityRosterSpaceReceipt(ctx, projection, receipt); err != nil {
		return CommunityRosterResult{}, err
	}
	return result, nil
}

func (o *SessionOrchestrator) GetCommunityRosterSnapshot(ctx context.Context, p SessionPrincipal, corporationKey string) (CommunityRosterSnapshotView, error) {
	if o == nil || o.Store == nil {
		return CommunityRosterSnapshotView{}, ErrRegistryUnavailable
	}
	return o.Store.GetCommunityRosterSnapshot(ctx, p, corporationKey)
}

// GetCommunityRosterSnapshot exposes only app/environment-scoped binding
// references and source grants; global Voice profile/account IDs are never
// serialized. It denies at the exact source lease boundary.
func (s *Store) GetCommunityRosterSnapshot(ctx context.Context, p SessionPrincipal, corporationKey string) (CommunityRosterSnapshotView, error) {
	if s == nil || s.Pool == nil || p.ApplicationID == uuid.Nil || p.EnvironmentID == uuid.Nil || !validCommunityKey(corporationKey) {
		return CommunityRosterSnapshotView{}, ErrInvalidCommunityRoster
	}
	var view CommunityRosterSnapshotView
	err := s.Pool.QueryRow(ctx, `SELECT application_id,environment_id,corporation_key,roster_source_revision,
		owner_generation,roster_sha256,roster_lease_expires_at
		FROM gis_community_owner_authority WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3
		AND status='active' AND roster_source_revision>0 AND roster_lease_expires_at>clock_timestamp()`,
		p.ApplicationID, p.EnvironmentID, corporationKey).Scan(&view.ApplicationID, &view.EnvironmentID,
		&view.CorporationKey, &view.SourceRevision, &view.OwnerGeneration, &view.SnapshotSHA256, &view.LeaseExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CommunityRosterSnapshotView{}, ErrStaleCommunityRoster
	}
	if err != nil {
		return CommunityRosterSnapshotView{}, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT g.binding_id,g.character_key,g.source_rank,g.source_reason
		FROM gis_community_grants g JOIN player_bindings b
		ON b.binding_id=g.binding_id AND b.application_id=g.application_id AND b.environment_id=g.environment_id
		JOIN gis_community_owner_authority a ON a.application_id=g.application_id AND a.environment_id=g.environment_id
		AND a.corporation_key=g.corporation_key AND a.owner_generation=g.owner_generation AND a.status='active'
		WHERE g.application_id=$1 AND g.environment_id=$2 AND g.corporation_key=$3
		AND g.owner_generation=$4 AND g.revoked_at IS NULL AND g.lease_expires_at>clock_timestamp()
		AND b.status='active' AND b.permitted_character_context ? g.character_key
		ORDER BY g.binding_id,g.character_key,g.source_reason`,
		p.ApplicationID, p.EnvironmentID, corporationKey, view.OwnerGeneration)
	if err != nil {
		return CommunityRosterSnapshotView{}, err
	}
	defer rows.Close()
	index := make(map[string]int)
	for rows.Next() {
		var grant CommunityRosterGrant
		var reason string
		if err := rows.Scan(&grant.BindingID, &grant.CharacterKey, &grant.SourceRank, &reason); err != nil {
			return CommunityRosterSnapshotView{}, err
		}
		key := grant.BindingID.String() + "\x00" + grant.CharacterKey
		position, exists := index[key]
		if !exists {
			position = len(view.Members)
			index[key] = position
			grant.GrantReasons = []string{reason}
			view.Members = append(view.Members, grant)
		} else {
			view.Members[position].GrantReasons = append(view.Members[position].GrantReasons, reason)
		}
	}
	if err := rows.Err(); err != nil {
		return CommunityRosterSnapshotView{}, err
	}
	if view.Members == nil {
		view.Members = []CommunityRosterGrant{}
	}
	return view, nil
}

func (s *Store) RevokeExpiredCommunityRosterGrants(ctx context.Context) (int64, error) {
	if s == nil || s.Pool == nil {
		return 0, ErrRegistryUnavailable
	}
	tag, err := s.Pool.Exec(ctx, `WITH due AS (
		SELECT ctid FROM gis_community_grants WHERE revoked_at IS NULL AND lease_expires_at<=clock_timestamp()
		ORDER BY lease_expires_at,application_id,environment_id LIMIT 500 FOR UPDATE SKIP LOCKED
	) UPDATE gis_community_grants grant_row
	SET revoked_at=clock_timestamp(),revocation_reason='source_expired',updated_at=clock_timestamp()
	FROM due WHERE grant_row.ctid=due.ctid`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func RunCommunityRosterExpiryWorker(ctx context.Context, store *Store, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			workCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			count, err := store.RevokeExpiredCommunityRosterGrants(workCtx)
			cancel()
			if err != nil {
				slog.Warn("GIS community roster expiry failed", "error", err)
			} else if count > 0 {
				slog.Info("GIS community grants expired", "count", count)
			}
		}
	}
}

// ApplyCommunityRosterSnapshot activates a fully verified snapshot atomically
// against the current GIS Owner generation. It writes only GIS-owned intent;
// Role/Chat consumers apply the durable desired state through owner APIs.
func (s *Store) ApplyCommunityRosterSnapshot(ctx context.Context, p SessionPrincipal, in CommunityRosterSnapshotInput) (CommunityRosterResult, error) {
	digest, err := validateCommunityRosterSnapshot(in)
	if err != nil || s == nil || s.Pool == nil || p.ApplicationID == uuid.Nil || p.EnvironmentID == uuid.Nil {
		return CommunityRosterResult{}, ErrInvalidCommunityRoster
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return CommunityRosterResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var savedDigest []byte
	var savedApp, savedEnv uuid.UUID
	var savedCorp string
	var savedRevision, savedGeneration int64
	var savedOutcome string
	var savedMembers int
	var savedLease *time.Time
	err = tx.QueryRow(ctx, `SELECT application_id,environment_id,corporation_key,source_revision,
		owner_generation,snapshot_sha256,outcome,member_count,lease_expires_at
		FROM gis_community_roster_snapshots WHERE operation_id=$1`, in.OperationID).
		Scan(&savedApp, &savedEnv, &savedCorp, &savedRevision, &savedGeneration,
			&savedDigest, &savedOutcome, &savedMembers, &savedLease)
	if err == nil {
		if savedApp != p.ApplicationID || savedEnv != p.EnvironmentID || savedCorp != in.CorporationKey ||
			savedRevision != in.SourceRevision || savedGeneration != in.OwnerGeneration || !bytes.Equal(savedDigest, digest) {
			return CommunityRosterResult{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return CommunityRosterResult{}, err
		}
		return CommunityRosterResult{OperationID: in.OperationID, ApplicationID: savedApp,
			EnvironmentID: savedEnv, CorporationKey: savedCorp, SourceRevision: savedRevision,
			OwnerGeneration: savedGeneration, SnapshotSHA256: savedDigest, Outcome: savedOutcome,
			MemberCount: savedMembers, LeaseExpiresAt: savedLease, Replayed: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CommunityRosterResult{}, err
	}
	var generation, currentRevision int64
	var ownerStatus string
	var currentHash []byte
	err = tx.QueryRow(ctx, `SELECT owner_generation,status,roster_source_revision,roster_sha256
		FROM gis_community_owner_authority WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 FOR UPDATE`,
		p.ApplicationID, p.EnvironmentID, in.CorporationKey).
		Scan(&generation, &ownerStatus, &currentRevision, &currentHash)
	if errors.Is(err, pgx.ErrNoRows) || ownerStatus != "active" || generation != in.OwnerGeneration {
		return CommunityRosterResult{}, ErrInvalidCommunityRoster
	}
	if err != nil {
		return CommunityRosterResult{}, err
	}
	if in.SourceRevision < currentRevision {
		return CommunityRosterResult{}, ErrStaleCommunityRoster
	}
	if in.SourceRevision == currentRevision {
		if bytes.Equal(currentHash, digest) {
			var priorOperation uuid.UUID
			var priorMembers int
			var priorLease time.Time
			if err = tx.QueryRow(ctx, `SELECT operation_id,member_count,lease_expires_at FROM gis_community_roster_snapshots
				WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND owner_generation=$4 AND source_revision=$5 AND snapshot_sha256=$6 AND outcome='accepted'`,
				p.ApplicationID, p.EnvironmentID, in.CorporationKey, generation, in.SourceRevision, digest).
				Scan(&priorOperation, &priorMembers, &priorLease); err != nil {
				return CommunityRosterResult{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return CommunityRosterResult{}, err
			}
			return CommunityRosterResult{OperationID: priorOperation, ApplicationID: p.ApplicationID,
				EnvironmentID: p.EnvironmentID, CorporationKey: in.CorporationKey,
				SourceRevision: in.SourceRevision, OwnerGeneration: in.OwnerGeneration,
				SnapshotSHA256: append([]byte(nil), digest...), Outcome: "accepted",
				MemberCount: priorMembers, LeaseExpiresAt: &priorLease, Replayed: true}, nil
		}
		return CommunityRosterResult{}, ErrIdempotencyConflict
	}
	for _, page := range in.Pages {
		for _, member := range page.Members {
			var active bool
			if err = tx.QueryRow(ctx, `SELECT status='active' AND permitted_character_context ? $4
				FROM player_bindings WHERE binding_id=$1 AND application_id=$2 AND environment_id=$3 FOR SHARE`,
				member.BindingID, p.ApplicationID, p.EnvironmentID, member.CharacterKey).Scan(&active); err != nil || !active {
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return CommunityRosterResult{}, err
				}
				return CommunityRosterResult{}, ErrInvalidCommunityRoster
			}
		}
	}
	var memberCount int
	for _, page := range in.Pages {
		memberCount += len(page.Members)
	}
	var lease time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()+interval '60 seconds'`).Scan(&lease); err != nil {
		return CommunityRosterResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gis_community_roster_snapshots
		(operation_id,application_id,environment_id,corporation_key,source_revision,owner_generation,
		 snapshot_sha256,page_count,member_count,outcome,lease_expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'accepted',$10)`, in.OperationID, p.ApplicationID,
		p.EnvironmentID, in.CorporationKey, in.SourceRevision, in.OwnerGeneration, digest,
		len(in.Pages), memberCount, lease); err != nil {
		return CommunityRosterResult{}, err
	}
	for _, page := range in.Pages {
		canonical, _ := communityRosterPageBytes(page)
		if _, err = tx.Exec(ctx, `INSERT INTO gis_community_roster_pages(operation_id,page_index,page_sha256,canonical_page_bytes)
			VALUES($1,$2,$3,$4)`, in.OperationID, page.Index, page.SHA256, canonical); err != nil {
			return CommunityRosterResult{}, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE gis_community_grants SET revoked_at=clock_timestamp(),revocation_reason='snapshot_replaced',updated_at=clock_timestamp()
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND owner_generation=$4 AND revoked_at IS NULL`,
		p.ApplicationID, p.EnvironmentID, in.CorporationKey, generation); err != nil {
		return CommunityRosterResult{}, err
	}
	for _, page := range in.Pages {
		for _, member := range page.Members {
			for _, reason := range member.GrantReasons {
				if _, err = tx.Exec(ctx, `INSERT INTO gis_community_grants
					(application_id,environment_id,corporation_key,binding_id,character_key,source_reason,source_rank,
					 source_membership_revision,source_revision,owner_generation,lease_expires_at)
					VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
					ON CONFLICT(application_id,environment_id,corporation_key,binding_id,character_key,source_reason,owner_generation)
					DO UPDATE SET source_rank=EXCLUDED.source_rank,source_membership_revision=EXCLUDED.source_membership_revision,
					 source_revision=EXCLUDED.source_revision,lease_expires_at=EXCLUDED.lease_expires_at,
					 revoked_at=NULL,revocation_reason=NULL,updated_at=clock_timestamp()`,
					p.ApplicationID, p.EnvironmentID, in.CorporationKey, member.BindingID, member.CharacterKey,
					reason, member.SourceRank, member.SourceMembershipRevision, in.SourceRevision, generation, lease); err != nil {
					return CommunityRosterResult{}, err
				}
			}
		}
	}
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()+interval '60 seconds'`).Scan(&lease); err != nil {
		return CommunityRosterResult{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE gis_community_roster_snapshots SET lease_expires_at=$2 WHERE operation_id=$1`, in.OperationID, lease); err != nil {
		return CommunityRosterResult{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE gis_community_grants SET lease_expires_at=$4,updated_at=clock_timestamp()
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND source_revision=$5 AND owner_generation=$6 AND revoked_at IS NULL`,
		p.ApplicationID, p.EnvironmentID, in.CorporationKey, lease, in.SourceRevision, generation); err != nil {
		return CommunityRosterResult{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE gis_community_owner_authority
		SET roster_source_revision=$4,roster_sha256=$5,roster_lease_expires_at=$6,updated_at=clock_timestamp()
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND owner_generation=$7 AND status='active'`,
		p.ApplicationID, p.EnvironmentID, in.CorporationKey, in.SourceRevision, digest, lease, generation); err != nil {
		return CommunityRosterResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CommunityRosterResult{}, err
	}
	return CommunityRosterResult{OperationID: in.OperationID, ApplicationID: p.ApplicationID,
		EnvironmentID: p.EnvironmentID, CorporationKey: in.CorporationKey,
		SourceRevision: in.SourceRevision, OwnerGeneration: generation,
		SnapshotSHA256: append([]byte(nil), digest...), Outcome: "accepted",
		MemberCount: memberCount, LeaseExpiresAt: &lease}, nil
}

// validateCommunityRosterSnapshot verifies completeness and hashes before any
// store mutation. The returned digest identifies the complete immutable input.
func validateCommunityRosterSnapshot(in CommunityRosterSnapshotInput) ([]byte, error) {
	if in.OperationID == uuid.Nil || !validCommunityKey(in.CorporationKey) ||
		in.SourceRevision <= 0 || in.OwnerGeneration <= 0 || len(in.Pages) == 0 || len(in.Pages) > 1000 {
		return nil, ErrInvalidCommunityRoster
	}
	pageCount := int32(len(in.Pages))
	seenMembers := make(map[string]struct{})
	pageDigests := make([]string, pageCount)
	lastMemberKey := ""
	for i, page := range in.Pages {
		if page.Index != int32(i) || page.Count != pageCount || len(page.SHA256) != sha256.Size {
			return nil, ErrInvalidCommunityRoster
		}
		for _, member := range page.Members {
			if member.BindingID == uuid.Nil || !validCharacterKey(member.CharacterKey) ||
				member.SourceMembershipRevision <= 0 || !validCommunityRank(member.SourceRank) ||
				!validGrantReasons(member.SourceRank, member.GrantReasons) {
				return nil, ErrInvalidCommunityRoster
			}
			key := member.BindingID.String() + "\x00" + member.CharacterKey
			if lastMemberKey != "" && key <= lastMemberKey {
				return nil, ErrInvalidCommunityRoster
			}
			lastMemberKey = key
			if _, duplicate := seenMembers[key]; duplicate {
				return nil, ErrInvalidCommunityRoster
			}
			seenMembers[key] = struct{}{}
		}
		body, err := communityRosterPageBytes(page)
		if err != nil {
			return nil, fmt.Errorf("canonicalize community roster page: %w", err)
		}
		digest := sha256.Sum256(body)
		if !bytes.Equal(page.SHA256, digest[:]) {
			return nil, ErrInvalidCommunityRoster
		}
		pageDigests[i] = hex.EncodeToString(page.SHA256)
	}
	body, err := communityRosterSnapshotBytes(in, pageDigests)
	if err != nil {
		return nil, fmt.Errorf("canonicalize community roster snapshot: %w", err)
	}
	digest := sha256.Sum256(body)
	if len(in.SnapshotSHA256) != sha256.Size || !bytes.Equal(in.SnapshotSHA256, digest[:]) {
		return nil, ErrInvalidCommunityRoster
	}
	return digest[:], nil
}

func communityRosterPageBytes(page CommunityRosterPage) ([]byte, error) {
	value := struct {
		Index   int32                   `json:"index"`
		Count   int32                   `json:"count"`
		Members []CommunityRosterMember `json:"members"`
	}{page.Index, page.Count, page.Members}
	return canonicalCommunityRosterJSON(value)
}

func communityRosterSnapshotBytes(in CommunityRosterSnapshotInput, pageDigests []string) ([]byte, error) {
	value := struct {
		OperationID     string   `json:"operation_id"`
		CorporationKey  string   `json:"corporation_key"`
		SourceRevision  int64    `json:"source_revision"`
		OwnerGeneration int64    `json:"owner_generation"`
		Pages           []string `json:"pages"`
	}{in.OperationID.String(), in.CorporationKey, in.SourceRevision, in.OwnerGeneration, pageDigests}
	return canonicalCommunityRosterJSON(value)
}

func canonicalCommunityRosterJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return jsoncanonicalizer.Transform(body)
}

func validCommunityKey(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value
}

func validCharacterKey(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value
}

func validCommunityRank(value string) bool {
	return value == "member" || value == "officer"
}

func validGrantReasons(rank string, reasons []string) bool {
	if rank == "member" {
		return len(reasons) == 1 && reasons[0] == "membership"
	}
	return rank == "officer" && len(reasons) == 2 && reasons[0] == "membership" && reasons[1] == "officer_rank"
}
