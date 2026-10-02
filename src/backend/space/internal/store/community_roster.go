package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrCommunityRosterConflict = errors.New("community roster conflicts with current Space authority")

type CommunityRosterInput struct {
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

type CommunityRosterReceipt struct {
	OperationID       uuid.UUID
	SpaceID           uuid.UUID
	OwnerGeneration   int64
	SourceRevision    int64
	SnapshotSHA256    []byte
	RequestSHA256     []byte
	ReceiptID         uuid.UUID
	MemberCount       int32
	Replayed          bool
	Current           bool
	ProfileIDs        []uuid.UUID
	RemovedProfileIDs []uuid.UUID
}

// ApplyCommunityRoster replaces only the game-owned source membership. It
// fences the current Space Owner generation and stores a replayable receipt in
// the same transaction as the effective-membership projection.
func (s *SpaceStore) ApplyCommunityRoster(ctx context.Context, in CommunityRosterInput) (CommunityRosterReceipt, error) {
	if s == nil || s.Pool == nil || in.OperationID == uuid.Nil || in.ApplicationID == uuid.Nil ||
		in.EnvironmentID == uuid.Nil || in.SpaceID == uuid.Nil || in.CorporationKey == "" || len(in.CorporationKey) > 256 ||
		in.OwnerGeneration <= 0 || in.SourceRevision <= 0 || len(in.SnapshotSHA256) != sha256.Size || in.LeaseExpiresAt.IsZero() {
		return CommunityRosterReceipt{}, errors.New("invalid community roster projection")
	}
	profiles := append([]uuid.UUID{}, in.ProfileIDs...)
	for i, id := range profiles {
		if id == uuid.Nil || (i > 0 && profiles[i-1].String() >= id.String()) {
			return CommunityRosterReceipt{}, errors.New("community roster profiles must be unique and sorted")
		}
	}
	requestBytes, err := json.Marshal(struct {
		ApplicationID   uuid.UUID   `json:"application_id"`
		EnvironmentID   uuid.UUID   `json:"environment_id"`
		CorporationKey  string      `json:"corporation_key"`
		SpaceID         uuid.UUID   `json:"space_id"`
		OwnerGeneration int64       `json:"owner_generation"`
		SourceRevision  int64       `json:"source_revision"`
		SnapshotSHA256  []byte      `json:"snapshot_sha256"`
		LeaseExpiresAt  time.Time   `json:"lease_expires_at"`
		ProfileIDs      []uuid.UUID `json:"profile_ids"`
	}{in.ApplicationID, in.EnvironmentID, in.CorporationKey, in.SpaceID, in.OwnerGeneration, in.SourceRevision, in.SnapshotSHA256, in.LeaseExpiresAt.UTC(), profiles})
	if err != nil {
		return CommunityRosterReceipt{}, err
	}
	requestHash := sha256.Sum256(requestBytes)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return CommunityRosterReceipt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var savedHash, savedDigest []byte
	var savedSpace uuid.UUID
	var receipt CommunityRosterReceipt
	err = tx.QueryRow(ctx, `SELECT space_id,owner_generation,source_revision,snapshot_sha256,request_hash,receipt_id,member_count,profile_ids,removed_profile_ids
		FROM community_roster_operations WHERE operation_id=$1`, in.OperationID).Scan(&savedSpace, &receipt.OwnerGeneration,
		&receipt.SourceRevision, &savedDigest, &savedHash, &receipt.ReceiptID, &receipt.MemberCount, &receipt.ProfileIDs, &receipt.RemovedProfileIDs)
	if err == nil {
		if savedSpace != in.SpaceID || !bytes.Equal(savedDigest, in.SnapshotSHA256) || !bytes.Equal(savedHash, requestHash[:]) {
			return CommunityRosterReceipt{}, ErrCommunityRosterConflict
		}
		if queryErr := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM community_owner_authority WHERE space_id=$1 AND status='active'
			AND owner_generation=$2 AND roster_source_revision=$3 AND roster_sha256=$4 AND roster_lease_expires_at>clock_timestamp())`,
			savedSpace, receipt.OwnerGeneration, receipt.SourceRevision, savedDigest).Scan(&receipt.Current); queryErr != nil {
			return CommunityRosterReceipt{}, queryErr
		}
		if err := tx.Commit(ctx); err != nil {
			return CommunityRosterReceipt{}, err
		}
		return CommunityRosterReceipt{OperationID: in.OperationID, SpaceID: savedSpace, OwnerGeneration: receipt.OwnerGeneration,
			SourceRevision: receipt.SourceRevision, SnapshotSHA256: savedDigest, RequestSHA256: savedHash, ReceiptID: receipt.ReceiptID,
			MemberCount: receipt.MemberCount, Replayed: true, Current: receipt.Current, ProfileIDs: receipt.ProfileIDs, RemovedProfileIDs: receipt.RemovedProfileIDs}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CommunityRosterReceipt{}, err
	}
	var generation, currentRevision int64
	var status string
	var currentHash []byte
	err = tx.QueryRow(ctx, `SELECT owner_generation,status,roster_source_revision,roster_sha256
		FROM community_owner_authority WHERE space_id=$1 AND application_id=$2 AND environment_id=$3 AND corporation_key=$4 FOR UPDATE`,
		in.SpaceID, in.ApplicationID, in.EnvironmentID, in.CorporationKey).Scan(&generation, &status, &currentRevision, &currentHash)
	if errors.Is(err, pgx.ErrNoRows) || status != "active" || generation != in.OwnerGeneration {
		return CommunityRosterReceipt{}, ErrCommunityRosterConflict
	}
	if err != nil {
		return CommunityRosterReceipt{}, err
	}
	var leaseValid bool
	if err = tx.QueryRow(ctx, `SELECT $1::timestamptz>clock_timestamp() AND $1::timestamptz<=clock_timestamp()+interval '2 minutes'`, in.LeaseExpiresAt.UTC()).Scan(&leaseValid); err != nil {
		return CommunityRosterReceipt{}, err
	}
	if !leaseValid { return CommunityRosterReceipt{}, ErrCommunityRosterConflict }
	if in.SourceRevision < currentRevision || (in.SourceRevision == currentRevision && !bytes.Equal(currentHash, in.SnapshotSHA256)) {
		return CommunityRosterReceipt{}, ErrCommunityRosterConflict
	}
	if in.SourceRevision == currentRevision && bytes.Equal(currentHash, in.SnapshotSHA256) {
		var prior CommunityRosterReceipt
		err = tx.QueryRow(ctx, `SELECT operation_id,space_id,owner_generation,source_revision,snapshot_sha256,receipt_id,member_count,request_hash,profile_ids,removed_profile_ids
			FROM community_roster_operations WHERE space_id=$1 AND owner_generation=$2 AND source_revision=$3 AND snapshot_sha256=$4`,
			in.SpaceID, generation, in.SourceRevision, in.SnapshotSHA256).Scan(&prior.OperationID, &prior.SpaceID,
			&prior.OwnerGeneration, &prior.SourceRevision, &prior.SnapshotSHA256, &prior.ReceiptID, &prior.MemberCount, &prior.RequestSHA256, &prior.ProfileIDs, &prior.RemovedProfileIDs)
		if err != nil {
			return CommunityRosterReceipt{}, err
		}
		var activeLease bool
		if err = tx.QueryRow(ctx, `SELECT roster_lease_expires_at>clock_timestamp() FROM community_owner_authority WHERE space_id=$1 AND owner_generation=$2 AND status='active'`, in.SpaceID, generation).Scan(&activeLease); err != nil {
			return CommunityRosterReceipt{}, err
		}
		if !activeLease { return CommunityRosterReceipt{}, ErrCommunityRosterConflict }
		if err = tx.Commit(ctx); err != nil {
			return CommunityRosterReceipt{}, err
		}
		prior.Replayed = true
		prior.Current = true
		return prior, nil
	}
	var receiptID uuid.UUID
	leaseExpiry := in.LeaseExpiresAt.UTC()
	if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()`).Scan(&receiptID); err != nil {
		return CommunityRosterReceipt{}, err
	}
	rows, err := tx.Query(ctx, `SELECT profile_id FROM community_roster_members WHERE space_id=$1`, in.SpaceID)
	if err != nil {
		return CommunityRosterReceipt{}, err
	}
	var previous []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if scanErr := rows.Scan(&id); scanErr != nil {
			rows.Close()
			return CommunityRosterReceipt{}, scanErr
		}
		previous = append(previous, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return CommunityRosterReceipt{}, err
	}
	rows.Close()
	newSet := make(map[uuid.UUID]struct{}, len(profiles))
	for _, id := range profiles {
		newSet[id] = struct{}{}
	}
	oldSet := make(map[uuid.UUID]struct{}, len(previous))
	for _, id := range previous {
		oldSet[id] = struct{}{}
	}
	removed := make([]uuid.UUID, 0)
	for _, id := range previous {
		if _, ok := newSet[id]; !ok {
			removed = append(removed, id)
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM community_roster_members WHERE space_id=$1`, in.SpaceID); err != nil {
		return CommunityRosterReceipt{}, err
	}
	for _, profileID := range profiles {
		if _, err = tx.Exec(ctx, `INSERT INTO community_roster_members(space_id,profile_id,source_rank,source_reasons,source_revision,owner_generation,lease_expires_at)
			VALUES($1,$2,'member',ARRAY['membership']::TEXT[],$3,$4,$5)`, in.SpaceID, profileID, in.SourceRevision, generation, leaseExpiry); err != nil {
			return CommunityRosterReceipt{}, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE community_owner_authority SET roster_source_revision=$2,roster_sha256=$3,
		roster_lease_expires_at=$4,updated_at=clock_timestamp() WHERE space_id=$1 AND owner_generation=$5 AND status='active'`,
		in.SpaceID, in.SourceRevision, in.SnapshotSHA256, leaseExpiry, generation); err != nil {
		return CommunityRosterReceipt{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE spaces SET member_count=(SELECT count(*)::int FROM (
		SELECT profile_id FROM space_members WHERE space_id=$1
		UNION SELECT m.profile_id FROM community_roster_members m JOIN community_owner_authority a
		ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation AND a.status='active'
		WHERE m.space_id=$1 AND m.revoked_at IS NULL AND m.lease_expires_at>clock_timestamp() AND a.roster_lease_expires_at>clock_timestamp()
	) active_members),updated_at=clock_timestamp() WHERE id=$1`, in.SpaceID); err != nil {
		return CommunityRosterReceipt{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO community_roster_operations(operation_id,space_id,application_id,environment_id,corporation_key,
		owner_generation,source_revision,snapshot_sha256,request_hash,receipt_id,member_count,profile_ids,removed_profile_ids)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, in.OperationID, in.SpaceID, in.ApplicationID, in.EnvironmentID,
		in.CorporationKey, generation, in.SourceRevision, in.SnapshotSHA256, requestHash[:], receiptID, len(profiles), profiles, removed); err != nil {
		return CommunityRosterReceipt{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CommunityRosterReceipt{}, err
	}
	return CommunityRosterReceipt{OperationID: in.OperationID, SpaceID: in.SpaceID, OwnerGeneration: generation,
		SourceRevision: in.SourceRevision, SnapshotSHA256: append([]byte(nil), in.SnapshotSHA256...), RequestSHA256: append([]byte(nil), requestHash[:]...), ReceiptID: receiptID,
		MemberCount: int32(len(profiles)), Current: true, ProfileIDs: profiles, RemovedProfileIDs: removed}, nil
}

func (s *SpaceStore) RevokeExpiredCommunityRosterMemberships(ctx context.Context) (int64, error) {
	if s == nil || s.Pool == nil {
		return 0, errors.New("space store: pool not configured")
	}
	// Keep the durable source rows for operation history, but remove them from
	// effective access immediately through lease-filtered membership queries.
	tag, err := s.Pool.Exec(ctx, `WITH due AS (
		SELECT ctid FROM community_roster_members WHERE revoked_at IS NULL AND lease_expires_at<=clock_timestamp()
		ORDER BY lease_expires_at,space_id,profile_id LIMIT 500 FOR UPDATE SKIP LOCKED
	), revoked AS (
		UPDATE community_roster_members m SET revoked_at=clock_timestamp() FROM due WHERE m.ctid=due.ctid RETURNING m.space_id
	), affected AS (SELECT DISTINCT space_id FROM revoked)
	UPDATE spaces s SET member_count=(SELECT count(*)::int FROM (
		SELECT profile_id FROM space_members WHERE space_id=s.id
		UNION SELECT m.profile_id FROM community_roster_members m JOIN community_owner_authority a
		ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation AND a.status='active'
		WHERE m.space_id=s.id AND m.revoked_at IS NULL AND m.lease_expires_at>clock_timestamp() AND a.roster_lease_expires_at>clock_timestamp()
	) active_members),updated_at=clock_timestamp() FROM affected WHERE s.id=affected.space_id`)
	if err != nil {
		return 0, fmt.Errorf("expire community roster leases: %w", err)
	}
	return tag.RowsAffected(), nil
}

func RunCommunityRosterExpiryWorker(ctx context.Context, s *SpaceStore, interval time.Duration) {
	if interval <= 0 { interval = time.Second }
	ticker:=time.NewTicker(interval); defer ticker.Stop()
	for {
		select {
		case <-ctx.Done(): return
		case <-ticker.C:
			workCtx,cancel:=context.WithTimeout(ctx,5*time.Second)
			count,err:=s.RevokeExpiredCommunityRosterMemberships(workCtx); cancel()
			if err!=nil { slog.Warn("Space community roster expiry failed","error",err) } else if count>0 { slog.Info("Space community membership lease expired","count",count) }
		}
	}
}
