package spacemedia

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/voice/internal/store"
)

var ErrAdmissionUnavailable = errors.New("Space media admission unavailable")
var ErrAdmissionConflict = errors.New("Space media admission conflicts with current state")
var ErrRoomHeadPending = errors.New("Space voice room incarnation is pending recovery")

type AdmissionState = string

const (
	AdmissionPrepared  AdmissionState = "PREPARED"
	AdmissionFenced    AdmissionState = "FENCED"
	AdmissionCommitted AdmissionState = "COMMITTED"
	AdmissionAborting  AdmissionState = "ABORTING"
)

type AdmissionEvent = store.SpaceMediaAdmissionEvent
type Admission = store.SpaceMediaAdmission

type PostgresAdmissionStore struct{ pool *pgxpool.Pool }

func NewPostgresAdmissionStore(pool *pgxpool.Pool) *PostgresAdmissionStore {
	return &PostgresAdmissionStore{pool: pool}
}

func (s *PostgresAdmissionStore) usable() bool { return s != nil && s.pool != nil }

// CheckSchema fails closed unless the durable admission journal, outbox, and
// operation-owned account-fence columns are all available. Redis projections
// must not be exposed when PostgreSQL cannot record their source of truth.
func (s *PostgresAdmissionStore) CheckSchema(ctx context.Context) error {
	if !s.usable() {
		return ErrAdmissionUnavailable
	}
	var ready bool
	err := s.pool.QueryRow(ctx, `SELECT
	to_regclass('voice_space_media_admissions') IS NOT NULL
	AND to_regclass('voice_space_media_admission_outbox') IS NOT NULL
	AND to_regclass('voice_space_media_room_heads') IS NOT NULL
	AND EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema=current_schema() AND table_name='voice_account_voice_fences' AND column_name='admission_operation_id')
	AND EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema=current_schema() AND table_name='voice_account_voice_fences' AND column_name='admission_generation')`).Scan(&ready)
	if err != nil {
		return fmt.Errorf("check Space media admission schema: %w", err)
	}
	if !ready {
		return ErrAdmissionUnavailable
	}
	return nil
}

