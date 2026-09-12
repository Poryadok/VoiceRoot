package store

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const providerEventHMACDomain = "voice-subscription-provider-dedup-v1"

var (
	ErrLifecycleBinding     = errors.New("space lifecycle operation binding conflict")
	ErrLifecycleGeneration  = errors.New("space lifecycle generation gap")
	ErrLifecycleState       = errors.New("space lifecycle state blocks operation")
	ErrProviderCancellation = errors.New("provider renewal cancellation failed")
)

type LifecycleFenceInput struct {
	SpaceID             uuid.UUID
	DeletionOperationID uuid.UUID
	Generation          int64
	State               string
	ManifestID          string
	ManifestSHA256      []byte
	ManifestItemCount   int64
	RequestSHA256       []byte
	RequestBytes        []byte
	ReceiptID           uuid.UUID
	ReceiptBytes        []byte
	AppliedAt           time.Time
}

type LifecycleFenceRecord struct {
	SpaceID             uuid.UUID
	DeletionOperationID uuid.UUID
	Generation          int64
	State               string
	ManifestID          string
	ManifestSHA256      []byte
	ManifestItemCount   int64
	RequestSHA256       []byte
	ReceiptID           uuid.UUID
	AppliedAt           time.Time
	ReceiptBytes        []byte
}

type PurgeInput struct {
	SpaceID             uuid.UUID
	DeletionOperationID uuid.UUID
	Generation          int64
	ManifestID          string
	ManifestSHA256      []byte
	ManifestItemCount   int64
	RequestSHA256       []byte
	RequestBytes        []byte
	ReceiptID           uuid.UUID
	ReceiptBytes        []byte
	CompletedAt         time.Time
}

type RenewalCanceller func(ctx context.Context, provider, providerSubscriptionID, idempotencyKey string) error

type ProviderEventHMACKey struct {
	Version string
	Key     []byte
}

func (s *SubscriptionStore) RecordProviderEventTx(ctx context.Context, tx pgx.Tx, provider string, eventID []byte, terminalOutcomeClass, keyVersion string, hmacKey []byte) (bool, error) {
	return s.RecordProviderEventWithKeysTx(ctx, tx, provider, eventID, terminalOutcomeClass, []ProviderEventHMACKey{{Version: keyVersion, Key: hmacKey}})
}

func (s *SubscriptionStore) RecordProviderEventWithKeysTx(ctx context.Context, tx pgx.Tx, provider string, eventID []byte, terminalOutcomeClass string, keys []ProviderEventHMACKey) (bool, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "paddle" && provider != "cloudpayments" {
		return false, fmt.Errorf("unsupported provider %q", provider)
	}
	if len(eventID) == 0 || len(keys) == 0 || strings.TrimSpace(terminalOutcomeClass) == "" {
		return false, errors.New("provider event HMAC key and complete binding are required")
	}
	type candidate struct {
		version string
		digest  []byte
	}
	candidates := make([]candidate, 0, len(keys))
	seenVersions := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		version := strings.TrimSpace(key.Version)
		if version == "" || len(key.Key) == 0 {
			return false, errors.New("provider event HMAC key and complete binding are required")
		}
		if _, duplicate := seenVersions[version]; duplicate {
			continue
		}
		seenVersions[version] = struct{}{}
		candidates = append(candidates, candidate{version: version, digest: providerEventDigest(provider, eventID, key.Key)})
	}
	if len(candidates) == 0 {
		return false, errors.New("provider event HMAC key and complete binding are required")
	}
	lockDigest := sha256.Sum256(append(append([]byte(provider), 0), eventID...))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(uint64(lockDigest[0])<<56|uint64(lockDigest[1])<<48|uint64(lockDigest[2])<<40|uint64(lockDigest[3])<<32|uint64(lockDigest[4])<<24|uint64(lockDigest[5])<<16|uint64(lockDigest[6])<<8|uint64(lockDigest[7]))); err != nil {
		return false, err
	}
	for _, candidate := range candidates {
		var storedOutcome, storedVersion string
		err := tx.QueryRow(ctx, `
SELECT terminal_outcome_class,key_version
FROM subscription_provider_event_fences
WHERE provider=$1 AND provider_event_hmac=$2
FOR UPDATE`, provider, candidate.digest).Scan(&storedOutcome, &storedVersion)
		if err == nil {
			if storedOutcome != terminalOutcomeClass || storedVersion != candidate.version {
				return false, ErrLifecycleBinding
			}
			return true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, err
		}
	}
	current := candidates[0]
	tag, err := tx.Exec(ctx, `
INSERT INTO subscription_provider_event_fences(
 provider,provider_event_hmac,first_seen_at,terminal_outcome_class,key_version,retain_until)
VALUES($1,$2,now(),$3,$4,'infinity')
ON CONFLICT(provider,provider_event_hmac) DO NOTHING`, provider, current.digest, terminalOutcomeClass, current.version)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 1 {
		return false, nil
	}
	var storedOutcome, storedVersion string
	if err := tx.QueryRow(ctx, `
SELECT terminal_outcome_class,key_version
FROM subscription_provider_event_fences
WHERE provider=$1 AND provider_event_hmac=$2`, provider, current.digest).Scan(&storedOutcome, &storedVersion); err != nil {
		return false, err
	}
	if storedOutcome != terminalOutcomeClass || storedVersion != current.version {
		return false, ErrLifecycleBinding
	}
	return true, nil
}

