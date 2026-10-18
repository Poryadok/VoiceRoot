package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"voice/backend/pkg/spaceaudit"
)

var ErrAuditEventConflict = errors.New("audit event conflicts with reserved ID")

// ImportAuditEvent records an already-committed, authenticated producer fact.
// It owns its transaction so a uniqueness race can roll back before a fresh
// committed receipt read. Current member/role state must not rewrite past facts.
func (s *SpaceStore) ImportAuditEvent(ctx context.Context, producer string, event spaceaudit.ReplayEvent) error {
	if err := spaceaudit.ValidateEvent(producer, event); err != nil {
		return err
	}
	digest, err := spaceaudit.ReplayFingerprint(event)
	if err != nil {
		return err
	}
	if s == nil || s.Pool == nil || s.tx != nil {
		return errors.New("audit ingestion requires an independent pool transaction")
	}
	id, _ := uuid.Parse(event.AuditEventID)
	spaceID, _ := uuid.Parse(event.SpaceID)
	actorID, _ := uuid.Parse(event.ActorProfileID)
	targetID, _ := uuid.Parse(event.TargetID)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin audit ingestion: %w", err)
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err := lockLifecycleSpace(ctx, tx, spaceID); err != nil {
		return fmt.Errorf("lock audit lifecycle: %w", err)
	}
	// Composite freeze/purge admission is added here by the accepted c5 integration
	// against its persisted outcome schema, before even an exact receipt replay.
	if found, err := s.matchAuditReceipt(ctx, tx, id, spaceID, producer, digest[:]); found || err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_log
        (id,space_id,actor_profile_id,action,target_type,target_id,details,created_at,
         ingress_origin,ingress_fingerprint_version,ingress_fingerprint)
        VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11)`, id, spaceID, actorID,
		event.Action, event.TargetType, targetID, event.DetailsJSON, event.OccurredAt.AsTime().UTC(),
		producer, spaceaudit.FingerprintVersion, digest[:])
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				return fmt.Errorf("roll back audit collision: %w", rollbackErr)
			}
			readTx, beginErr := s.Pool.Begin(ctx)
			if beginErr != nil {
				return fmt.Errorf("begin audit collision read: %w", beginErr)
			}
			defer rollbackLifecycleTx(ctx, readTx)
			if lockErr := lockLifecycleSpace(ctx, readTx, spaceID); lockErr != nil {
				return fmt.Errorf("lock audit collision lifecycle: %w", lockErr)
			}
			found, receiptErr := s.matchAuditReceipt(ctx, readTx, id, spaceID, producer, digest[:])
			if found || receiptErr != nil {
				return receiptErr
			}
		}
		return fmt.Errorf("insert audit fact: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		// A lost commit acknowledgement is deliberately not a second mutation.
		// A fresh authenticated retry resolves the durable receipt first.
		return fmt.Errorf("commit audit fact: %w", err)
	}
	return nil
}

func (s *SpaceStore) matchAuditReceipt(ctx context.Context, db spaceStoreDB, id, spaceID uuid.UUID, producer string, digest []byte) (bool, error) {
	var savedSpace uuid.UUID
	var origin string
	var version *int
	var savedDigest []byte
	err := db.QueryRow(ctx, `SELECT space_id,origin,fingerprint_version,fingerprint
        FROM audit_event_receipts WHERE audit_event_id=$1`, id).Scan(&savedSpace, &origin, &version, &savedDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read audit receipt: %w", err)
	}
	if savedSpace != spaceID || origin != producer || version == nil || *version != spaceaudit.FingerprintVersion || !bytes.Equal(savedDigest, digest) {
		return true, ErrAuditEventConflict
	}
	return true, nil
}
