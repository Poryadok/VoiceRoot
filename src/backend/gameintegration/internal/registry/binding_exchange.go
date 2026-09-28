package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var bindingExchangeHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var ErrBindingExchangeConflict = errors.New("binding exchange operation conflict")

type BindingExchangeOperation struct {
	Challenge     BindingChallenge
	OperationID   uuid.UUID
	RequestSHA256 string
	Status        string
	HandoffJWS    string
	ClaimID       uuid.UUID
	AssertionJTI  uuid.UUID
	BindingID     uuid.UUID
	Result        json.RawMessage
}

type BindingExchangeResult struct {
	OperationID uuid.UUID `json:"operation_id"`
	BindingID   uuid.UUID `json:"binding_id"`
	Status      string    `json:"status"`
}

type BindingExchangeCompletion struct {
	OperationID  uuid.UUID
	ClaimID      uuid.UUID
	AssertionJTI uuid.UUID
	BindingID    uuid.UUID
	Outcome      string
}

// BeginPlayerBindingExchange durably pins the stable GIS-owned operation to
// the exact caller body hash before any Auth exchange or claim call.
func (s *Store) BeginPlayerBindingExchange(ctx context.Context, challengeID uuid.UUID,
	requestSHA256 string) (BindingExchangeOperation, error) {
	if challengeID == uuid.Nil || !bindingExchangeHashPattern.MatchString(requestSHA256) {
		return BindingExchangeOperation{}, ErrInvalidBindingChallenge
	}
	if s == nil || s.Pool == nil {
		return BindingExchangeOperation{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BindingExchangeOperation{}, fmt.Errorf("begin binding exchange operation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var saved BindingExchangeOperation
	var savedChallenge, savedOperation uuid.UUID
	var savedClaim, savedJTI, savedBinding *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT operation_id,challenge_id,request_sha256,status,COALESCE(handoff_jws,''),
		auth_claim_id,assertion_jti,binding_id,result FROM player_binding_exchange_operations
		WHERE challenge_id=$1 FOR UPDATE`, challengeID).Scan(&savedOperation, &savedChallenge, &saved.RequestSHA256,
		&saved.Status, &saved.HandoffJWS, &savedClaim, &savedJTI, &savedBinding, &saved.Result)
	if savedClaim != nil {
		saved.ClaimID = *savedClaim
	}
	if savedJTI != nil {
		saved.AssertionJTI = *savedJTI
	}
	if savedBinding != nil {
		saved.BindingID = *savedBinding
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return BindingExchangeOperation{}, fmt.Errorf("read binding exchange operation: %w", err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		challenge, loadErr := loadBindingChallenge(ctx, tx, challengeID, true)
		if loadErr != nil || challenge.Status != "pending" || !challenge.ExpiresAt.After(s.now()) {
			if loadErr != nil {
				return BindingExchangeOperation{}, loadErr
			}
			return BindingExchangeOperation{}, ErrBindingChallengeNotFound
		}
		// Another exchange may have committed while this transaction waited for
		// the challenge lock. Re-read under that lock so exact retries converge.
		err = tx.QueryRow(ctx, `SELECT operation_id,challenge_id,request_sha256,status,COALESCE(handoff_jws,''),
			auth_claim_id,assertion_jti,binding_id,result FROM player_binding_exchange_operations
			WHERE challenge_id=$1`, challengeID).Scan(&savedOperation, &savedChallenge, &saved.RequestSHA256,
			&saved.Status, &saved.HandoffJWS, &savedClaim, &savedJTI, &savedBinding, &saved.Result)
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `INSERT INTO player_binding_exchange_operations
				(operation_id,challenge_id,request_sha256,status) VALUES ($1,$2,$3,'pending')`,
				challenge.OperationID, challengeID, requestSHA256)
			if err != nil {
				return BindingExchangeOperation{}, fmt.Errorf("persist binding exchange operation: %w", err)
			}
			savedOperation, savedChallenge, saved.RequestSHA256, saved.Status = challenge.OperationID, challengeID,
				requestSHA256, "pending"
		} else if err != nil {
			return BindingExchangeOperation{}, fmt.Errorf("re-read binding exchange operation: %w", err)
		}
	}
	if savedClaim != nil {
		saved.ClaimID = *savedClaim
	}
	if savedJTI != nil {
		saved.AssertionJTI = *savedJTI
	}
	if savedBinding != nil {
		saved.BindingID = *savedBinding
	}
	if savedChallenge != challengeID || savedOperation == uuid.Nil || saved.RequestSHA256 != requestSHA256 {
		return BindingExchangeOperation{}, ErrBindingExchangeConflict
	}
	challenge, err := loadBindingChallenge(ctx, tx, challengeID, false)
	if err != nil {
		return BindingExchangeOperation{}, err
	}
	if challenge.OperationID != savedOperation {
		return BindingExchangeOperation{}, ErrBindingExchangeConflict
	}
	saved.Challenge, saved.OperationID = challenge, savedOperation
	if err := tx.Commit(ctx); err != nil {
		return BindingExchangeOperation{}, fmt.Errorf("commit binding exchange operation: %w", err)
	}
	return saved, nil
}

// RecordPlayerBindingExchangeClaim persists the authenticated Auth receipt
// and exact signed handoff before GIS attempts its binding transaction.
func (s *Store) RecordPlayerBindingExchangeClaim(ctx context.Context, operationID uuid.UUID,
	requestSHA256, handoffJWS string, claimID, assertionJTI uuid.UUID) error {
	if operationID == uuid.Nil || !bindingExchangeHashPattern.MatchString(requestSHA256) || handoffJWS == "" ||
		len(handoffJWS) > 16<<10 || claimID == uuid.Nil || assertionJTI == uuid.Nil {
		return ErrInvalidBindingChallenge
	}
	if s == nil || s.Pool == nil {
		return ErrRegistryUnavailable
	}
	command, err := s.Pool.Exec(ctx, `UPDATE player_binding_exchange_operations SET status='claimed', handoff_jws=$3,
		auth_claim_id=$4, assertion_jti=$5, updated_at=clock_timestamp()
		WHERE operation_id=$1 AND request_sha256=$2 AND status='pending'`,
		operationID, requestSHA256, handoffJWS, claimID, assertionJTI)
	if err != nil {
		return fmt.Errorf("record Auth binding claim: %w", err)
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	var savedHash, savedJWS string
	var savedClaim, savedJTI *uuid.UUID
	var status string
	err = s.Pool.QueryRow(ctx, `SELECT request_sha256,status,handoff_jws,auth_claim_id,assertion_jti
		FROM player_binding_exchange_operations WHERE operation_id=$1`, operationID).
		Scan(&savedHash, &status, &savedJWS, &savedClaim, &savedJTI)
	if err != nil {
		return fmt.Errorf("read Auth binding claim receipt: %w", err)
	}
	if savedHash != requestSHA256 || savedJWS != handoffJWS || savedClaim == nil || savedJTI == nil ||
		*savedClaim != claimID || *savedJTI != assertionJTI ||
		(status != "claimed" && status != "created" && status != "completed" && status != "failed") {
		return ErrBindingExchangeConflict
	}
	return nil
}

// CommitPlayerBindingExchange atomically creates a pending GIS binding,
// consumes the challenge and adds the Auth completion to the durable outbox.
func (s *Store) CommitPlayerBindingExchange(ctx context.Context, operationID uuid.UUID, requestSHA256 string,
	claimID, assertionJTI uuid.UUID, input CreatePlayerBindingInput) (BindingExchangeResult, error) {
	if operationID == uuid.Nil || !bindingExchangeHashPattern.MatchString(requestSHA256) || claimID == uuid.Nil ||
		assertionJTI == uuid.Nil || input.ApplicationID == uuid.Nil || input.EnvironmentID == uuid.Nil ||
		input.AccountID == uuid.Nil || input.ActorID == uuid.Nil || input.ProfileID == uuid.Nil || input.DeviceID == uuid.Nil ||
		!bindingProviderPattern.MatchString(input.Provider) || !bindingSubjectDigestPattern.MatchString(input.ProviderSubjectDigest) {
		return BindingExchangeResult{}, ErrInvalidPlayerBinding
	}
	if s == nil || s.Pool == nil {
		return BindingExchangeResult{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BindingExchangeResult{}, fmt.Errorf("begin GIS binding creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var challengeID, savedClaim, savedJTI, bindingID uuid.UUID
	var savedHash, status string
	err = tx.QueryRow(ctx, `SELECT challenge_id,request_sha256,status,auth_claim_id,assertion_jti,binding_id
		FROM player_binding_exchange_operations WHERE operation_id=$1 FOR UPDATE`, operationID).
		Scan(&challengeID, &savedHash, &status, &savedClaim, &savedJTI, &bindingID)
	if err != nil {
		return BindingExchangeResult{}, fmt.Errorf("lock GIS binding operation: %w", err)
	}
	if savedHash != requestSHA256 || savedClaim != claimID || savedJTI != assertionJTI {
		return BindingExchangeResult{}, ErrBindingExchangeConflict
	}
	if status == "completed" || status == "failed" {
		var result BindingExchangeResult
		var resultJSON []byte
		if err := tx.QueryRow(ctx, `SELECT result FROM player_binding_exchange_operations WHERE operation_id=$1`,
			operationID).Scan(&resultJSON); err != nil {
			return BindingExchangeResult{}, fmt.Errorf("read terminal GIS binding result: %w", err)
		}
		if err := json.Unmarshal(resultJSON, &result); err != nil {
			return BindingExchangeResult{}, fmt.Errorf("decode terminal GIS binding result: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return BindingExchangeResult{}, fmt.Errorf("commit GIS binding retry: %w", err)
		}
		return result, nil
	}
	if status == "failed" {
		var result BindingExchangeResult
		var resultJSON []byte
		if err := tx.QueryRow(ctx, `SELECT result FROM player_binding_exchange_operations WHERE operation_id=$1`,
			operationID).Scan(&resultJSON); err != nil {
			return BindingExchangeResult{}, fmt.Errorf("read failed GIS binding result: %w", err)
		}
		if err := json.Unmarshal(resultJSON, &result); err != nil {
			return BindingExchangeResult{}, fmt.Errorf("decode failed GIS binding result: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return BindingExchangeResult{}, fmt.Errorf("commit GIS binding failure retry: %w", err)
		}
		return result, nil
	}
	if status == "created" {
		if err := tx.Commit(ctx); err != nil {
			return BindingExchangeResult{}, fmt.Errorf("commit GIS binding created retry: %w", err)
		}
		return BindingExchangeResult{OperationID: operationID, BindingID: bindingID, Status: "pending"}, nil
	}
	if status != "claimed" {
		return BindingExchangeResult{}, ErrBindingExchangeConflict
	}
	challenge, err := loadBindingChallenge(ctx, tx, challengeID, true)
	if err != nil {
		return BindingExchangeResult{}, err
	}
	if challenge.Status != "pending" || !challenge.ExpiresAt.After(s.now()) || challenge.OperationID != operationID ||
		challenge.ApplicationID != input.ApplicationID || challenge.EnvironmentID != input.EnvironmentID {
		return BindingExchangeResult{}, ErrBindingExchangeConflict
	}
	var appStatus, environmentStatus string
	err = tx.QueryRow(ctx, `SELECT a.status,e.status FROM applications a JOIN environments e ON e.application_id=a.id
		WHERE a.id=$1 AND e.id=$2 FOR SHARE OF a,e`, input.ApplicationID, input.EnvironmentID).
		Scan(&appStatus, &environmentStatus)
	if err != nil || (appStatus != "sandbox" && appStatus != "active") || environmentStatus != "active" {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return BindingExchangeResult{}, fmt.Errorf("read GIS binding app scope: %w", err)
		}
		return BindingExchangeResult{}, ErrInvalidPlayerBinding
	}
	bindingID = uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO player_bindings
		(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,
		 status,authority_revision,permitted_character_context)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending',1,'[]'::jsonb)`, bindingID, input.ApplicationID,
		input.EnvironmentID, input.Provider, input.ProviderSubjectDigest, input.AccountID, input.ActorID,
		input.ProfileID, input.DeviceID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return BindingExchangeResult{}, ErrInvalidPlayerBinding
		}
		return BindingExchangeResult{}, fmt.Errorf("persist pending GIS binding: %w", err)
	}
	command, err := tx.Exec(ctx, `UPDATE player_binding_challenges SET status='consumed',consumed_at=clock_timestamp()
		WHERE challenge_id=$1 AND operation_id=$2 AND status='pending' AND expires_at>clock_timestamp()`, challengeID, operationID)
	if err != nil {
		return BindingExchangeResult{}, fmt.Errorf("consume GIS binding challenge: %w", err)
	}
	if command.RowsAffected() != 1 {
		return BindingExchangeResult{}, ErrBindingExchangeConflict
	}
	command, err = tx.Exec(ctx, `UPDATE player_binding_exchange_operations SET status='created',binding_id=$2,
		updated_at=clock_timestamp() WHERE operation_id=$1 AND status='claimed'`, operationID, bindingID)
	if err != nil {
		return BindingExchangeResult{}, fmt.Errorf("record GIS binding creation: %w", err)
	}
	if command.RowsAffected() != 1 {
		return BindingExchangeResult{}, ErrBindingExchangeConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO player_binding_completion_outbox
		(operation_id,claim_id,assertion_jti,binding_id,outcome) VALUES ($1,$2,$3,$4,'succeeded')`,
		operationID, claimID, assertionJTI, bindingID)
	if err != nil {
		return BindingExchangeResult{}, fmt.Errorf("enqueue Auth binding completion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return BindingExchangeResult{}, fmt.Errorf("commit GIS binding creation and completion outbox: %w", err)
	}
	return BindingExchangeResult{OperationID: operationID, BindingID: bindingID, Status: "pending"}, nil
}

func (s *Store) NextPlayerBindingCompletion(ctx context.Context) (BindingExchangeCompletion, error) {
	if s == nil || s.Pool == nil {
		return BindingExchangeCompletion{}, ErrRegistryUnavailable
	}
	var result BindingExchangeCompletion
	var bindingID *uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT operation_id,claim_id,assertion_jti,binding_id,outcome
		FROM player_binding_completion_outbox WHERE completed_at IS NULL AND next_attempt_at<=clock_timestamp()
		ORDER BY next_attempt_at,created_at LIMIT 1`,
	).Scan(&result.OperationID, &result.ClaimID, &result.AssertionJTI, &bindingID, &result.Outcome)
	if errors.Is(err, pgx.ErrNoRows) {
		return BindingExchangeCompletion{}, pgx.ErrNoRows
	}
	if err != nil {
		return BindingExchangeCompletion{}, fmt.Errorf("load GIS binding completion outbox: %w", err)
	}
	if bindingID != nil {
		result.BindingID = *bindingID
	}
	return result, nil
}

func (s *Store) RecordPlayerBindingCompletionAttempt(ctx context.Context, operationID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return ErrRegistryUnavailable
	}
	command, err := s.Pool.Exec(ctx, `UPDATE player_binding_completion_outbox SET attempt_count=attempt_count+1,
		last_attempt_at=clock_timestamp(),next_attempt_at=clock_timestamp()+interval '1 second'
		WHERE operation_id=$1 AND completed_at IS NULL`, operationID)
	if err != nil {
		return fmt.Errorf("record GIS binding completion attempt: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrBindingExchangeConflict
	}
	return nil
}

// FailPlayerBindingExchange settles a deterministic denial after Auth already
// accepted the handoff claim. Transient database errors must not call this.
func (s *Store) FailPlayerBindingExchange(ctx context.Context, operationID uuid.UUID, requestSHA256 string,
	claimID, assertionJTI uuid.UUID) error {
	if s == nil || s.Pool == nil || operationID == uuid.Nil || !bindingExchangeHashPattern.MatchString(requestSHA256) ||
		claimID == uuid.Nil || assertionJTI == uuid.Nil {
		return ErrInvalidPlayerBinding
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin GIS binding failure receipt: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var challengeID, savedClaim, savedJTI uuid.UUID
	var savedHash, status string
	err = tx.QueryRow(ctx, `SELECT challenge_id,request_sha256,status,auth_claim_id,assertion_jti
		FROM player_binding_exchange_operations WHERE operation_id=$1 FOR UPDATE`, operationID).
		Scan(&challengeID, &savedHash, &status, &savedClaim, &savedJTI)
	if err != nil {
		return fmt.Errorf("read GIS binding failure operation: %w", err)
	}
	if savedHash != requestSHA256 || savedClaim != claimID || savedJTI != assertionJTI {
		return ErrBindingExchangeConflict
	}
	if status == "failed" {
		return tx.Commit(ctx)
	}
	if status != "claimed" {
		return ErrBindingExchangeConflict
	}
	command, err := tx.Exec(ctx, `UPDATE player_binding_challenges SET status='consumed',consumed_at=clock_timestamp()
		WHERE challenge_id=$1 AND operation_id=$2 AND status='pending'`, challengeID, operationID)
	if err != nil {
		return fmt.Errorf("consume denied GIS binding challenge: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrBindingExchangeConflict
	}
	result, err := json.Marshal(BindingExchangeResult{OperationID: operationID, Status: "failed"})
	if err != nil {
		return fmt.Errorf("encode denied GIS binding result: %w", err)
	}
	command, err = tx.Exec(ctx, `UPDATE player_binding_exchange_operations SET status='failed',result=$2,
		updated_at=clock_timestamp() WHERE operation_id=$1 AND status='claimed'`, operationID, result)
	if err != nil {
		return fmt.Errorf("persist denied GIS binding result: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrBindingExchangeConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO player_binding_completion_outbox
		(operation_id,claim_id,assertion_jti,outcome) VALUES ($1,$2,$3,'failed')`, operationID, claimID, assertionJTI)
	if err != nil {
		return fmt.Errorf("enqueue GIS binding failure completion: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Store) CompleteFailedPlayerBindingExchange(ctx context.Context, operationID, claimID, assertionJTI uuid.UUID) error {
	if s == nil || s.Pool == nil || operationID == uuid.Nil || claimID == uuid.Nil || assertionJTI == uuid.Nil {
		return ErrInvalidPlayerBinding
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin GIS binding failure ACK: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var savedClaim, savedJTI uuid.UUID
	var outcome string
	var bindingID *uuid.UUID
	var completedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT claim_id,assertion_jti,outcome,binding_id,completed_at
		FROM player_binding_completion_outbox WHERE operation_id=$1 FOR UPDATE`, operationID).
		Scan(&savedClaim, &savedJTI, &outcome, &bindingID, &completedAt)
	if err != nil {
		return fmt.Errorf("read GIS binding failure ACK: %w", err)
	}
	if savedClaim != claimID || savedJTI != assertionJTI || outcome != "failed" || bindingID != nil {
		return ErrBindingExchangeConflict
	}
	if completedAt != nil {
		return tx.Commit(ctx)
	}
	result, err := json.Marshal(BindingExchangeResult{OperationID: operationID, Status: "failed"})
	if err != nil {
		return fmt.Errorf("encode failed GIS binding result: %w", err)
	}
	command, err := tx.Exec(ctx, `UPDATE player_binding_exchange_operations SET status='failed',result=$2,
		updated_at=clock_timestamp() WHERE operation_id=$1 AND status='failed'`, operationID, result)
	if err != nil {
		return fmt.Errorf("persist failed GIS binding exchange ACK: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrBindingExchangeConflict
	}
	command, err = tx.Exec(ctx, `UPDATE player_binding_completion_outbox SET completed_at=clock_timestamp()
		WHERE operation_id=$1 AND completed_at IS NULL`, operationID)
	if err != nil {
		return fmt.Errorf("ACK failed GIS binding exchange: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrBindingExchangeConflict
	}
	return tx.Commit(ctx)
}

func (s *Store) CompletePlayerBindingExchange(ctx context.Context, operationID, claimID, assertionJTI, bindingID uuid.UUID) error {
	if s == nil || s.Pool == nil || operationID == uuid.Nil || claimID == uuid.Nil || assertionJTI == uuid.Nil || bindingID == uuid.Nil {
		return ErrInvalidPlayerBinding
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin GIS binding completion receipt: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var outboxClaim, outboxJTI, outboxBinding uuid.UUID
	var completedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT claim_id,assertion_jti,binding_id,completed_at FROM player_binding_completion_outbox
		WHERE operation_id=$1 FOR UPDATE`, operationID).Scan(&outboxClaim, &outboxJTI, &outboxBinding, &completedAt)
	if err != nil {
		return fmt.Errorf("read GIS binding completion receipt: %w", err)
	}
	if outboxClaim != claimID || outboxJTI != assertionJTI || outboxBinding != bindingID {
		return ErrBindingExchangeConflict
	}
	if completedAt != nil {
		return tx.Commit(ctx)
	}
	command, err := tx.Exec(ctx, `UPDATE player_bindings SET status='active',authority_revision=authority_revision+1,
		updated_at=clock_timestamp() WHERE binding_id=$1 AND status='pending'`, bindingID)
	if err != nil {
		return fmt.Errorf("activate GIS player binding: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrBindingExchangeConflict
	}
	result, err := json.Marshal(BindingExchangeResult{OperationID: operationID, BindingID: bindingID, Status: "active"})
	if err != nil {
		return fmt.Errorf("encode completed GIS binding result: %w", err)
	}
	command, err = tx.Exec(ctx, `UPDATE player_binding_exchange_operations SET status='completed',result=$2,
		updated_at=clock_timestamp() WHERE operation_id=$1 AND status='created'`, operationID, result)
	if err != nil {
		return fmt.Errorf("persist completed GIS binding exchange: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrBindingExchangeConflict
	}
	command, err = tx.Exec(ctx, `UPDATE player_binding_completion_outbox SET completed_at=clock_timestamp()
		WHERE operation_id=$1 AND completed_at IS NULL`, operationID)
	if err != nil {
		return fmt.Errorf("ack GIS binding completion outbox: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrBindingExchangeConflict
	}
	return tx.Commit(ctx)
}

func loadBindingChallenge(ctx context.Context, tx pgx.Tx, id uuid.UUID, forUpdate bool) (BindingChallenge, error) {
	query := `SELECT challenge_id,nonce,application_id,environment_id,provider,redirect_uri_sha256,pkce_challenge,
		device_key_id,device_key_thumbprint,operation_id,expires_at,status FROM player_binding_challenges WHERE challenge_id=$1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var challenge BindingChallenge
	err := tx.QueryRow(ctx, query, id).Scan(&challenge.ChallengeID, &challenge.Nonce, &challenge.ApplicationID,
		&challenge.EnvironmentID, &challenge.Provider, &challenge.RedirectURIHash, &challenge.PKCEChallenge,
		&challenge.DeviceKeyID, &challenge.DeviceKeyThumbprint, &challenge.OperationID, &challenge.ExpiresAt,
		&challenge.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return BindingChallenge{}, ErrBindingChallengeNotFound
	}
	if err != nil {
		return BindingChallenge{}, fmt.Errorf("read GIS binding exchange challenge: %w", err)
	}
	return challenge, nil
}