func providerEventDigest(provider string, eventID, hmacKey []byte) []byte {
	mac := hmac.New(sha256.New, hmacKey)
	_, _ = mac.Write([]byte(providerEventHMACDomain))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(provider))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(eventID)
	return mac.Sum(nil)
}

func (s *SubscriptionStore) ActivateSpaceProProviderEvent(ctx context.Context, provider string, eventID []byte, terminalOutcomeClass, keyVersion string, hmacKey []byte, spaceID, purchaserID uuid.UUID, details json.RawMessage) (*SpaceSubscriptionRow, bool, error) {
	return s.ActivateSpaceProProviderEventWithKeys(ctx, provider, eventID, terminalOutcomeClass, []ProviderEventHMACKey{{Version: keyVersion, Key: hmacKey}}, spaceID, purchaserID, details)
}

func (s *SubscriptionStore) ActivateSpaceProProviderEventWithKeys(ctx context.Context, provider string, eventID []byte, terminalOutcomeClass string, keys []ProviderEventHMACKey, spaceID, purchaserID uuid.UUID, details json.RawMessage) (*SpaceSubscriptionRow, bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	replayed, err := s.RecordProviderEventWithKeysTx(ctx, tx, provider, eventID, terminalOutcomeClass, keys)
	if err != nil {
		return nil, false, err
	}
	if replayed {
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}
	if err := lockMutableSpaceLifecycleTx(ctx, tx, spaceID); err != nil {
		return nil, false, err
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	eventIDText := string(eventID)
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}
	if inserted, err := insertBillingEventTx(ctx, tx, nil, nil, "subscription.activated", provider, eventIDText, details); err != nil {
		return nil, false, err
	} else if !inserted {
		return nil, false, ErrDuplicateBillingEvent
	}
	if _, err := tx.Exec(ctx, `DELETE FROM billing_events WHERE space_subscription_id IN (SELECT id FROM space_subscriptions WHERE space_id=$1)`, spaceID); err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM space_subscriptions WHERE space_id=$1`, spaceID); err != nil {
		return nil, false, err
	}
	now := time.Now().UTC()
	subscriptionID := uuid.New()
	if _, err := tx.Exec(ctx, `
INSERT INTO space_subscriptions(
 id,space_id,purchaser_account_id,plan,billing_period,status,provider,provider_subscription_id,
 current_period_start,current_period_end)
VALUES($1,$2,$3,'space_pro','monthly','active',$4,$5,$6,$7)`,
		subscriptionID, spaceID, purchaserID, provider, provider+"_space_"+eventIDText, now, now.AddDate(0, 1, 0)); err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE billing_events SET space_subscription_id=$1 WHERE provider=$2 AND provider_event_id=$3`, subscriptionID, provider, eventIDText); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	row, err := s.GetSpaceSubscriptionBySpaceID(ctx, spaceID)
	return row, false, err
}

