package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
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
	AuthReceipt  *OwnershipAuthReceipt
	RoleReceipt  *OwnershipRolePreparedReceipt
	PendingAudit *OwnershipPendingAudit

	terminalReceiptBytes []byte
	terminalReceiptHash  [32]byte
	hasTerminalReceipt   bool
}

// OwnershipTerminalReceiptEvidence returns defensive copies of immutable
// authoritative terminal evidence when the journal has reached a terminal state.
func (j *OwnershipJournal) OwnershipTerminalReceiptEvidence() ([]byte, [32]byte, bool) {
	if j == nil || !j.hasTerminalReceipt {
		return nil, [32]byte{}, false
	}
	return append([]byte(nil), j.terminalReceiptBytes...), j.terminalReceiptHash, true
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
	existing, err := loadOwnershipJournal(ctx, tx, b.OperationID)
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
	return loadOwnershipJournal(ctx, s.Pool, operationID)
}

const ownershipJournalColumns = `operation_id,protocol_version,space_id,account_id,actor_profile_id,
	new_owner_profile_id,session_epoch,proof_digest,binding_bytes,binding_hash,state,audit_id,event_id,
	auth_receipt_id,auth_account_id,auth_profile_id,auth_space_id,auth_new_owner_profile_id,
	auth_operation_id,auth_session_epoch,auth_consumed_at,auth_verified_factors`

const ownershipJournalCommitColumns = ownershipJournalColumns + `,
	role_receipt_bytes,role_receipt_hash,role_intent_bytes,role_intent_hash,
	pending_audit_action,pending_audit_target_type,pending_audit_target_id,pending_audit_details::text`

type ownershipJournalQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadOwnershipJournal(ctx context.Context, querier ownershipJournalQuerier, operationID uuid.UUID) (*OwnershipJournal, error) {
	var hasCommitSchema, hasTerminalSchema bool
	if err := querier.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM information_schema.columns
		WHERE table_schema=current_schema() AND table_name='ownership_journal' AND column_name='role_receipt_bytes'
	), EXISTS(
		SELECT 1 FROM information_schema.columns
		WHERE table_schema=current_schema() AND table_name='ownership_journal' AND column_name='role_terminal_receipt_bytes'
	)`).Scan(&hasCommitSchema, &hasTerminalSchema); err != nil {
		return nil, err
	}
	if hasTerminalSchema {
		return scanOwnershipJournalTerminal(querier.QueryRow(ctx,
			`SELECT `+ownershipJournalTerminalColumns+` FROM ownership_journal WHERE operation_id=$1`, operationID))
	}
	if hasCommitSchema {
		return scanOwnershipJournalCommit(querier.QueryRow(ctx,
			`SELECT `+ownershipJournalCommitColumns+` FROM ownership_journal WHERE operation_id=$1`, operationID))
	}
	return scanOwnershipJournal(querier.QueryRow(ctx,
		`SELECT `+ownershipJournalColumns+` FROM ownership_journal WHERE operation_id=$1`, operationID))
}

func scanOwnershipJournal(row pgx.Row) (*OwnershipJournal, error) {
	return scanOwnershipJournalEvidence(row, false, false)
}

func scanOwnershipJournalCommit(row pgx.Row) (*OwnershipJournal, error) {
	return scanOwnershipJournalEvidence(row, true, false)
}

const ownershipJournalTerminalColumns = ownershipJournalCommitColumns + `,
	role_terminal_receipt_bytes,role_terminal_receipt_hash`

func scanOwnershipJournalTerminal(row pgx.Row) (*OwnershipJournal, error) {
	return scanOwnershipJournalEvidence(row, true, true)
}

func scanOwnershipJournalEvidence(row pgx.Row, includeCommit, includeTerminal bool) (*OwnershipJournal, error) {
	out := new(OwnershipJournal)
	b := &out.Binding
	var storedHash []byte
	var receiptID, accountID, profileID, spaceID, newOwnerID, operationID *uuid.UUID
	var sessionEpoch *int64
	var consumedAt *time.Time
	var verifiedFactors []string
	var roleReceiptBytes, roleReceiptHash, roleIntentBytes, roleIntentHash []byte
	var pendingAction, pendingTargetType, pendingDetails *string
	var pendingTargetID *uuid.UUID
	var terminalReceiptBytes, terminalReceiptHash []byte
	destinations := []any{&b.OperationID, &b.ProtocolVersion, &b.SpaceID, &b.AccountID,
		&b.ActorProfileID, &b.NewOwnerProfileID, &b.SessionEpoch, &b.ProofDigest,
		&out.BindingBytes, &storedHash, &out.State, &out.AuditID, &out.EventID,
		&receiptID, &accountID, &profileID, &spaceID, &newOwnerID, &operationID,
		&sessionEpoch, &consumedAt, &verifiedFactors}
	if includeCommit {
		destinations = append(destinations, &roleReceiptBytes, &roleReceiptHash, &roleIntentBytes, &roleIntentHash,
			&pendingAction, &pendingTargetType, &pendingTargetID, &pendingDetails)
	}
	if includeTerminal {
		destinations = append(destinations, &terminalReceiptBytes, &terminalReceiptHash)
	}
	err := row.Scan(destinations...)
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
	if receiptID == nil {
		if accountID != nil || profileID != nil || spaceID != nil || newOwnerID != nil ||
			operationID != nil || sessionEpoch != nil || consumedAt != nil || verifiedFactors != nil {
			return nil, errors.New("ownership journal auth receipt evidence is incomplete")
		}
		return finishOwnershipTerminalEvidence(out, includeCommit, includeTerminal, roleReceiptBytes, roleReceiptHash, roleIntentBytes, roleIntentHash,
			pendingAction, pendingTargetType, pendingTargetID, pendingDetails, terminalReceiptBytes, terminalReceiptHash)
	}
	if accountID == nil || profileID == nil || spaceID == nil || newOwnerID == nil ||
		operationID == nil || sessionEpoch == nil || consumedAt == nil || verifiedFactors == nil {
		return nil, errors.New("ownership journal auth receipt evidence is incomplete")
	}
	receipt := OwnershipAuthReceipt{
		ReceiptID:         *receiptID,
		AccountID:         *accountID,
		ProfileID:         *profileID,
		SpaceID:           *spaceID,
		NewOwnerProfileID: *newOwnerID,
		OperationID:       *operationID,
		SessionEpoch:      *sessionEpoch,
		ConsumedAt:        consumedAt.UTC(),
		VerifiedFactors:   append([]string(nil), verifiedFactors...),
	}
	if err := validateOwnershipAuthReceiptShape(receipt); err != nil || !ownershipAuthReceiptMatchesBinding(receipt, *b) {
		return nil, errors.New("ownership journal auth receipt evidence is inconsistent")
	}
	out.AuthReceipt = &receipt
	return finishOwnershipTerminalEvidence(out, includeCommit, includeTerminal, roleReceiptBytes, roleReceiptHash, roleIntentBytes, roleIntentHash,
		pendingAction, pendingTargetType, pendingTargetID, pendingDetails, terminalReceiptBytes, terminalReceiptHash)
}

func finishOwnershipTerminalEvidence(out *OwnershipJournal, includeCommit, includeTerminal bool,
	receiptBytes, receiptHash, intentBytes, intentHash []byte,
	pendingAction, pendingTargetType *string, pendingTargetID *uuid.UUID, pendingDetails *string,
	terminalReceiptBytes, terminalReceiptHash []byte,
) (*OwnershipJournal, error) {
	out, err := finishOwnershipCommitEvidence(out, includeCommit, receiptBytes, receiptHash, intentBytes, intentHash,
		pendingAction, pendingTargetType, pendingTargetID, pendingDetails)
	if err != nil || !includeTerminal {
		return out, err
	}
	if terminalReceiptBytes == nil {
		if terminalReceiptHash != nil || out.State == "completed" || out.State == "aborted" {
			return nil, errors.New("ownership journal terminal receipt evidence is incomplete")
		}
		return out, nil
	}
	digest := sha256.Sum256(terminalReceiptBytes)
	if len(terminalReceiptHash) != sha256.Size || !bytes.Equal(terminalReceiptHash, digest[:]) || (out.State != "completed" && out.State != "aborted") {
		return nil, errors.New("ownership journal terminal receipt evidence is inconsistent")
	}
	var decoded rolev1.OwnershipTransferReceipt
	if err := proto.Unmarshal(terminalReceiptBytes, &decoded); err != nil {
		return nil, errors.New("ownership journal terminal receipt evidence is inconsistent")
	}
	wantState := rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED
	wantOwner := out.Binding.ActorProfileID
	if out.State == "completed" {
		wantState = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED
		wantOwner = out.Binding.NewOwnerProfileID
	}
	evidence, err := validateOwnershipTerminalReceipt(out.Binding, &decoded, wantState, wantOwner)
	if err != nil || !bytes.Equal(evidence.bytes, terminalReceiptBytes) {
		return nil, errors.New("ownership journal terminal receipt evidence is inconsistent")
	}
	if out.RoleReceipt != nil {
		if !bytes.Equal(out.RoleReceipt.IntentBytes, evidence.intentBytes) {
			return nil, errors.New("ownership journal terminal receipt evidence is inconsistent")
		}
	} else {
		canonicalIntent, canonicalErr := canonicalOwnershipIntentBytes(out.Binding)
		if canonicalErr != nil || !bytes.Equal(canonicalIntent, evidence.intentBytes) {
			return nil, errors.New("ownership journal terminal receipt evidence is inconsistent")
		}
	}
	copy(out.terminalReceiptHash[:], terminalReceiptHash)
	out.terminalReceiptBytes = append([]byte(nil), terminalReceiptBytes...)
	out.hasTerminalReceipt = true
	return out, nil
}

func finishOwnershipCommitEvidence(out *OwnershipJournal, includeCommit bool, receiptBytes, receiptHash, intentBytes, intentHash []byte,
	pendingAction, pendingTargetType *string, pendingTargetID *uuid.UUID, pendingDetails *string,
) (*OwnershipJournal, error) {
	if !includeCommit {
		return out, nil
	}
	if receiptBytes == nil {
		if receiptHash != nil || intentBytes != nil || intentHash != nil {
			return nil, errors.New("ownership journal role receipt evidence is incomplete")
		}
	} else {
		receiptDigest := sha256.Sum256(receiptBytes)
		intentDigest := sha256.Sum256(intentBytes)
		if len(receiptHash) != sha256.Size || intentBytes == nil || len(intentHash) != sha256.Size ||
			!bytes.Equal(receiptHash, receiptDigest[:]) || !bytes.Equal(intentHash, intentDigest[:]) {
			return nil, errors.New("ownership journal role receipt evidence is inconsistent")
		}
		var storedReceiptHash, storedIntentHash [32]byte
		copy(storedReceiptHash[:], receiptHash)
		copy(storedIntentHash[:], intentHash)
		stored := &OwnershipRolePreparedReceipt{
			ReceiptBytes: append([]byte(nil), receiptBytes...),
			ReceiptHash:  storedReceiptHash,
			IntentBytes:  append([]byte(nil), intentBytes...),
			IntentHash:   storedIntentHash,
		}
		decoded, err := decodeOwnershipRolePreparedReceipt(out.Binding, stored.ReceiptBytes)
		if err != nil || !ownershipRolePreparedReceiptsEqual(stored, decoded) {
			return nil, errors.New("ownership journal role receipt evidence is inconsistent")
		}
		out.RoleReceipt = stored
	}
	if pendingAction == nil {
		if pendingTargetType != nil || pendingTargetID != nil || pendingDetails != nil {
			return nil, errors.New("ownership journal pending audit evidence is incomplete")
		}
		return out, nil
	}
	if out.RoleReceipt == nil || pendingTargetType == nil || pendingTargetID == nil || pendingDetails == nil ||
		*pendingAction != "ownership_transferred" || *pendingTargetType != "profile" ||
		*pendingTargetID != out.Binding.NewOwnerProfileID || *pendingDetails != "{}" {
		return nil, errors.New("ownership journal pending audit evidence is inconsistent")
	}
	out.PendingAudit = &OwnershipPendingAudit{
		ID:             out.AuditID,
		SpaceID:        out.Binding.SpaceID,
		ActorProfileID: out.Binding.ActorProfileID,
		Action:         *pendingAction,
		TargetType:     *pendingTargetType,
		TargetID:       *pendingTargetID,
		DetailsJSON:    *pendingDetails,
	}
	return out, nil
}
