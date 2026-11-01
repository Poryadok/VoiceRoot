package registry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CommunityRosterProjection struct {
	OperationID     uuid.UUID
	ApplicationID   uuid.UUID
	EnvironmentID   uuid.UUID
	CorporationKey  string
	SpaceID         uuid.UUID
	OwnerGeneration int64
	SourceRevision  int64
	SnapshotSHA256  []byte
	LeaseExpiresAt  time.Time
	ProfileIDs      []uuid.UUID
}

type CommunityRosterSpaceReceipt struct {
	OperationID uuid.UUID
	SpaceID     uuid.UUID
	ReceiptID   uuid.UUID
	RequestHash []byte
	MemberCount int32
}

type CommunityRosterSpaceProjector interface {
	ProjectCommunityRoster(context.Context, CommunityRosterProjection) (CommunityRosterSpaceReceipt, error)
}

func (s *Store) BuildCommunityRosterProjection(ctx context.Context, p SessionPrincipal, in CommunityRosterSnapshotInput) (CommunityRosterProjection, error) {
	if s == nil || s.Pool == nil {
		return CommunityRosterProjection{}, ErrRegistryUnavailable
	}
	var projection CommunityRosterProjection
	err := s.Pool.QueryRow(ctx, `SELECT a.space_id,a.owner_generation,a.roster_source_revision,a.roster_sha256,a.roster_lease_expires_at
		FROM gis_community_owner_authority a JOIN gis_community_roster_snapshots r
		ON r.application_id=a.application_id AND r.environment_id=a.environment_id AND r.corporation_key=a.corporation_key
		WHERE a.application_id=$1 AND a.environment_id=$2 AND a.corporation_key=$3 AND a.status='active'
		AND r.operation_id=$4 AND r.outcome='accepted' AND a.owner_generation=r.owner_generation
		AND a.roster_source_revision=r.source_revision AND a.roster_sha256=r.snapshot_sha256
		AND a.roster_lease_expires_at>clock_timestamp()`, p.ApplicationID, p.EnvironmentID, in.CorporationKey, in.OperationID).
		Scan(&projection.SpaceID, &projection.OwnerGeneration, &projection.SourceRevision, &projection.SnapshotSHA256, &projection.LeaseExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CommunityRosterProjection{}, ErrStaleCommunityRoster
	}
	if err != nil {
		return CommunityRosterProjection{}, err
	}
	projection.OperationID, projection.ApplicationID, projection.EnvironmentID, projection.CorporationKey = in.OperationID, p.ApplicationID, p.EnvironmentID, in.CorporationKey
	rows, err := s.Pool.Query(ctx, `SELECT DISTINCT b.profile_id FROM gis_community_grants g JOIN player_bindings b
		ON b.binding_id=g.binding_id AND b.application_id=g.application_id AND b.environment_id=g.environment_id
		WHERE g.application_id=$1 AND g.environment_id=$2 AND g.corporation_key=$3 AND g.owner_generation=$4
		AND g.source_revision=$5 AND g.revoked_at IS NULL AND g.lease_expires_at>clock_timestamp()
		AND b.status='active' AND b.permitted_character_context ? g.character_key ORDER BY b.profile_id`,
		p.ApplicationID, p.EnvironmentID, in.CorporationKey, projection.OwnerGeneration, projection.SourceRevision)
	if err != nil {
		return CommunityRosterProjection{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return CommunityRosterProjection{}, err
		}
		projection.ProfileIDs = append(projection.ProfileIDs, id)
	}
	if err := rows.Err(); err != nil {
		return CommunityRosterProjection{}, err
	}
	if projection.ProfileIDs == nil {
		projection.ProfileIDs = []uuid.UUID{}
	}
	return projection, nil
}

func (s *Store) RecordCommunityRosterSpaceReceipt(ctx context.Context, projection CommunityRosterProjection, receipt CommunityRosterSpaceReceipt) error {
	if s == nil || s.Pool == nil || receipt.OperationID != projection.OperationID || receipt.SpaceID != projection.SpaceID || receipt.ReceiptID == uuid.Nil ||
		receipt.MemberCount != int32(len(projection.ProfileIDs)) || len(receipt.RequestHash) != sha256.Size {
		return ErrInvalidCommunityRoster
	}
	body, err := json.Marshal(struct {
		ApplicationID   uuid.UUID   `json:"application_id"`
		EnvironmentID   uuid.UUID   `json:"environment_id"`
		CorporationKey  string      `json:"corporation_key"`
		SpaceID         uuid.UUID   `json:"space_id"`
		OwnerGeneration int64       `json:"owner_generation"`
		SourceRevision  int64       `json:"source_revision"`
		SnapshotSHA256  []byte      `json:"snapshot_sha256"`
		LeaseExpiresAt  time.Time   `json:"lease_expires_at"`
		ProfileIDs      []uuid.UUID `json:"profile_ids"`
	}{
		projection.ApplicationID, projection.EnvironmentID, projection.CorporationKey, projection.SpaceID,
		projection.OwnerGeneration, projection.SourceRevision, projection.SnapshotSHA256,
		projection.LeaseExpiresAt.UTC(), projection.ProfileIDs})
	if err != nil {
		return err
	}
	want := sha256.Sum256(body)
	if !bytes.Equal(want[:], receipt.RequestHash) {
		return ErrIdempotencyConflict
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE gis_community_roster_snapshots r SET space_projection_status='applied',space_receipt_id=$2,space_request_hash=$3
		FROM gis_community_owner_authority a WHERE r.operation_id=$1 AND r.application_id=$4 AND r.environment_id=$5 AND r.corporation_key=$6
		AND r.owner_generation=$7 AND r.source_revision=$8 AND r.snapshot_sha256=$9 AND r.outcome='accepted'
		AND a.application_id=r.application_id AND a.environment_id=r.environment_id AND a.corporation_key=r.corporation_key
		AND a.space_id=$10 AND a.owner_generation=r.owner_generation AND a.status='active'
		AND a.roster_source_revision=r.source_revision AND a.roster_sha256=r.snapshot_sha256
		AND a.roster_lease_expires_at>clock_timestamp()
		AND (r.space_projection_status='pending' OR (r.space_receipt_id=$2 AND r.space_request_hash=$3))`,
		projection.OperationID, receipt.ReceiptID, receipt.RequestHash, projection.ApplicationID, projection.EnvironmentID, projection.CorporationKey,
		projection.OwnerGeneration, projection.SourceRevision, projection.SnapshotSHA256, projection.SpaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleCommunityRoster
	}
	return nil
}