func (s *SubscriptionStore) ApplySpaceLifecycleFence(ctx context.Context, input LifecycleFenceInput) (*LifecycleFenceRecord, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockSpaceLifecycleTx(ctx, tx, input.SpaceID, true); err != nil {
		return nil, err
	}
	current, err := loadLifecycleFenceTx(ctx, tx, input.SpaceID, true)
	if err != nil {
		return nil, err
	}
	var savedHash, savedRequest, savedReceipt []byte
	err = tx.QueryRow(ctx, `
SELECT request_sha256,request_bytes,receipt_bytes
FROM subscription_space_lifecycle_operations
WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND operation_kind='FENCE'`,
		input.SpaceID, input.DeletionOperationID, input.Generation).Scan(&savedHash, &savedRequest, &savedReceipt)
	if err == nil {
		if !bytes.Equal(savedHash, input.RequestSHA256) || savedRequest == nil || !bytes.Equal(savedRequest, input.RequestBytes) {
			return nil, ErrLifecycleBinding
		}
		if len(savedReceipt) > 0 {
			current.ReceiptBytes = savedReceipt
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return current, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if current != nil {
		if input.Generation < current.Generation {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return current, nil
		}
		if input.Generation == current.Generation {
			return nil, ErrLifecycleBinding
		}
		if current.State == "PURGE_DECIDED" || current.State == "PURGED" {
			return nil, ErrLifecycleState
		}
		if input.Generation != current.Generation+1 {
			return nil, ErrLifecycleGeneration
		}
	}
	if err := enableEvidenceMutation(ctx, tx); err != nil {
		return nil, err
	}
	if current == nil {
		_, err = tx.Exec(ctx, `
INSERT INTO subscription_space_lifecycle_fences(
 space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,
 request_sha256,receipt_id,applied_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, input.SpaceID, input.DeletionOperationID, input.Generation,
			input.State, input.ManifestID, input.ManifestSHA256, input.ManifestItemCount, input.RequestSHA256, input.ReceiptID, input.AppliedAt)
	} else {
		_, err = tx.Exec(ctx, `
UPDATE subscription_space_lifecycle_fences SET
 deletion_operation_id=$2,generation=$3,state=$4,manifest_id=$5,manifest_sha256=$6,
 manifest_item_count=$7,request_sha256=$8,receipt_id=$9,applied_at=$10
WHERE space_id=$1`, input.SpaceID, input.DeletionOperationID, input.Generation, input.State, input.ManifestID,
			input.ManifestSHA256, input.ManifestItemCount, input.RequestSHA256, input.ReceiptID, input.AppliedAt)
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO subscription_space_lifecycle_operations(
 space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,
 receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_attempts)
VALUES($1,$2,$3,'FENCE',$4,$5,$6,$7,'COMPLETED',$8,$8::timestamptz+interval '30 days',0)`, input.SpaceID,
		input.DeletionOperationID, input.Generation, input.RequestSHA256, input.RequestBytes, input.ReceiptID, input.ReceiptBytes, input.AppliedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &LifecycleFenceRecord{
		SpaceID: input.SpaceID, DeletionOperationID: input.DeletionOperationID, Generation: input.Generation,
		State: input.State, ManifestID: input.ManifestID, ManifestSHA256: append([]byte(nil), input.ManifestSHA256...),
		ManifestItemCount: input.ManifestItemCount, RequestSHA256: append([]byte(nil), input.RequestSHA256...),
		ReceiptID: input.ReceiptID, AppliedAt: input.AppliedAt, ReceiptBytes: append([]byte(nil), input.ReceiptBytes...),
	}, nil
}

func (s *SubscriptionStore) PurgeSpace(ctx context.Context, input PurgeInput, cancel RenewalCanceller) ([]byte, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lockSpaceLifecycleTx(ctx, tx, input.SpaceID, true); err != nil {
		return nil, err
	}
	fence, err := loadLifecycleFenceTx(ctx, tx, input.SpaceID, true)
	if err != nil {
		return nil, err
	}
	if fence == nil || fence.Generation != input.Generation || fence.DeletionOperationID != input.DeletionOperationID || fence.ManifestID != input.ManifestID || !bytes.Equal(fence.ManifestSHA256, input.ManifestSHA256) || fence.ManifestItemCount != input.ManifestItemCount {
		return nil, ErrLifecycleBinding
	}
	if fence.State != "PURGE_DECIDED" && fence.State != "PURGED" {
		return nil, ErrLifecycleState
	}
	var savedHash, savedRequest, savedReceipt []byte
	var attempts int
	var terminal string
	err = tx.QueryRow(ctx, `
SELECT request_sha256,request_bytes,receipt_bytes,provider_cancel_attempts,terminal_state
FROM subscription_space_lifecycle_operations
WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND operation_kind='PURGE'`,
		input.SpaceID, input.DeletionOperationID, input.Generation).Scan(&savedHash, &savedRequest, &savedReceipt, &attempts, &terminal)
	if err == nil {
		if !bytes.Equal(savedHash, input.RequestSHA256) || savedRequest == nil || !bytes.Equal(savedRequest, input.RequestBytes) {
			return nil, ErrLifecycleBinding
		}
		if terminal == "COMPLETED" {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return savedReceipt, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var provider, providerSubscriptionID string
	err = tx.QueryRow(ctx, `
SELECT provider,provider_subscription_id
FROM space_subscriptions WHERE space_id=$1
ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, input.SpaceID).Scan(&provider, &providerSubscriptionID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	idempotencyKey := "space-purge:" + input.DeletionOperationID.String()
	if !errors.Is(err, pgx.ErrNoRows) {
		if cancel == nil {
			return nil, ErrProviderCancellation
		}
		if cancelErr := cancel(ctx, provider, providerSubscriptionID, idempotencyKey); cancelErr != nil {
			if err := enableEvidenceMutation(ctx, tx); err != nil {
				return nil, err
			}
			next := time.Now().UTC().Add(time.Minute)
			if attempts == 0 {
				_, err = tx.Exec(ctx, `
INSERT INTO subscription_space_lifecycle_operations(
 space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,
 terminal_state,provider_cancel_state,provider_cancel_attempts,provider_cancel_idempotency_key,
 provider_cancel_last_error,provider_cancel_next_attempt_at)
VALUES($1,$2,$3,'PURGE',$4,$5,'PENDING','RETRYABLE',1,$6,$7,$8)`, input.SpaceID,
					input.DeletionOperationID, input.Generation, input.RequestSHA256, input.RequestBytes, idempotencyKey, cancelErr.Error(), next)
			} else {
				_, err = tx.Exec(ctx, `
UPDATE subscription_space_lifecycle_operations SET
 provider_cancel_attempts=provider_cancel_attempts+1,provider_cancel_last_error=$5,
 provider_cancel_next_attempt_at=$6
WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND operation_kind='PURGE'
  AND request_sha256=$4`,
					input.SpaceID, input.DeletionOperationID, input.Generation, input.RequestSHA256, cancelErr.Error(), next)
			}
			if err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("%w: %v", ErrProviderCancellation, cancelErr)
		}
	}
	if err := enableEvidenceMutation(ctx, tx); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM billing_events WHERE space_subscription_id IN (SELECT id FROM space_subscriptions WHERE space_id=$1)`, input.SpaceID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM space_subscriptions WHERE space_id=$1`, input.SpaceID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE subscription_space_lifecycle_fences SET state='PURGED',request_sha256=$2,receipt_id=$3,applied_at=$4
WHERE space_id=$1`, input.SpaceID, input.RequestSHA256, input.ReceiptID, input.CompletedAt); err != nil {
		return nil, err
	}
	if attempts == 0 {
		_, err = tx.Exec(ctx, `
INSERT INTO subscription_space_lifecycle_operations(
 space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,receipt_id,
 receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_state,provider_cancel_attempts,
 provider_cancel_idempotency_key)
VALUES($1,$2,$3,'PURGE',$4,$5,$6,$7,'COMPLETED',$8,$8::timestamptz+interval '30 days','COMPLETED',1,$9)`,
			input.SpaceID, input.DeletionOperationID, input.Generation, input.RequestSHA256, input.RequestBytes,
			input.ReceiptID, input.ReceiptBytes, input.CompletedAt, idempotencyKey)
	} else {
		_, err = tx.Exec(ctx, `
UPDATE subscription_space_lifecycle_operations SET
 receipt_id=$5,receipt_bytes=$6,terminal_state='COMPLETED',completed_at=$7,retain_until=$7::timestamptz+interval '30 days',
 provider_cancel_state='COMPLETED',provider_cancel_attempts=provider_cancel_attempts+1,
 provider_cancel_last_error=NULL,provider_cancel_next_attempt_at=NULL
WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 AND operation_kind='PURGE' AND request_sha256=$4`,
			input.SpaceID, input.DeletionOperationID, input.Generation, input.RequestSHA256, input.ReceiptID, input.ReceiptBytes, input.CompletedAt)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return input.ReceiptBytes, nil
}

func (s *SubscriptionStore) CleanupSpaceLifecycleEvidence(ctx context.Context, now time.Time) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := enableEvidenceMutation(ctx, tx); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `
UPDATE subscription_space_lifecycle_operations
SET request_bytes=NULL,receipt_bytes=NULL
WHERE terminal_state='COMPLETED' AND retain_until <= $1
  AND (request_bytes IS NOT NULL OR receipt_bytes IS NOT NULL)`, now.UTC())
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func loadLifecycleFenceTx(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID, forUpdate bool) (*LifecycleFenceRecord, error) {
	query := `
SELECT space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,
 request_sha256,receipt_id,applied_at
FROM subscription_space_lifecycle_fences WHERE space_id=$1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var record LifecycleFenceRecord
	err := tx.QueryRow(ctx, query, spaceID).Scan(&record.SpaceID, &record.DeletionOperationID, &record.Generation,
		&record.State, &record.ManifestID, &record.ManifestSHA256, &record.ManifestItemCount, &record.RequestSHA256,
		&record.ReceiptID, &record.AppliedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &record, err
}

func lockMutableSpaceLifecycleTx(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) error {
	if err := lockSpaceLifecycleTx(ctx, tx, spaceID, true); err != nil {
		return err
	}
	var state string
	err := tx.QueryRow(ctx, `SELECT state FROM subscription_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if state != "LIVE" {
		return ErrLifecycleState
	}
	return nil
}

func lockReadableSpaceLifecycleTx(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) (bool, error) {
	if err := lockSpaceLifecycleTx(ctx, tx, spaceID, false); err != nil {
		return false, err
	}
	var state string
	err := tx.QueryRow(ctx, `SELECT state FROM subscription_space_lifecycle_fences WHERE space_id=$1 FOR SHARE`, spaceID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return state == "LIVE", nil
}

func lockSpaceLifecycleTx(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID, exclusive bool) error {
	const advisoryNamespace int64 = 0x5355425343524950
	query := `SELECT pg_advisory_xact_lock_shared(hashtextextended($1::text,$2::bigint))`
	if exclusive {
		query = `SELECT pg_advisory_xact_lock(hashtextextended($1::text,$2::bigint))`
	}
	_, err := tx.Exec(ctx, query, spaceID.String(), advisoryNamespace)
	return err
}

func enableEvidenceMutation(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT set_config('voice.subscription_evidence_write','on',true)`)
	return err
}
