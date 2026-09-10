package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrOwnershipBindingInvalid = errors.New("invalid ownership binding")
	ErrOwnershipConflict       = errors.New("ownership operation binding conflict")
	ErrOwnershipActive         = errors.New("space has an active ownership operation")
	ErrOwnershipMissing        = errors.New("ownership operation not found")
)

// OwnershipBinding contains the immutable, non-secret authorization identity.
// ActorProfileID is the old owner. ProofDigest identifies the original proof;
// the opaque proof and credentials must never enter this store API.
type OwnershipBinding struct {
	ProtocolVersion   uint32
	OperationID       uuid.UUID
	SpaceID           uuid.UUID
	AccountID         uuid.UUID
	ActorProfileID    uuid.UUID
	NewOwnerProfileID uuid.UUID
	SessionEpoch      int64
	ProofDigest       string
}

// OwnershipJournal is durable operation evidence, including stable artifact IDs.
type OwnershipJournal struct {
	Binding      OwnershipBinding
	BindingBytes []byte
	BindingHash  [32]byte
	State        string
	AuditID      uuid.UUID
	EventID      uuid.UUID
}

// EncodeOwnershipBinding derives codec-v1 bytes and their digest exclusively
// from validated typed fields. Its encoding version is independent of the
// ownership protocol version; callers cannot supply a competing body or hash.
func EncodeOwnershipBinding(b OwnershipBinding) ([]byte, [32]byte, error) {
	if b.ProtocolVersion != 2 || b.OperationID == uuid.Nil || b.SpaceID == uuid.Nil ||
		b.AccountID == uuid.Nil || b.ActorProfileID == uuid.Nil || b.NewOwnerProfileID == uuid.Nil ||
		b.ActorProfileID == b.NewOwnerProfileID || b.SessionEpoch <= 0 ||
		len(b.ProofDigest) != 64 || strings.ToLower(b.ProofDigest) != b.ProofDigest {
		return nil, [32]byte{}, ErrOwnershipBindingInvalid
	}
	proofDigest, err := hex.DecodeString(b.ProofDigest)
	if err != nil {
		return nil, [32]byte{}, ErrOwnershipBindingInvalid
	}
	encoded := []byte("voice.space.ownership.binding.v1\x00")
	encoded = binary.BigEndian.AppendUint32(encoded, b.ProtocolVersion)
	for _, id := range []uuid.UUID{b.OperationID, b.SpaceID, b.AccountID, b.ActorProfileID, b.NewOwnerProfileID} {
		encoded = append(encoded, id[:]...)
	}
	encoded = binary.BigEndian.AppendUint64(encoded, uint64(b.SessionEpoch))
	encoded = append(encoded, proofDigest...)
	return encoded, sha256.Sum256(encoded), nil
}

// ReserveOwnership persists an immutable reservation before proof consumption.
// Callers must not hold a legacy session mutation lease on another connection.
func (s *SpaceStore) ReserveOwnership(ctx context.Context, b OwnershipBinding) (*OwnershipJournal, error) {
	encoded, hash, err := EncodeOwnershipBinding(b)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		cleanupCtx, cancel := BoundedCleanupContext(ctx)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()
	if err := lockOwnershipTransaction(ctx, tx, b.OperationID, b.SpaceID); err != nil {
		return nil, err
	}
	existing, err := scanOwnershipJournal(tx.QueryRow(ctx,
		`SELECT `+ownershipJournalColumns+` FROM ownership_journal WHERE operation_id=$1`, b.OperationID))
	if err == nil {
		if !bytes.Equal(existing.BindingBytes, encoded) {
			return nil, ErrOwnershipConflict
		}
		return existing, nil
	}
	if !errors.Is(err, ErrOwnershipMissing) {
		return nil, err
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ownership_journal
		WHERE space_id=$1 AND state NOT IN ('completed','aborted'))`, b.SpaceID).Scan(&active); err != nil {
		return nil, err
	}
	if active {
		return nil, ErrOwnershipActive
	}
	var owner uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1 FOR UPDATE`, b.SpaceID).Scan(&owner); err != nil {
		return nil, err
	}
	if owner != b.ActorProfileID {
		return nil, ErrNotSpaceOwner
	}
	var member uuid.UUID
	err = tx.QueryRow(ctx, `SELECT profile_id FROM space_members WHERE space_id=$1 AND profile_id=$2 FOR KEY SHARE`, b.SpaceID, b.NewOwnerProfileID).Scan(&member)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	row, err := scanOwnershipJournal(tx.QueryRow(ctx, `INSERT INTO ownership_journal
		(operation_id,protocol_version,space_id,account_id,actor_profile_id,new_owner_profile_id,
		session_epoch,proof_digest,binding_bytes,binding_hash,audit_id,event_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+ownershipJournalColumns,
		b.OperationID, b.ProtocolVersion, b.SpaceID, b.AccountID, b.ActorProfileID, b.NewOwnerProfileID,
		b.SessionEpoch, b.ProofDigest, encoded, hash[:], uuid.New(), uuid.New()))
	if err != nil {
		var constraint *pgconn.PgError
		if errors.As(err, &constraint) && constraint.Code == "23505" {
			switch constraint.ConstraintName {
			case "ownership_journal_pkey":
				return nil, ErrOwnershipConflict
			case "ownership_journal_one_active_space":
				return nil, ErrOwnershipActive
			}
		}
		return nil, err
	}
	// A failed commit may have succeeded remotely. Never infer an opposite
	// decision; callers recover by loading the same durable operation.
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return row, nil
}

// LoadOwnership reads authoritative durable evidence without recreating it.
func (s *SpaceStore) LoadOwnership(ctx context.Context, operationID uuid.UUID) (*OwnershipJournal, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	return scanOwnershipJournal(s.Pool.QueryRow(ctx,
		`SELECT `+ownershipJournalColumns+` FROM ownership_journal WHERE operation_id=$1`, operationID))
}

const ownershipJournalColumns = `operation_id,protocol_version,space_id,account_id,actor_profile_id,
	new_owner_profile_id,session_epoch,proof_digest,binding_bytes,binding_hash,state,audit_id,event_id`

func scanOwnershipJournal(row pgx.Row) (*OwnershipJournal, error) {
	out := new(OwnershipJournal)
	b := &out.Binding
	var storedHash []byte
	err := row.Scan(&b.OperationID, &b.ProtocolVersion, &b.SpaceID, &b.AccountID,
		&b.ActorProfileID, &b.NewOwnerProfileID, &b.SessionEpoch, &b.ProofDigest,
		&out.BindingBytes, &storedHash, &out.State, &out.AuditID, &out.EventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOwnershipMissing
	}
	if err != nil {
		return nil, err
	}
	canonical, hash, err := EncodeOwnershipBinding(*b)
	if err != nil || !bytes.Equal(canonical, out.BindingBytes) || !bytes.Equal(hash[:], storedHash) {
		return nil, errors.New("ownership journal binding evidence is inconsistent")
	}
	out.BindingHash = hash
	return out, nil
}