// ClaimRoomHead serializes the first join and all later joins for one
// canonical Space voice room. An OPEN head is usable only after its creator's
// PostgreSQL admission and Redis projection are both confirmed.
func (s *PostgresAdmissionStore) ClaimRoomHead(ctx context.Context, voiceRoomID, spaceID, proposedRoomID string, operationID uuid.UUID, allowCreate bool) (store.SpaceMediaRoomHead, error) {
	if !s.usable() || voiceRoomID == "" || spaceID == "" || proposedRoomID == "" || operationID == uuid.Nil {
		return store.SpaceMediaRoomHead{}, ErrAdmissionUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return store.SpaceMediaRoomHead{}, fmt.Errorf("begin Space voice-room claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, voiceRoomID); err != nil {
		return store.SpaceMediaRoomHead{}, fmt.Errorf("serialize Space voice-room claim: %w", err)
	}
	var head store.SpaceMediaRoomHead
	var roomGeneration int64
	err = tx.QueryRow(ctx, `SELECT voice_room_id,space_id,call_room_id,room_generation,creator_operation_id,state
FROM voice_space_media_room_heads WHERE voice_room_id=$1 ORDER BY room_generation DESC LIMIT 1 FOR UPDATE`, voiceRoomID).Scan(
		&head.VoiceRoomID, &head.SpaceID, &head.RoomID, &roomGeneration, &head.CreatorOperationID, &head.State)
	if err == nil {
		head.RoomGeneration = uint64(roomGeneration)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if !allowCreate {
			return store.SpaceMediaRoomHead{}, ErrAdmissionConflict
		}
		_, err = tx.Exec(ctx, `INSERT INTO voice_space_media_room_heads
(voice_room_id,room_generation,space_id,call_room_id,creator_operation_id,state)
VALUES($1,1,$2,$3,$4,'OPEN')`, voiceRoomID, spaceID, proposedRoomID, operationID)
		if err != nil {
			return store.SpaceMediaRoomHead{}, fmt.Errorf("claim initial Space voice-room incarnation: %w", err)
		}
		err = tx.QueryRow(ctx, `SELECT voice_room_id,space_id,call_room_id,room_generation,creator_operation_id,state
FROM voice_space_media_room_heads WHERE voice_room_id=$1 ORDER BY room_generation DESC LIMIT 1 FOR UPDATE`, voiceRoomID).Scan(
			&head.VoiceRoomID, &head.SpaceID, &head.RoomID, &roomGeneration, &head.CreatorOperationID, &head.State)
		if err == nil {
			head.RoomGeneration = uint64(roomGeneration)
		}
	}
	if err != nil {
		return store.SpaceMediaRoomHead{}, fmt.Errorf("read Space voice-room incarnation: %w", err)
	}
	if head.SpaceID != spaceID {
		return store.SpaceMediaRoomHead{}, ErrAdmissionConflict
	}
	switch head.State {
	case "OPEN":
		if head.CreatorOperationID == operationID {
			head.Ready = false
			return head, tx.Commit(ctx)
		}
		var ready bool
		err = tx.QueryRow(ctx, `SELECT state='COMMITTED' AND projection_applied_at IS NOT NULL
FROM voice_space_media_admissions WHERE operation_id=$1 AND voice_room_id=$2 AND room_generation=$3 AND room_id=$4`,
			head.CreatorOperationID, voiceRoomID, int64(head.RoomGeneration), head.RoomID).Scan(&ready)
		if err != nil || !ready {
			return store.SpaceMediaRoomHead{}, ErrRoomHeadPending
		}
		head.Ready = true
	case "CLOSING":
		return store.SpaceMediaRoomHead{}, ErrRoomHeadPending
	case "ENDED":
		if !allowCreate {
			return store.SpaceMediaRoomHead{}, ErrAdmissionConflict
		}
		if head.RoomGeneration >= uint64(^uint64(0)>>1) {
			return store.SpaceMediaRoomHead{}, ErrAdmissionConflict
		}
		var unresolved bool
		err = tx.QueryRow(ctx, `SELECT EXISTS (
	SELECT 1 FROM voice_space_media_admissions WHERE voice_room_id=$1 AND room_generation=$2
	 AND (participant_state <> 'ENDED' OR (state='ABORTING' AND cleanup_completed_at IS NULL)
	      OR (state='COMMITTED' AND projection_applied_at IS NULL)))`, voiceRoomID, head.RoomGeneration).Scan(&unresolved)
		if err != nil || unresolved {
			return store.SpaceMediaRoomHead{}, ErrRoomHeadPending
		}
		head.RoomGeneration++
		head.RoomID = proposedRoomID
		head.CreatorOperationID = operationID
		head.State = "OPEN"
		head.Ready = false
		_, err = tx.Exec(ctx, `INSERT INTO voice_space_media_room_heads(voice_room_id,room_generation,space_id,call_room_id,creator_operation_id,state)
VALUES($1,$2,$3,$4,$5,'OPEN')`, voiceRoomID, int64(head.RoomGeneration), spaceID, proposedRoomID, operationID)
		if err != nil {
			return store.SpaceMediaRoomHead{}, fmt.Errorf("reopen Space voice-room incarnation: %w", err)
		}
	default:
		return store.SpaceMediaRoomHead{}, ErrAdmissionConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return store.SpaceMediaRoomHead{}, fmt.Errorf("commit Space voice-room claim: %w", err)
	}
	return head, nil
}

// AbandonRoomClaim ends only a creator-owned OPEN head that has no live or
// unresolved participant operation. It is used when creator Prepare failed
// before a durable admission could be confirmed.
func (s *PostgresAdmissionStore) AbandonRoomClaim(ctx context.Context, voiceRoomID string, roomGeneration uint64, operationID uuid.UUID) error {
	if !s.usable() || voiceRoomID == "" || roomGeneration == 0 || operationID == uuid.Nil {
		return ErrAdmissionUnavailable
	}
	tag, err := s.pool.Exec(ctx, `UPDATE voice_space_media_room_heads h SET state='ENDED',updated_at=clock_timestamp()
WHERE h.voice_room_id=$1 AND h.room_generation=$2 AND h.creator_operation_id=$3 AND h.state='OPEN'
 AND NOT EXISTS (SELECT 1 FROM voice_space_media_admissions a WHERE a.voice_room_id=h.voice_room_id AND a.room_generation=h.room_generation
  AND (a.participant_state <> 'ENDED' OR (a.state='ABORTING' AND a.cleanup_completed_at IS NULL)
   OR (a.state='COMMITTED' AND a.projection_applied_at IS NULL)))`, voiceRoomID, int64(roomGeneration), operationID)
	if err != nil {
		return fmt.Errorf("abandon exact Space voice-room claim: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrRoomHeadPending
	}
	return nil
}

func admissionFingerprint(a Admission) ([32]byte, error) {
	type eventFingerprint struct {
		ID            string   `json:"id"`
		Subject       string   `json:"subject"`
		PayloadSHA256 [32]byte `json:"payload_sha256"`
	}
	rows := make([]eventFingerprint, 0, len(a.Events))
	for _, event := range a.Events {
		rows = append(rows, eventFingerprint{event.ID.String(), event.Subject, sha256.Sum256(event.Payload)})
	}
	canonical := struct {
		OperationID     string             `json:"operation_id"`
		Generation      string             `json:"generation"`
		RoomGeneration  uint64             `json:"room_generation"`
		AccountID       string             `json:"account_id"`
		ProfileID       string             `json:"profile_id"`
		SpaceID         string             `json:"space_id"`
		RoomID          string             `json:"room_id"`
		VoiceRoomID     string             `json:"voice_room_id"`
		Identity        string             `json:"identity"`
		CreatedRoom     bool               `json:"created_room"`
		StartedAt       time.Time          `json:"call_started_at"`
		MaxParticipants int                `json:"max_participants"`
		SessionEpoch    uint64             `json:"session_epoch"`
		AccessEpoch     uint64             `json:"access_epoch"`
		PolicyEpoch     uint64             `json:"policy_epoch"`
		CanJoin         bool               `json:"can_join"`
		CanPublish      bool               `json:"can_publish_audio"`
		CanSubscribe    bool               `json:"can_subscribe"`
		Events          []eventFingerprint `json:"events"`
	}{a.OperationID.String(), a.Generation, a.RoomGeneration, a.AccountID.String(), a.ProfileID.String(), a.SpaceID.String(), a.RoomID, a.VoiceRoomID,
		a.Identity, a.CreatedRoom, a.CallStartedAt.UTC(), a.MaxParticipants, a.SessionEpoch, a.AccessEpoch, a.PolicyEpoch, a.CanJoin, a.CanPublishAudio, a.CanSubscribe, rows}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

func validateAdmission(a Admission) error {
	if a.OperationID == uuid.Nil || a.Generation == "" || a.AccountID == uuid.Nil || a.ProfileID == uuid.Nil ||
		a.SpaceID == uuid.Nil || a.RoomID == "" || a.VoiceRoomID == "" || a.RoomGeneration == 0 || a.Identity == "" || a.CallStartedAt.IsZero() || a.MaxParticipants <= 0 || a.SessionEpoch == 0 || a.AccessEpoch == 0 || a.PolicyEpoch == 0 || !a.CanJoin || !a.CanSubscribe || len(a.Events) == 0 {
		return ErrAdmissionConflict
	}
	seen := map[uuid.UUID]bool{}
	started, joined := 0, 0
	for _, event := range a.Events {
		if event.ID == uuid.Nil || len(event.Payload) == 0 || (event.Subject != "voice.call_started" && event.Subject != "voice.member_joined") || seen[event.ID] {
			return ErrAdmissionConflict
		}
		seen[event.ID] = true
		switch event.Subject {
		case "voice.call_started":
			started++
		case "voice.member_joined":
			joined++
		}
	}
	if joined != 1 || (a.CreatedRoom && started != 1) || (!a.CreatedRoom && started != 0) {
		return ErrAdmissionConflict
	}
	return nil
}

// Prepare durably records the immutable intent and fixed event payloads. Its
// events cannot be claimed by the relay until the row is COMMITTED and the
// Redis projection-applied marker is confirmed.
func (s *PostgresAdmissionStore) Prepare(ctx context.Context, a Admission) error {
	if !s.usable() || validateAdmission(a) != nil {
		return ErrAdmissionUnavailable
	}
	fingerprint, err := admissionFingerprint(a)
	if err != nil {
		return ErrAdmissionUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Space media admission: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var claimedGeneration int64
	var claimedSpace, claimedRoom string
	var claimedCreator uuid.UUID
	var claimedState string
	err = tx.QueryRow(ctx, `SELECT room_generation,space_id,call_room_id,creator_operation_id,state
FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, a.VoiceRoomID, int64(a.RoomGeneration)).Scan(&claimedGeneration, &claimedSpace, &claimedRoom, &claimedCreator, &claimedState)
	if err != nil || claimedGeneration != int64(a.RoomGeneration) || claimedSpace != a.SpaceID.String() || claimedRoom != a.RoomID || claimedState != "OPEN" ||
		(a.CreatedRoom && claimedCreator != a.OperationID) || (!a.CreatedRoom && claimedCreator == a.OperationID) {
		return ErrRoomHeadPending
	}
	_, err = tx.Exec(ctx, `INSERT INTO voice_space_media_admissions
(operation_id,generation,account_id,profile_id,space_id,room_id,voice_room_id,room_generation,livekit_identity,created_room,call_started_at,max_participants,session_epoch,access_epoch,policy_epoch,can_join,can_publish_audio,can_subscribe,state,request_sha256)
SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,'PREPARED',$19
WHERE EXISTS (SELECT 1 FROM voice_space_media_room_heads h WHERE h.voice_room_id=$7 AND h.room_generation=$8
 AND h.space_id=$5 AND h.call_room_id=$6 AND h.state='OPEN'
 AND (($10::boolean AND h.creator_operation_id=$1) OR (NOT $10::boolean AND h.creator_operation_id<>$1)))
ON CONFLICT(operation_id) DO NOTHING`,
		a.OperationID, a.Generation, a.AccountID, a.ProfileID, a.SpaceID, a.RoomID, a.VoiceRoomID, int64(a.RoomGeneration), a.Identity, a.CreatedRoom, a.CallStartedAt.UTC(), a.MaxParticipants, int64(a.SessionEpoch), int64(a.AccessEpoch), int64(a.PolicyEpoch), a.CanJoin, a.CanPublishAudio, a.CanSubscribe, fingerprint[:])
	if err != nil {
		return fmt.Errorf("persist Space media admission intent: %w", err)
	}
	var stored []byte
	err = tx.QueryRow(ctx, `SELECT request_sha256 FROM voice_space_media_admissions WHERE operation_id=$1 FOR UPDATE`, a.OperationID).Scan(&stored)
	if err != nil || !equalDigest(stored, fingerprint[:]) {
		return ErrAdmissionConflict
	}
	for _, event := range a.Events {
		_, err = tx.Exec(ctx, `INSERT INTO voice_space_media_admission_outbox(event_id,operation_id,voice_room_id,room_generation,subject,payload)
VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(event_id) DO NOTHING`, event.ID, a.OperationID, a.VoiceRoomID, int64(a.RoomGeneration), event.Subject, event.Payload)
		if err != nil {
			return fmt.Errorf("persist Space media event intent: %w", err)
		}
		var op uuid.UUID
		var eventRoomID uuid.UUID
		var eventRoomGeneration int64
		var subject string
		var payload []byte
		err = tx.QueryRow(ctx, `SELECT operation_id,voice_room_id,room_generation,subject,payload FROM voice_space_media_admission_outbox WHERE event_id=$1 FOR UPDATE`, event.ID).Scan(&op, &eventRoomID, &eventRoomGeneration, &subject, &payload)
		if err != nil || op != a.OperationID || eventRoomID.String() != a.VoiceRoomID || eventRoomGeneration != int64(a.RoomGeneration) || subject != event.Subject || string(payload) != string(event.Payload) {
			return ErrAdmissionConflict
		}
	}
	return tx.Commit(ctx)
}

// Fence installs the operation-owned active account fence and advances the
// matching prepared operation in one PostgreSQL transaction.
func (s *PostgresAdmissionStore) Fence(ctx context.Context, a Admission) error {
	if !s.usable() || a.OperationID == uuid.Nil || a.AccountID == uuid.Nil || a.ProfileID == uuid.Nil || a.RoomID == "" || a.Generation == "" {
		return ErrAdmissionUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Space media fence: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var headGeneration int64
	var headSpace, headRoom, headState string
	var headCreator uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT room_generation,space_id,call_room_id,creator_operation_id,state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, a.VoiceRoomID, int64(a.RoomGeneration)).Scan(&headGeneration, &headSpace, &headRoom, &headCreator, &headState); err != nil || headGeneration != int64(a.RoomGeneration) || headSpace != a.SpaceID.String() || headRoom != a.RoomID || headState != "OPEN" || (a.CreatedRoom && headCreator != a.OperationID) || (!a.CreatedRoom && headCreator == a.OperationID) {
		return ErrRoomHeadPending
	}
	if _, err = tx.Exec(ctx, `INSERT INTO voice_profile_account_mappings(profile_id,account_id) VALUES($1,$2) ON CONFLICT(profile_id) DO NOTHING`, a.ProfileID, a.AccountID); err != nil {
		return fmt.Errorf("persist Space media account mapping: %w", err)
	}
	var mapped uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT account_id FROM voice_profile_account_mappings WHERE profile_id=$1 FOR UPDATE`, a.ProfileID).Scan(&mapped); err != nil || mapped != a.AccountID {
		return ErrAdmissionConflict
	}
	tag, err := tx.Exec(ctx, `INSERT INTO voice_account_voice_fences(account_id,profile_id,room_id,state,reservation_expires_at,admission_operation_id,admission_generation)
VALUES($1,$2,$3,'active',NULL,$4,$5)
ON CONFLICT(account_id) DO UPDATE SET profile_id=EXCLUDED.profile_id,room_id=EXCLUDED.room_id,state='active',reservation_expires_at=NULL,
 admission_operation_id=EXCLUDED.admission_operation_id,admission_generation=EXCLUDED.admission_generation,updated_at=clock_timestamp()
WHERE voice_account_voice_fences.profile_id=EXCLUDED.profile_id AND voice_account_voice_fences.room_id=EXCLUDED.room_id
 AND ((voice_account_voice_fences.admission_operation_id=EXCLUDED.admission_operation_id AND voice_account_voice_fences.admission_generation=EXCLUDED.admission_generation)
  OR (voice_account_voice_fences.admission_operation_id IS NULL AND voice_account_voice_fences.state='reserving'
   AND voice_account_voice_fences.reservation_expires_at<=clock_timestamp()))`,
		a.AccountID, a.ProfileID, a.RoomID, a.OperationID, a.Generation)
	if err != nil {
		return fmt.Errorf("install operation-owned account Voice fence: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	tag, err = tx.Exec(ctx, `UPDATE voice_space_media_admissions SET state='FENCED',updated_at=clock_timestamp()
WHERE operation_id=$1 AND generation=$2 AND account_id=$3 AND profile_id=$4 AND room_id=$5 AND state IN ('PREPARED','FENCED')`,
		a.OperationID, a.Generation, a.AccountID, a.ProfileID, a.RoomID)
	if err != nil {
		return fmt.Errorf("mark Space media admission fenced: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	return tx.Commit(ctx)
}

func (s *PostgresAdmissionStore) MarkCommitted(ctx context.Context, operationID uuid.UUID, generation string) error {
	if !s.usable() || operationID == uuid.Nil || generation == "" {
		return ErrAdmissionUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Space media admission commit: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var voiceRoomID, roomID string
	var roomGeneration int64
	err = tx.QueryRow(ctx, `SELECT voice_room_id,room_generation,room_id FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operationID, generation).Scan(&voiceRoomID, &roomGeneration, &roomID)
	if err != nil {
		return err
	}
	var headState, headRoomID string
	var currentRoomGeneration int64
	err = tx.QueryRow(ctx, `SELECT state,room_generation,call_room_id FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, voiceRoomID, roomGeneration).Scan(&headState, &currentRoomGeneration, &headRoomID)
	if err != nil || headState != "OPEN" || currentRoomGeneration != roomGeneration || headRoomID != roomID {
		return ErrRoomHeadPending
	}
	var storedRoomID string
	err = tx.QueryRow(ctx, `SELECT room_id FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2 FOR UPDATE`, operationID, generation).Scan(&storedRoomID)
	if err != nil || storedRoomID != roomID {
		return ErrAdmissionConflict
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_space_media_admissions SET state='COMMITTED',updated_at=clock_timestamp()
WHERE operation_id=$1 AND generation=$2 AND state='FENCED'`, operationID, generation)
	if err != nil {
		return fmt.Errorf("commit Space media admission: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return tx.Commit(ctx)
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT state FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operationID, generation).Scan(&state)
	if err == nil && state == string(AdmissionCommitted) {
		return tx.Commit(ctx)
	}
	return ErrAdmissionConflict
}

func (s *PostgresAdmissionStore) MarkProjectionApplied(ctx context.Context, operationID uuid.UUID, generation string) error {
	if !s.usable() || operationID == uuid.Nil || generation == "" {
		return ErrAdmissionUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var voiceRoomID string
	var roomGeneration int64
	if err = tx.QueryRow(ctx, `SELECT voice_room_id,room_generation FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operationID, generation).Scan(&voiceRoomID, &roomGeneration); err != nil {
		return err
	}
	var state string
	var currentGeneration int64
	if err = tx.QueryRow(ctx, `SELECT state,room_generation FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, voiceRoomID, roomGeneration).Scan(&state, &currentGeneration); err != nil || state != "OPEN" || currentGeneration != roomGeneration {
		return ErrRoomHeadPending
	}
	var storedGeneration int64
	if err = tx.QueryRow(ctx, `SELECT room_generation FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2 FOR UPDATE`, operationID, generation).Scan(&storedGeneration); err != nil || storedGeneration != roomGeneration {
		return ErrAdmissionConflict
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_space_media_admissions SET projection_applied_at=COALESCE(projection_applied_at,clock_timestamp()),participant_state='ACTIVE',updated_at=clock_timestamp()
WHERE operation_id=$1 AND generation=$2 AND state='COMMITTED'`, operationID, generation)
	if err != nil {
		return fmt.Errorf("record Space media projection: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	return tx.Commit(ctx)
}

func (s *PostgresAdmissionStore) ConfirmProjection(ctx context.Context, operationID uuid.UUID, generation string) error {
	if !s.usable() || operationID == uuid.Nil || generation == "" {
		return ErrAdmissionUnavailable
	}
	var confirmed bool
	err := s.pool.QueryRow(ctx, `SELECT state='COMMITTED' AND participant_state='ACTIVE' AND projection_applied_at IS NOT NULL
FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operationID, generation).Scan(&confirmed)
	if err != nil {
		return fmt.Errorf("read Space media admission confirmation: %w", err)
	}
	if !confirmed {
		return ErrAdmissionConflict
	}
	return nil
}

func (s *PostgresAdmissionStore) ConfirmRoomHeadOpen(ctx context.Context, a Admission) error {
	if !s.usable() || a.OperationID == uuid.Nil || a.RoomGeneration == 0 || a.VoiceRoomID == "" || a.RoomID == "" {
		return ErrAdmissionUnavailable
	}
	var creator uuid.UUID
	var state string
	err := s.pool.QueryRow(ctx, `SELECT creator_operation_id,state FROM voice_space_media_room_heads
WHERE voice_room_id=$1 AND room_generation=$2 AND call_room_id=$3`, a.VoiceRoomID, int64(a.RoomGeneration), a.RoomID).Scan(&creator, &state)
	if err != nil || state != "OPEN" || (a.CreatedRoom && creator != a.OperationID) || (!a.CreatedRoom && creator == a.OperationID) {
		return ErrRoomHeadPending
	}
	return nil
}

// RecoverOrphanRoomHead closes only an old creator claim that has no durable
// admission row. The age guard prevents recovery from racing the short
// claim-to-Prepare window of a live request.
func (s *PostgresAdmissionStore) RecoverOrphanRoomHeads(ctx context.Context) error {
	if !s.usable() {
		return ErrAdmissionUnavailable
	}
	_, err := s.pool.Exec(ctx, `UPDATE voice_space_media_room_heads h SET state='ENDED',updated_at=clock_timestamp()
WHERE h.state='OPEN' AND h.created_at < clock_timestamp()-interval '30 seconds'
 AND NOT EXISTS (SELECT 1 FROM voice_space_media_admissions a WHERE a.operation_id=h.creator_operation_id)
 AND NOT EXISTS (SELECT 1 FROM voice_space_media_admissions a WHERE a.voice_room_id=h.voice_room_id AND a.room_generation=h.room_generation)`)
	if err != nil {
		return fmt.Errorf("recover orphan Space voice-room claims: %w", err)
	}
	return nil
}

func (s *PostgresAdmissionStore) MarkAborting(ctx context.Context, operationID uuid.UUID, generation string) error {
	if !s.usable() || operationID == uuid.Nil || generation == "" {
		return ErrAdmissionUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var voiceRoomID string
	var roomGeneration int64
	if err = tx.QueryRow(ctx, `SELECT voice_room_id,room_generation FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operationID, generation).Scan(&voiceRoomID, &roomGeneration); err != nil {
		return err
	}
	var headGeneration int64
	var headState string
	if err = tx.QueryRow(ctx, `SELECT room_generation,state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, voiceRoomID, roomGeneration).Scan(&headGeneration, &headState); err != nil || headGeneration != roomGeneration {
		return ErrRoomHeadPending
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_space_media_admissions SET state='ABORTING',updated_at=clock_timestamp()
WHERE operation_id=$1 AND generation=$2 AND (state IN ('PREPARED','FENCED') OR (state='COMMITTED' AND projection_applied_at IS NULL))`, operationID, generation)
	if err != nil {
		return fmt.Errorf("mark Space media admission aborting: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return tx.Commit(ctx)
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT state FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operationID, generation).Scan(&state)
	if err == nil && state == string(AdmissionAborting) {
		return tx.Commit(ctx)
	}
	return ErrAdmissionConflict
}

// ReleaseFence removes only the exact operation incarnation. Legacy tuple
// release is kept separate and cannot remove an owned Space-media fence.
func (s *PostgresAdmissionStore) ReleaseFence(ctx context.Context, a Admission) error {
	if !s.usable() {
		return ErrAdmissionUnavailable
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM voice_account_voice_fences WHERE account_id=$1 AND profile_id=$2 AND room_id=$3 AND admission_operation_id=$4 AND admission_generation=$5`, a.AccountID, a.ProfileID, a.RoomID, a.OperationID, a.Generation)
	if err != nil {
		return fmt.Errorf("release exact Space media fence: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	return nil
}

// BeginRoomLeave serializes a participant drain with admissions and room
// closure. The Redis identity remains revoking until CompleteRoomLeave.
func (s *PostgresAdmissionStore) BeginRoomLeave(ctx context.Context, operationID uuid.UUID, generation string) error {
	if !s.usable() || operationID == uuid.Nil || generation == "" {
		return ErrAdmissionUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var voiceRoomID string
	var roomGeneration int64
	if err = tx.QueryRow(ctx, `SELECT voice_room_id,room_generation FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operationID, generation).Scan(&voiceRoomID, &roomGeneration); err != nil {
		return err
	}
	var headState string
	var headGeneration int64
	if err = tx.QueryRow(ctx, `SELECT state,room_generation FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, voiceRoomID, roomGeneration).Scan(&headState, &headGeneration); err != nil || headGeneration != roomGeneration || (headState != "OPEN" && headState != "CLOSING") {
		return ErrRoomHeadPending
	}
	var participantState, admissionState string
	if err = tx.QueryRow(ctx, `SELECT participant_state,state FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2 FOR UPDATE`, operationID, generation).Scan(&participantState, &admissionState); err != nil {
		return err
	}
	if participantState == "ENDED" {
		return tx.Commit(ctx)
	}
	if participantState != "ACTIVE" && participantState != "REVOKING" && !(participantState == "PREPARED" && admissionState == "ABORTING") {
		return ErrAdmissionConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE voice_space_media_admissions SET participant_state='REVOKING',updated_at=clock_timestamp() WHERE operation_id=$1 AND generation=$2`, operationID, generation); err != nil {
		return err
	}
	var activeOthers int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admissions WHERE voice_room_id=$1 AND room_generation=$2
 AND participant_state='ACTIVE' AND NOT (operation_id=$3 AND generation=$4)`, voiceRoomID, roomGeneration, operationID, generation).Scan(&activeOthers); err != nil {
		return err
	}
	if activeOthers == 0 {
		if _, err = tx.Exec(ctx, `UPDATE voice_space_media_room_heads SET state='CLOSING',updated_at=clock_timestamp() WHERE voice_room_id=$1 AND room_generation=$2 AND state='OPEN'`, voiceRoomID, roomGeneration); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE voice_space_media_admissions SET state='ABORTING',updated_at=clock_timestamp()
WHERE voice_room_id=$1 AND room_generation=$2 AND state IN ('PREPARED','FENCED')`, voiceRoomID, roomGeneration); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE voice_space_media_admissions SET state='ABORTING',updated_at=clock_timestamp()
WHERE voice_room_id=$1 AND room_generation=$2 AND state='COMMITTED' AND projection_applied_at IS NULL`, voiceRoomID, roomGeneration); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// CompleteRoomLeave releases only the exact admission-owned fence after the
// caller has confirmed removal (or NotFound) of that exact LiveKit identity.
func (s *PostgresAdmissionStore) CompleteRoomLeave(ctx context.Context, a Admission) error {
	if !s.usable() || a.OperationID == uuid.Nil || a.Generation == "" || a.AccountID == uuid.Nil || a.ProfileID == uuid.Nil || a.RoomID == "" {
		return ErrAdmissionUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var voiceRoomID string
	var roomGeneration int64
	if err = tx.QueryRow(ctx, `SELECT voice_room_id,room_generation FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, a.OperationID, a.Generation).Scan(&voiceRoomID, &roomGeneration); err != nil {
		return err
	}
	var headState string
	var headGeneration int64
	if err = tx.QueryRow(ctx, `SELECT state,room_generation FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, voiceRoomID, roomGeneration).Scan(&headState, &headGeneration); err != nil || headGeneration != roomGeneration {
		return ErrRoomHeadPending
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT participant_state FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2 FOR UPDATE`, a.OperationID, a.Generation).Scan(&state); err != nil {
		return err
	}
	if state == "ENDED" {
		return tx.Commit(ctx)
	}
	if state != "REVOKING" {
		return ErrAdmissionConflict
	}
	tag, err := tx.Exec(ctx, `DELETE FROM voice_account_voice_fences WHERE account_id=$1 AND profile_id=$2 AND room_id=$3 AND admission_operation_id=$4 AND admission_generation=$5`, a.AccountID, a.ProfileID, a.RoomID, a.OperationID, a.Generation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE voice_space_media_admissions SET participant_state='ENDED',updated_at=clock_timestamp() WHERE operation_id=$1 AND generation=$2 AND participant_state='REVOKING'`, a.OperationID, a.Generation); err != nil {
		return err
	}
	if headState == "CLOSING" {
		var unresolved bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM voice_space_media_admissions WHERE voice_room_id=$1 AND room_generation=$2
 AND (participant_state IN ('ACTIVE','REVOKING','PREPARED') OR (state IN ('PREPARED','FENCED') AND cleanup_completed_at IS NULL)
  OR (state='ABORTING' AND cleanup_completed_at IS NULL) OR (state='COMMITTED' AND projection_applied_at IS NULL)))`, voiceRoomID, roomGeneration).Scan(&unresolved); err != nil {
			return err
		}
		if !unresolved {
			if _, err = tx.Exec(ctx, `UPDATE voice_space_media_room_heads SET state='ENDED',updated_at=clock_timestamp() WHERE voice_room_id=$1 AND room_generation=$2 AND state='CLOSING'`, voiceRoomID, roomGeneration); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresAdmissionStore) MarkCleanupCompleted(ctx context.Context, operationID uuid.UUID, generation string) error {
	if !s.usable() {
		return ErrAdmissionUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var voiceRoomID string
	var roomGeneration int64
	if err = tx.QueryRow(ctx, `SELECT voice_room_id,room_generation FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2`, operationID, generation).Scan(&voiceRoomID, &roomGeneration); err != nil {
		return err
	}
	var headState string
	var headGeneration int64
	var creator uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT state,room_generation,creator_operation_id FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2 FOR UPDATE`, voiceRoomID, roomGeneration).Scan(&headState, &headGeneration, &creator); err != nil || headGeneration != roomGeneration {
		return ErrRoomHeadPending
	}
	var admissionState string
	if err = tx.QueryRow(ctx, `SELECT state FROM voice_space_media_admissions WHERE operation_id=$1 AND generation=$2 FOR UPDATE`, operationID, generation).Scan(&admissionState); err != nil || admissionState != string(AdmissionAborting) {
		return ErrAdmissionConflict
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_space_media_admissions SET cleanup_completed_at=COALESCE(cleanup_completed_at,clock_timestamp()),participant_state='ENDED',updated_at=clock_timestamp()
WHERE operation_id=$1 AND generation=$2 AND state='ABORTING'`, operationID, generation)
	if err != nil {
		return fmt.Errorf("complete Space media cleanup: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	if headState == "CLOSING" || (headState == "OPEN" && creator == operationID) {
		var unresolved bool
		err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM voice_space_media_admissions
WHERE voice_room_id=$1 AND room_generation=$2 AND (participant_state IN ('PREPARED','ACTIVE','REVOKING')
 OR (state IN ('PREPARED','FENCED') AND cleanup_completed_at IS NULL)
 OR (state='ABORTING' AND cleanup_completed_at IS NULL)
 OR (state='COMMITTED' AND projection_applied_at IS NULL)))`, voiceRoomID, roomGeneration).Scan(&unresolved)
		if err != nil {
			return err
		}
		if !unresolved {
			_, err = tx.Exec(ctx, `UPDATE voice_space_media_room_heads SET state='ENDED',updated_at=clock_timestamp()
WHERE voice_room_id=$1 AND room_generation=$2 AND state IN ('OPEN','CLOSING')`, voiceRoomID, roomGeneration)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresAdmissionStore) RecoveryRows(ctx context.Context, limit int) ([]Admission, error) {
	if !s.usable() || limit < 1 || limit > 1000 {
		return nil, ErrAdmissionUnavailable
	}
	rows, err := s.pool.Query(ctx, `SELECT operation_id,generation,account_id,profile_id,space_id,room_id,voice_room_id,room_generation,livekit_identity,created_room,call_started_at,max_participants,session_epoch,access_epoch,policy_epoch,can_join,can_publish_audio,can_subscribe,state,participant_state,projection_applied_at IS NOT NULL
FROM voice_space_media_admissions WHERE cleanup_completed_at IS NULL
 AND (state <> 'COMMITTED' OR projection_applied_at IS NULL OR participant_state='REVOKING') ORDER BY updated_at,operation_id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("read Space media recovery rows: %w", err)
	}
	defer rows.Close()
	out := make([]Admission, 0)
	for rows.Next() {
		var a Admission
		var state string
		var roomGeneration, session, access, policy int64
		if err := rows.Scan(&a.OperationID, &a.Generation, &a.AccountID, &a.ProfileID, &a.SpaceID, &a.RoomID, &a.VoiceRoomID, &roomGeneration, &a.Identity, &a.CreatedRoom, &a.CallStartedAt, &a.MaxParticipants, &session, &access, &policy, &a.CanJoin, &a.CanPublishAudio, &a.CanSubscribe, &state, &a.ParticipantState, &a.ProjectionApplied); err != nil {
			return nil, err
		}
		if roomGeneration <= 0 || session <= 0 || access <= 0 || policy <= 0 {
			return nil, ErrAdmissionConflict
		}
		a.SessionEpoch = uint64(session)
		a.RoomGeneration = uint64(roomGeneration)
		a.AccessEpoch = uint64(access)
		a.PolicyEpoch = uint64(policy)
		a.State = AdmissionState(state)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ProjectionRows pages through durable active admissions independently of the
// Redis indexes. Creator rows are read first by the coordinator so it can
// reconstruct a missing call shell before restoring its other participants.
func (s *PostgresAdmissionStore) ProjectionRows(ctx context.Context, after uuid.UUID, limit int, creators bool) ([]Admission, error) {
	if !s.usable() || limit < 1 || limit > 1000 {
		return nil, ErrAdmissionUnavailable
	}
	rows, err := s.pool.Query(ctx, `SELECT operation_id,generation,account_id,profile_id,space_id,room_id,voice_room_id,room_generation,livekit_identity,created_room,call_started_at,max_participants,session_epoch,access_epoch,policy_epoch,can_join,can_publish_audio,can_subscribe,state,participant_state,projection_applied_at IS NOT NULL
FROM voice_space_media_admissions WHERE operation_id > $1 AND state='COMMITTED' AND participant_state='ACTIVE'
 AND projection_applied_at IS NOT NULL AND created_room=$2 ORDER BY operation_id LIMIT $3`, after, creators, limit)
	if err != nil {
		return nil, fmt.Errorf("read durable Space media projections: %w", err)
	}
	defer rows.Close()
	out := make([]Admission, 0)
	for rows.Next() {
		var a Admission
		var state string
		var roomGeneration, session, access, policy int64
		if err := rows.Scan(&a.OperationID, &a.Generation, &a.AccountID, &a.ProfileID, &a.SpaceID, &a.RoomID, &a.VoiceRoomID, &roomGeneration, &a.Identity, &a.CreatedRoom, &a.CallStartedAt, &a.MaxParticipants, &session, &access, &policy, &a.CanJoin, &a.CanPublishAudio, &a.CanSubscribe, &state, &a.ParticipantState, &a.ProjectionApplied); err != nil {
			return nil, err
		}
		if roomGeneration <= 0 || session <= 0 || access <= 0 || policy <= 0 {
			return nil, ErrAdmissionConflict
		}
		a.SessionEpoch, a.RoomGeneration, a.AccessEpoch, a.PolicyEpoch = uint64(session), uint64(roomGeneration), uint64(access), uint64(policy)
		a.State = AdmissionState(state)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *PostgresAdmissionStore) RoomCreator(ctx context.Context, voiceRoomID string, roomGeneration uint64) (Admission, error) {
	if !s.usable() || voiceRoomID == "" || roomGeneration == 0 {
		return Admission{}, ErrAdmissionUnavailable
	}
	var a Admission
	var state string
	var roomGen, session, access, policy int64
	err := s.pool.QueryRow(ctx, `SELECT a.operation_id,a.generation,a.account_id,a.profile_id,a.space_id,a.room_id,a.voice_room_id,a.room_generation,a.livekit_identity,a.created_room,a.call_started_at,a.max_participants,a.session_epoch,a.access_epoch,a.policy_epoch,a.can_join,a.can_publish_audio,a.can_subscribe,a.state,a.participant_state,a.projection_applied_at IS NOT NULL
FROM voice_space_media_room_heads h JOIN voice_space_media_admissions a ON a.operation_id=h.creator_operation_id
 AND a.voice_room_id=h.voice_room_id AND a.room_generation=h.room_generation
WHERE h.voice_room_id=$1 AND h.room_generation=$2 AND h.state='OPEN' AND a.created_room
 AND a.state='COMMITTED' AND a.projection_applied_at IS NOT NULL`, voiceRoomID, int64(roomGeneration)).Scan(
		&a.OperationID, &a.Generation, &a.AccountID, &a.ProfileID, &a.SpaceID, &a.RoomID, &a.VoiceRoomID, &roomGen,
		&a.Identity, &a.CreatedRoom, &a.CallStartedAt, &a.MaxParticipants, &session, &access, &policy,
		&a.CanJoin, &a.CanPublishAudio, &a.CanSubscribe, &state, &a.ParticipantState, &a.ProjectionApplied)
	if err != nil {
		return Admission{}, fmt.Errorf("read Space voice-room creator admission: %w", err)
	}
	if roomGen <= 0 || session <= 0 || access <= 0 || policy <= 0 || !a.CreatedRoom || !a.ProjectionApplied {
		return Admission{}, ErrAdmissionConflict
	}
	a.RoomGeneration, a.SessionEpoch, a.AccessEpoch, a.PolicyEpoch = uint64(roomGen), uint64(session), uint64(access), uint64(policy)
	a.State = AdmissionState(state)
	return a, nil
}

type AdmissionOutboxItem struct {
	ID          uuid.UUID
	OperationID uuid.UUID
	Subject     string
	Payload     []byte
	LeaseToken  uuid.UUID
}

func (s *PostgresAdmissionStore) ClaimOutbox(ctx context.Context, lease time.Duration) (*AdmissionOutboxItem, error) {
	if !s.usable() || lease <= 0 || lease > time.Minute {
		return nil, ErrAdmissionUnavailable
	}
	item := &AdmissionOutboxItem{LeaseToken: uuid.New()}
	err := s.pool.QueryRow(ctx, `WITH candidate AS (
 SELECT o.event_id FROM voice_space_media_admission_outbox o JOIN voice_space_media_admissions a USING(operation_id)
 WHERE o.delivered_at IS NULL AND (o.lease_until IS NULL OR o.lease_until<=clock_timestamp())
  AND a.state='COMMITTED' AND a.projection_applied_at IS NOT NULL
 ORDER BY o.created_at,o.event_id FOR UPDATE OF o SKIP LOCKED LIMIT 1
)
UPDATE voice_space_media_admission_outbox o SET lease_token=$1,lease_until=clock_timestamp()+$2,attempts=attempts+1
FROM candidate c WHERE o.event_id=c.event_id RETURNING o.event_id,o.operation_id,o.subject,o.payload`, item.LeaseToken, lease).Scan(&item.ID, &item.OperationID, &item.Subject, &item.Payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim Space media event intent: %w", err)
	}
	return item, nil
}

func (s *PostgresAdmissionStore) MarkOutboxDelivered(ctx context.Context, item AdmissionOutboxItem) error {
	if !s.usable() {
		return ErrAdmissionUnavailable
	}
	tag, err := s.pool.Exec(ctx, `UPDATE voice_space_media_admission_outbox o SET delivered_at=clock_timestamp(),lease_token=NULL,lease_until=NULL
FROM voice_space_media_admissions a WHERE o.event_id=$1 AND o.operation_id=$2 AND o.lease_token=$3 AND o.delivered_at IS NULL
 AND a.operation_id=o.operation_id AND a.state='COMMITTED' AND a.projection_applied_at IS NOT NULL`, item.ID, item.OperationID, item.LeaseToken)
	if err != nil {
		return fmt.Errorf("acknowledge Space media event intent: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	return nil
}

func (s *PostgresAdmissionStore) ReleaseOutboxLease(ctx context.Context, item AdmissionOutboxItem) error {
	if !s.usable() {
		return ErrAdmissionUnavailable
	}
	tag, err := s.pool.Exec(ctx, `UPDATE voice_space_media_admission_outbox SET lease_token=NULL,lease_until=NULL
WHERE event_id=$1 AND operation_id=$2 AND lease_token=$3 AND delivered_at IS NULL`, item.ID, item.OperationID, item.LeaseToken)
	if err != nil {
		return fmt.Errorf("release Space media event lease: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrAdmissionConflict
	}
	return nil
}

func equalDigest(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for i := range left {
		difference |= left[i] ^ right[i]
	}
	return difference == 0
}
