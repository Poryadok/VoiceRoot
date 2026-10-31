package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrSessionConsentDenied   = errors.New("session consent denied")
	ErrSessionConsentConflict = errors.New("session consent conflicts with prior receipt")
)

type SessionConsentReceipt struct {
	ReceiptID      uuid.UUID `json:"receipt_id"`
	OperationID    uuid.UUID `json:"operation_id"`
	ApplicationID  uuid.UUID `json:"application_id"`
	EnvironmentID  uuid.UUID `json:"environment_id"`
	SessionID      uuid.UUID `json:"session_id"`
	AccountID      uuid.UUID `json:"account_id"`
	ProfileID      uuid.UUID `json:"profile_id"`
	RosterRevision int64     `json:"roster_revision"`
	DestinationKey string    `json:"destination_key"`
	PolicyRevision int64     `json:"policy_revision"`
}

// RecordSessionConsent stores a participant's explicit keep-group decision.
// The caller identity is supplied only from the verified Auth token; destination
// and policy revisions are resolved from current GIS-owned state.
func (s *Store) RecordSessionConsent(ctx context.Context, accountID, profileID, sessionID, operationID uuid.UUID, rosterRevision int64) (SessionConsentReceipt, error) {
	if s == nil || s.Pool == nil || accountID == uuid.Nil || profileID == uuid.Nil || sessionID == uuid.Nil || operationID == uuid.Nil || rosterRevision <= 0 {
		return SessionConsentReceipt{}, ErrSessionConsentDenied
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return SessionConsentReceipt{}, fmt.Errorf("begin session consent: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var applicationID, environmentID uuid.UUID
	var kind, externalKey string
	var parentPartyKey *string
	var currentRevision int64
	var complete, live bool
	var members []uuid.UUID
	err = tx.QueryRow(ctx, `
SELECT application_id,environment_id,kind,external_key,parent_party_key,roster_revision,
       roster_complete,members,(session_status='active' AND lease_expires_at > clock_timestamp())
FROM gis_sessions WHERE id=$1 FOR UPDATE`, sessionID).Scan(
		&applicationID, &environmentID, &kind, &externalKey, &parentPartyKey, &currentRevision, &complete, &members, &live)
	if err != nil || !live || !complete || currentRevision != rosterRevision || !slices.Contains(members, profileID) {
		return SessionConsentReceipt{}, ErrSessionConsentDenied
	}

	var activeBinding bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (
SELECT 1 FROM player_bindings
WHERE application_id=$1 AND environment_id=$2 AND account_id=$3 AND profile_id=$4 AND status='active'
)`, applicationID, environmentID, accountID, profileID).Scan(&activeBinding)
	if err != nil || !activeBinding {
		return SessionConsentReceipt{}, ErrSessionConsentDenied
	}

	var policyRevision int64
	err = tx.QueryRow(ctx, `SELECT e.revision FROM environments e JOIN applications a ON a.id=e.application_id AND a.status IN ('sandbox','active') WHERE e.id=$1 AND e.application_id=$2 AND e.status='active'`, environmentID, applicationID).Scan(&policyRevision)
	if err != nil || policyRevision <= 0 {
		return SessionConsentReceipt{}, ErrSessionConsentDenied
	}
	destination := "session/" + sessionID.String()
	if kind == "party" {
		destination = "party/" + externalKey
	} else if parentPartyKey != nil && *parentPartyKey != "" {
		destination = "party/" + *parentPartyKey
	}

	requestBytes, err := json.Marshal(struct {
		Version        int       `json:"version"`
		AccountID      uuid.UUID `json:"account_id"`
		ProfileID      uuid.UUID `json:"profile_id"`
		SessionID      uuid.UUID `json:"session_id"`
		RosterRevision int64     `json:"roster_revision"`
	}{1, accountID, profileID, sessionID, rosterRevision})
	if err != nil {
		return SessionConsentReceipt{}, err
	}
	requestHash := sha256.Sum256(requestBytes)

	var storedHash, receiptBytes []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,receipt_bytes FROM gis_session_consent_receipts WHERE operation_id=$1`, operationID).Scan(&storedHash, &receiptBytes)
	if err == nil {
		if !equalBytes(storedHash, requestHash[:]) {
			return SessionConsentReceipt{}, ErrSessionConsentConflict
		}
		var receipt SessionConsentReceipt
		if json.Unmarshal(receiptBytes, &receipt) != nil {
			return SessionConsentReceipt{}, errors.New("stored consent receipt is invalid")
		}
		if err := tx.Commit(ctx); err != nil {
			return SessionConsentReceipt{}, fmt.Errorf("commit consent replay: %w", err)
		}
		return receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SessionConsentReceipt{}, fmt.Errorf("read consent receipt: %w", err)
	}

	var alreadyConsented bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM gis_session_consent_receipts WHERE session_id=$1 AND roster_revision=$2 AND profile_id=$3)`, sessionID, rosterRevision, profileID).Scan(&alreadyConsented); err != nil {
		return SessionConsentReceipt{}, fmt.Errorf("check existing consent: %w", err)
	}
	if alreadyConsented {
		return SessionConsentReceipt{}, ErrSessionConsentConflict
	}
	receipt := SessionConsentReceipt{
		ReceiptID: uuid.New(), OperationID: operationID, ApplicationID: applicationID, EnvironmentID: environmentID,
		SessionID: sessionID, AccountID: accountID, ProfileID: profileID, RosterRevision: rosterRevision,
		DestinationKey: destination, PolicyRevision: policyRevision,
	}
	receiptBytes, err = json.Marshal(receipt)
	if err != nil {
		return SessionConsentReceipt{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_session_consent_receipts
(operation_id,receipt_id,request_hash,application_id,environment_id,session_id,account_id,profile_id,roster_revision,destination_key,policy_revision,receipt_bytes)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, operationID, receipt.ReceiptID, requestHash[:], applicationID, environmentID,
		sessionID, accountID, profileID, rosterRevision, destination, policyRevision, receiptBytes)
	if err != nil {
		return SessionConsentReceipt{}, fmt.Errorf("insert consent receipt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return SessionConsentReceipt{}, fmt.Errorf("commit consent receipt: %w", err)
	}
	return receipt, nil
}

// CurrentSessionConsentProfiles returns only currently eligible explicit
// consents. A roster, binding, lease, or environment-policy change invalidates
// the old receipt for destination writes without deleting its audit record.
func (s *Store) CurrentSessionConsentProfiles(ctx context.Context, sessionID uuid.UUID, rosterRevision int64) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil || sessionID == uuid.Nil || rosterRevision <= 0 {
		return nil, ErrSessionConsentDenied
	}
	rows, err := s.Pool.Query(ctx, `
SELECT c.profile_id
FROM gis_session_consent_receipts c
JOIN gis_sessions s ON s.id=c.session_id
JOIN environments e ON e.id=s.environment_id AND e.application_id=s.application_id AND e.status='active'
JOIN applications a ON a.id=s.application_id AND a.status IN ('sandbox','active')
JOIN player_bindings b ON b.application_id=s.application_id AND b.environment_id=s.environment_id
  AND b.account_id=c.account_id AND b.profile_id=c.profile_id AND b.status='active'
WHERE s.id=$1 AND s.session_status='active' AND s.roster_complete
  AND s.roster_revision=$2 AND s.lease_expires_at > clock_timestamp()
  AND c.roster_revision=s.roster_revision AND c.policy_revision=e.revision
  AND c.application_id=s.application_id AND c.environment_id=s.environment_id
  AND s.members @> ARRAY[c.profile_id]::uuid[]
ORDER BY c.profile_id`, sessionID, rosterRevision)
	if err != nil {
		return nil, fmt.Errorf("resolve current session consents: %w", err)
	}
	defer rows.Close()
	profiles := make([]uuid.UUID, 0)
	for rows.Next() {
		var profileID uuid.UUID
		if err := rows.Scan(&profileID); err != nil {
			return nil, err
		}
		profiles = append(profiles, profileID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return profiles, nil
}
