package gameprovision

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	callsv1 "voice.app/voice/calls/v1"
)

var ErrConflict = errors.New("game session provisioning conflict")
var ErrInvalidRequest = errors.New("invalid game session provisioning request")
var ErrNotFound = errors.New("managed game session room not found")
var ErrFenceTimeout = errors.New("managed game session media fence exceeded five seconds")

type ManagedGameSessionMediaFencer interface {
	FenceManagedGameSession(context.Context, string, string) error
}

type Room struct {
	RoomID, ChatID, LiveKitRoomName string
	ApplicationID, EnvironmentID    string
	SessionID                       string
	CreatedAt                       time.Time
	RosterRevision                  uint64
	ProfileIDs                      []string
	LeaseExpiresAt                  *time.Time
}

type ManagedGameSessionRosterMediaFencer interface {
	FenceManagedGameSessionMembers(context.Context, string, string, []string) error
}

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) CheckSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return errors.New("game session store unavailable")
	}
	var ready bool
	if err := s.pool.QueryRow(ctx, `SELECT to_regclass('voice_game_session_operations') IS NOT NULL
AND to_regclass('voice_game_session_closures') IS NOT NULL
AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema()
 AND table_name='voice_game_session_operations' AND column_name='session_id')
AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('voice_room_instances') AND conname='voice_room_instances_purpose_shape')
AND to_regclass('voice_game_session_roster_receipts') IS NOT NULL
AND (SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='voice_room_instances'
     AND column_name = ANY($1::text[])) = 6
AND (SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='voice_game_session_operations'
     AND column_name = ANY($2::text[])) = 5`, []string{"purpose", "room_type", "chat_id", "owner_id", "creation_operation_id", "chat_creation_receipt_id"}, []string{"roster_revision", "roster_profile_ids", "roster_body_hash", "lease_expires_at", "lease_fenced_at"}).Scan(&ready); err != nil {
		return err
	}
	if !ready {
		return errors.New("voice game session schema is missing")
	}
	return nil
}

func (s *PostgresStore) GetRoom(ctx context.Context, roomID string) (Room, error) {
	return s.getRoom(ctx, roomID, nil)
}

// GetRoomAt evaluates the lease against a supplied clock for deterministic
// admission-boundary tests and storage consumers with an injected clock.
func (s *PostgresStore) GetRoomAt(ctx context.Context, roomID string, now time.Time) (Room, error) {
	now = now.UTC()
	return s.getRoom(ctx, roomID, &now)
}

func (s *PostgresStore) getRoom(ctx context.Context, roomID string, now *time.Time) (Room, error) {
	if s == nil || s.pool == nil {
		return Room{}, errors.New("game session store unavailable")
	}
	id, err := uuid.Parse(roomID)
	if err != nil || id == uuid.Nil || id.String() != roomID {
		return Room{}, ErrNotFound
	}
	var room Room
	leasePredicate := "(o.lease_expires_at IS NULL OR o.lease_expires_at > clock_timestamp())"
	args := []any{id}
	if now != nil {
		leasePredicate = "(o.lease_expires_at IS NULL OR o.lease_expires_at > $2)"
		args = append(args, *now)
	}
	query := `SELECT r.room_id::text,r.chat_id::text,r.livekit_room_name,
o.application_id::text,o.environment_id::text,o.session_id::text,r.created_at
 ,o.roster_revision,o.roster_profile_ids,o.lease_expires_at
FROM voice_game_session_operations o
JOIN voice_room_instances r ON r.room_id=o.room_id
WHERE r.room_id=$1 AND r.purpose='GAME_SESSION' AND r.state='active' AND o.session_id IS NOT NULL AND ` + leasePredicate
	err = s.pool.QueryRow(ctx, query, args...).Scan(
		&room.RoomID, &room.ChatID, &room.LiveKitRoomName, &room.ApplicationID, &room.EnvironmentID, &room.SessionID, &room.CreatedAt,
		&room.RosterRevision, &room.ProfileIDs, &room.LeaseExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Room{}, ErrNotFound
	}
	if err != nil {
		return Room{}, err
	}
	return room, nil
}

// ApplyGameSessionRoster stores a complete, GIS-authenticated roster snapshot.
// Same-revision retries are inert, and removed participants are fenced after
// the durable roster commit before this method returns.
func (s *PostgresStore) ApplyGameSessionRoster(ctx context.Context, req *callsv1.ApplyGameSessionRosterRequest, fencer ManagedGameSessionRosterMediaFencer) (*callsv1.ApplyGameSessionRosterResponse, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("game session store unavailable")
	}
	app, env, op, session, roomID, revision, profiles, deadline, err := validateRoster(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal game session roster request: %w", err)
	}
	hash := sha256.Sum256(requestBytes)
	body := proto.Clone(req).(*callsv1.ApplyGameSessionRosterRequest)
	body.OperationId = ""
	bodyBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal game session roster body: %w", err)
	}
	bodyHash := sha256.Sum256(bodyBytes)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7271843201)`); err != nil {
		return nil, err
	}
	var savedHash, responseBytes []byte
	var removedIDs []string
	var fenceDone bool
	err = tx.QueryRow(ctx, `SELECT request_hash,response_bytes,removed_profile_ids,media_fenced_at IS NOT NULL
		FROM voice_game_session_roster_receipts WHERE operation_id=$1 FOR UPDATE`, op).Scan(&savedHash, &responseBytes, &removedIDs, &fenceDone)
	if err == nil {
		if !equalHash(savedHash, hash[:]) {
			return nil, ErrConflict
		}
		response := new(callsv1.ApplyGameSessionRosterResponse)
		if err := proto.Unmarshal(responseBytes, response); err != nil {
			return nil, fmt.Errorf("stored roster response is corrupt: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		if len(removedIDs) != 0 && !fenceDone {
			if err := s.fenceRemovedMembersSafely(ctx, fencer, roomID, op, removedIDs); err != nil {
				return nil, err
			}
		}
		return response, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var storedApp, storedEnv, storedSession string
	var storedRevision int64
	var oldProfiles []string
	var oldBodyHash []byte
	var oldDeadline *time.Time
	err = tx.QueryRow(ctx, `SELECT o.application_id::text,o.environment_id::text,o.session_id::text,o.roster_revision,o.roster_profile_ids,o.roster_body_hash,o.lease_expires_at
		FROM voice_game_session_operations o JOIN voice_room_instances r ON r.room_id=o.room_id
		WHERE r.room_id=$1 AND r.purpose='GAME_SESSION' AND r.state='active' FOR UPDATE`, roomID).Scan(&storedApp, &storedEnv, &storedSession, &storedRevision, &oldProfiles, &oldBodyHash, &oldDeadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if storedApp != app.String() || storedEnv != env.String() || storedSession != session.String() {
		return nil, ErrNotFound
	}
	acceptedRevision, acceptedDeadline := uint64(storedRevision), oldDeadline
	removedIDs = []string{}
	if int64(revision) == storedRevision {
		if storedRevision != 0 && !equalHash(oldBodyHash, bodyHash[:]) {
			return nil, ErrConflict
		}
	} else if int64(revision) < storedRevision {
		// A stale complete snapshot gets a durable no-op receipt.
	} else {
		removedIDs = difference(oldProfiles, profiles)
		if _, err := tx.Exec(ctx, `UPDATE voice_game_session_operations SET roster_revision=$2,roster_profile_ids=$3::uuid[],roster_body_hash=$4,lease_expires_at=$5,lease_fenced_at=NULL WHERE room_id=$1`, roomID, int64(revision), profiles, bodyHash[:], deadline); err != nil {
			return nil, err
		}
		acceptedRevision, acceptedDeadline = revision, &deadline
	}
	if acceptedDeadline == nil {
		return nil, ErrInvalidRequest
	}
	receiptID := uuid.New()
	response := &callsv1.ApplyGameSessionRosterResponse{OperationId: op.String(), ApplicationId: app.String(), EnvironmentId: env.String(), SessionId: session.String(), VoiceRoomId: roomID.String(), ReceiptId: receiptID.String(), RequestHash: append([]byte(nil), hash[:]...), AcceptedRevision: acceptedRevision, LeaseExpiresAt: timestamppb.New(*acceptedDeadline)}
	responseBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO voice_game_session_roster_receipts(operation_id,room_id,request_hash,request_bytes,receipt_id,response_bytes,accepted_revision,lease_expires_at,removed_profile_ids)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::uuid[])`, op, roomID, hash[:], requestBytes, receiptID, responseBytes, int64(acceptedRevision), acceptedDeadline, removedIDs)
	if err != nil {
		return nil, normalizeConflict(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if len(removedIDs) != 0 {
		if err := s.fenceRemovedMembersSafely(ctx, fencer, roomID, op, removedIDs); err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (s *PostgresStore) fenceRemovedMembersSafely(ctx context.Context, fencer ManagedGameSessionRosterMediaFencer, roomID, operationID uuid.UUID, removedIDs []string) error {
	if fencer == nil {
		return errors.New("managed game session roster media fencer unavailable")
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(7271843201)`); err != nil {
		return err
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, `SELECT pg_advisory_unlock(7271843201)`)
	}()
	var current []string
	if err := conn.QueryRow(ctx, `SELECT roster_profile_ids FROM voice_game_session_operations WHERE room_id=$1`, roomID).Scan(&current); err != nil {
		return err
	}
	stillRemoved := make([]string, 0, len(removedIDs))
	for _, removed := range removedIDs {
		if !containsString(current, removed) {
			stillRemoved = append(stillRemoved, removed)
		}
	}
	if len(stillRemoved) != 0 {
		fenceCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		err := fencer.FenceManagedGameSessionMembers(fenceCtx, roomID.String(), operationID.String(), stillRemoved)
		cancel()
		if err != nil {
			return fmt.Errorf("fence removed game session members: %w", err)
		}
	}
	if _, err := conn.Exec(ctx, `UPDATE voice_game_session_roster_receipts SET media_fenced_at=clock_timestamp() WHERE operation_id=$1 AND media_fenced_at IS NULL`, operationID); err != nil {
		return err
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validateRoster(req *callsv1.ApplyGameSessionRosterRequest) (app, env, op, session, roomID uuid.UUID, revision uint64, profiles []string, deadline time.Time, err error) {
	if req == nil || req.LeaseExpiresAt == nil {
		err = errors.New("request and lease deadline are required")
		return
	}
	for _, item := range []struct {
		name, value string
		target      *uuid.UUID
	}{{"application_id", req.ApplicationId, &app}, {"environment_id", req.EnvironmentId, &env}, {"operation_id", req.OperationId, &op}, {"session_id", req.SessionId, &session}, {"voice_room_id", req.VoiceRoomId, &roomID}} {
		parsed, parseErr := uuid.Parse(item.value)
		if parseErr != nil || parsed == uuid.Nil || parsed.String() != item.value {
			err = fmt.Errorf("%s must be a canonical non-nil UUID", item.name)
			return
		}
		*item.target = parsed
	}
	if req.RosterRevision == 0 || req.RosterRevision > uint64(^uint64(0)>>1) {
		err = errors.New("roster_revision must be positive and fit PostgreSQL BIGINT")
		return
	}
	revision = req.RosterRevision
	if len(req.ProfileIds) > 10000 {
		err = errors.New("profile roster is too large")
		return
	}
	profiles = append([]string(nil), req.ProfileIds...)
	for i, profile := range profiles {
		parsed, parseErr := uuid.Parse(profile)
		if parseErr != nil || parsed == uuid.Nil || parsed.String() != profile || (i > 0 && profiles[i-1] >= profile) {
			err = errors.New("profile_ids must be sorted, unique canonical UUIDs")
			return
		}
	}
	if err = req.LeaseExpiresAt.CheckValid(); err != nil {
		return
	}
	deadline = req.LeaseExpiresAt.AsTime().UTC()
	return
}

func difference(old, next []string) []string {
	removed := make([]string, 0)
	j := 0
	for _, id := range old {
		for j < len(next) && next[j] < id {
			j++
		}
		if j == len(next) || next[j] != id {
			removed = append(removed, id)
		}
	}
	return removed
}

// GetRoomForClose resolves the immutable LiveKit room name after the durable
// owner row has entered CLOSING; it is never an admission projection.
func (s *PostgresStore) GetRoomForClose(ctx context.Context, roomID string) (Room, error) {
	if s == nil || s.pool == nil {
		return Room{}, errors.New("game session store unavailable")
	}
	id, err := uuid.Parse(roomID)
	if err != nil || id == uuid.Nil || id.String() != roomID {
		return Room{}, ErrNotFound
	}
	var room Room
	err = s.pool.QueryRow(ctx, `SELECT r.room_id::text,r.chat_id::text,r.livekit_room_name,
o.application_id::text,o.environment_id::text,o.session_id::text,r.created_at
FROM voice_game_session_operations o JOIN voice_room_instances r ON r.room_id=o.room_id
WHERE r.room_id=$1 AND r.purpose='GAME_SESSION' AND o.session_id IS NOT NULL`, id).Scan(
		&room.RoomID, &room.ChatID, &room.LiveKitRoomName, &room.ApplicationID, &room.EnvironmentID, &room.SessionID, &room.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Room{}, ErrNotFound
	}
	if err != nil {
		return Room{}, err
	}
	return room, nil
}

// FenceExpiredGameSessionLeases ejects participants from expired active rooms.
// The receipt marker is written only after the LiveKit fence succeeds, so a
// restarted worker retries any interrupted fence.
func (s *PostgresStore) FenceExpiredGameSessionLeases(ctx context.Context, fencer ManagedGameSessionRosterMediaFencer) error {
	return s.FenceExpiredGameSessionLeasesAt(ctx, fencer, time.Now().UTC())
}

func (s *PostgresStore) FenceExpiredGameSessionLeasesAt(ctx context.Context, fencer ManagedGameSessionRosterMediaFencer, now time.Time) error {
	if s == nil || s.pool == nil || fencer == nil {
		return errors.New("game session lease fencer unavailable")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7271843201)`); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT o.room_id::text,o.roster_revision,o.roster_profile_ids
		FROM voice_game_session_operations o JOIN voice_room_instances r ON r.room_id=o.room_id
		WHERE r.purpose='GAME_SESSION' AND r.state='active' AND o.lease_expires_at <= $1
		AND o.lease_fenced_at IS NULL ORDER BY o.lease_expires_at LIMIT 100`, now.UTC())
	if err != nil {
		return err
	}
	type expiredRoom struct {
		id       string
		revision int64
		profiles []string
	}
	var expired []expiredRoom
	for rows.Next() {
		var room expiredRoom
		if err := rows.Scan(&room.id, &room.revision, &room.profiles); err != nil {
			rows.Close()
			return err
		}
		expired = append(expired, room)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	type fenceResult struct {
		room expiredRoom
		err  error
	}
	results := make(chan fenceResult, len(expired))
	var wg sync.WaitGroup
	for _, room := range expired {
		wg.Add(1)
		go func(room expiredRoom) {
			defer wg.Done()
			fenceCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			err := fencer.FenceManagedGameSessionMembers(fenceCtx, room.id, fmt.Sprintf("lease:%s:%d", room.id, room.revision), room.profiles)
			cancel()
			results <- fenceResult{room: room, err: err}
		}(room)
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result.err != nil {
			return fmt.Errorf("fence expired game session %s: %w", result.room.id, result.err)
		}
		if _, err := tx.Exec(ctx, `UPDATE voice_game_session_operations SET lease_fenced_at=clock_timestamp()
			WHERE room_id=$1 AND roster_revision=$2 AND lease_expires_at <= $3 AND lease_fenced_at IS NULL`, result.room.id, result.room.revision, now.UTC()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) Provision(ctx context.Context, req *callsv1.ProvisionGameSessionRoomRequest) (*callsv1.ProvisionGameSessionRoomResponse, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("game session store unavailable")
	}
	app, env, op, session, chat, chatOp, err := validate(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal game session request: %w", err)
	}
	hash := sha256.Sum256(requestBytes)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// A single transaction-scoped lock makes operation and resource uniqueness
	// checks atomic even when two different operation IDs race for one resource.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7271843201)`); err != nil {
		return nil, err
	}
	var oldHash, responseBytes []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response_bytes FROM voice_game_session_operations WHERE operation_id=$1`, op).Scan(&oldHash, &responseBytes)
	if err == nil {
		if !equalHash(oldHash, hash[:]) {
			return nil, ErrConflict
		}
		response := new(callsv1.ProvisionGameSessionRoomResponse)
		if err := proto.Unmarshal(responseBytes, response); err != nil {
			return nil, fmt.Errorf("stored game session response is corrupt: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return response, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_game_session_operations WHERE application_id=$1 AND environment_id=$2 AND resource_kind=$3 AND external_resource_key=$4)`, app, env, int16(req.Resource.Kind), req.Resource.ExternalResourceKey).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrConflict
	}
	roomID, receiptID := uuid.New(), uuid.New()
	roomName := "voice-game-session-" + roomID.String()
	createdAt := time.Now().UTC()
	response := &callsv1.ProvisionGameSessionRoomResponse{
		OperationId: op.String(), ApplicationId: app.String(), EnvironmentId: env.String(), SessionId: session.String(),
		Resource: proto.Clone(req.Resource).(*callsv1.GameSessionResourceRef), ChatId: chat.String(),
		ChatCreationOperationId: chatOp.String(), RoomId: roomID.String(), LivekitRoomName: roomName,
		VoiceCreationReceiptId: receiptID.String(), RequestHash: append([]byte(nil), hash[:]...), CreatedAt: timestamppb.New(createdAt),
	}
	responseBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO voice_room_instances
(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at,closed_at,room_type,purpose,chat_id,owner_id,creation_operation_id,creation_manifest_hash,creation_receipt_id,chat_creation_receipt_id)
VALUES($1,NULL,NULL,$2,'active',0,$3,$3,NULL,'group_voice','GAME_SESSION',$4,NULL,$5,$6,$7,$8)`,
		roomID, roomName, createdAt, chat, op, hash[:], receiptID, chatOp)
	if err != nil {
		return nil, normalizeConflict(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO voice_game_session_operations
(operation_id,application_id,environment_id,resource_kind,external_resource_key,request_hash,chat_id,chat_creation_operation_id,room_id,voice_creation_receipt_id,response_bytes,created_at,session_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, op, app, env, int16(req.Resource.Kind), req.Resource.ExternalResourceKey, hash[:], chat, chatOp, roomID, receiptID, responseBytes, createdAt, session)
	if err != nil {
		return nil, normalizeConflict(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

// CloseGameSessionRoom commits CLOSING before fencing media. Only the winning
// operation may finish the one durable CLOSING -> CLOSED transition.
func (s *PostgresStore) CloseGameSessionRoom(ctx context.Context, req *callsv1.CloseGameSessionRoomRequest, fencer ManagedGameSessionMediaFencer) (*callsv1.CloseGameSessionRoomResponse, error) {
	if s == nil || s.pool == nil || fencer == nil {
		return nil, errors.New("game session close dependencies unavailable")
	}
	app, env, op, session, chat, chatOp, err := validateClose(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal game session close request: %w", err)
	}
	hash := sha256.Sum256(requestBytes)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7271843201)`); err != nil {
		return nil, err
	}
	var savedHash, responseBytes []byte
	var roomID, receiptID uuid.UUID
	var state string
	var closingAt time.Time
	err = tx.QueryRow(ctx, `SELECT request_hash,response_bytes,room_id,close_receipt_id,status,closing_at
		FROM voice_game_session_closures WHERE operation_id=$1 FOR UPDATE`, op).Scan(
		&savedHash, &responseBytes, &roomID, &receiptID, &state, &closingAt)
	if err == nil {
		if !equalHash(savedHash, hash[:]) {
			return nil, ErrConflict
		}
		if state == "CLOSED" {
			response := new(callsv1.CloseGameSessionRoomResponse)
			if err := proto.Unmarshal(responseBytes, response); err != nil {
				return nil, fmt.Errorf("stored game session close response is corrupt: %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return response, nil
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT room_id FROM voice_game_session_operations
			WHERE application_id=$1 AND environment_id=$2 AND resource_kind=$3 AND external_resource_key=$4
			AND chat_id=$5 AND chat_creation_operation_id=$6 AND session_id=$7 FOR UPDATE`,
			app, env, int16(req.Resource.Kind), req.Resource.ExternalResourceKey, chat, chatOp, session).Scan(&roomID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		var otherOperation uuid.UUID
		err = tx.QueryRow(ctx, `SELECT operation_id FROM voice_game_session_closures WHERE room_id=$1 FOR UPDATE`, roomID).Scan(&otherOperation)
		if err == nil {
			return nil, ErrConflict
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		receiptID, closingAt = uuid.New(), time.Now().UTC()
		_, err = tx.Exec(ctx, `INSERT INTO voice_game_session_closures
			(operation_id,room_id,request_hash,request_bytes,status,close_receipt_id,closing_at)
			VALUES($1,$2,$3,$4,'CLOSING',$5,$6)`, op, roomID, hash[:], requestBytes, receiptID, closingAt)
		if err != nil {
			return nil, normalizeConflict(err)
		}
		command, err := tx.Exec(ctx, `UPDATE voice_room_instances SET state='closing',updated_at=$2
			WHERE room_id=$1 AND purpose='GAME_SESSION' AND state='active'`, roomID, closingAt)
		if err != nil || command.RowsAffected() != 1 {
			if err != nil {
				return nil, err
			}
			return nil, ErrConflict
		}
	} else if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	startedFence := time.Now().UTC()
	if err := fencer.FenceManagedGameSession(ctx, roomID.String(), op.String()); err != nil {
		return nil, fmt.Errorf("fence managed game session media: %w", err)
	}
	mediaFencedAt := time.Now().UTC()
	if mediaFencedAt.Sub(startedFence) > 5*time.Second {
		return nil, ErrFenceTimeout
	}
	if mediaFencedAt.Before(startedFence) {
		mediaFencedAt = startedFence
	}
	closedAt := time.Now().UTC()
	response := &callsv1.CloseGameSessionRoomResponse{
		OperationId: op.String(), ApplicationId: app.String(), EnvironmentId: env.String(),
		Resource: proto.Clone(req.Resource).(*callsv1.GameSessionResourceRef), ChatId: chat.String(),
		ChatCreationOperationId: chatOp.String(), SessionId: session.String(), RoomId: roomID.String(),
		Status: "CLOSED", CloseReceiptId: receiptID.String(), RequestHash: append([]byte(nil), hash[:]...),
		ClosingAt: timestamppb.New(closingAt), MediaFencedAt: timestamppb.New(mediaFencedAt), ClosedAt: timestamppb.New(closedAt),
	}
	responseBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, err
	}
	finish, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = finish.Rollback(ctx) }()
	if _, err := finish.Exec(ctx, `SELECT pg_advisory_xact_lock(7271843201)`); err != nil {
		return nil, err
	}
	command, err := finish.Exec(ctx, `UPDATE voice_game_session_closures SET status='CLOSED',media_fenced_at=$2,
		closed_at=$3,response_bytes=$4 WHERE operation_id=$1 AND status='CLOSING' AND request_hash=$5`,
		op, mediaFencedAt, closedAt, responseBytes, hash[:])
	if err != nil || command.RowsAffected() != 1 {
		if err != nil {
			return nil, err
		}
		return nil, ErrConflict
	}
	command, err = finish.Exec(ctx, `UPDATE voice_room_instances SET state='closed',closed_at=$2,updated_at=$2
		WHERE room_id=$1 AND state='closing'`, roomID, closedAt)
	if err != nil || command.RowsAffected() != 1 {
		if err != nil {
			return nil, err
		}
		return nil, ErrConflict
	}
	if err := finish.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

func validateClose(req *callsv1.CloseGameSessionRoomRequest) (app, env, op, session, chat, chatOp uuid.UUID, err error) {
	if req == nil || req.Resource == nil {
		return app, env, op, session, chat, chatOp, errors.New("game session close request and resource are required")
	}
	for _, item := range []struct {
		name, value string
		target      *uuid.UUID
	}{
		{"application_id", req.ApplicationId, &app}, {"environment_id", req.EnvironmentId, &env},
		{"operation_id", req.OperationId, &op}, {"session_id", req.SessionId, &session},
		{"chat_id", req.ChatId, &chat}, {"chat_creation_operation_id", req.ChatCreationOperationId, &chatOp},
	} {
		parsed, parseErr := uuid.Parse(item.value)
		if parseErr != nil || parsed == uuid.Nil {
			return app, env, op, session, chat, chatOp, fmt.Errorf("%s must be a non-nil UUID", item.name)
		}
		*item.target = parsed
	}
	if req.Resource.Kind < callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_PARTY ||
		req.Resource.Kind > callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_FLEET_SESSION ||
		len(req.Resource.ExternalResourceKey) == 0 || len(req.Resource.ExternalResourceKey) > 512 {
		return app, env, op, session, chat, chatOp, errors.New("resource identity is invalid")
	}
	return app, env, op, session, chat, chatOp, nil
}

func validate(req *callsv1.ProvisionGameSessionRoomRequest) (app, env, op, session, chat, chatOp uuid.UUID, err error) {
	if req == nil || req.Resource == nil {
		err = errors.New("game session request and resource are required")
		return
	}
	for _, item := range []struct {
		name, value string
		target      *uuid.UUID
	}{
		{"application_id", req.ApplicationId, &app}, {"environment_id", req.EnvironmentId, &env}, {"operation_id", req.OperationId, &op}, {"session_id", req.SessionId, &session},
		{"chat_id", req.ChatId, &chat}, {"chat_creation_operation_id", req.ChatCreationOperationId, &chatOp},
	} {
		parsed, parseErr := uuid.Parse(item.value)
		if parseErr != nil || parsed == uuid.Nil {
			err = fmt.Errorf("%s must be a non-nil UUID", item.name)
			return
		}
		*item.target = parsed
	}
	if req.Resource.Kind < callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_PARTY || req.Resource.Kind > callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_FLEET_SESSION {
		err = errors.New("resource kind is not supported")
		return
	}
	key := req.Resource.ExternalResourceKey
	if len(key) == 0 || len(key) > 512 || strings.TrimSpace(key) == "" || strings.IndexFunc(key, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		err = errors.New("external resource key is invalid")
		return
	}
	return
}

func equalHash(a, b []byte) bool { return len(a) == len(b) && string(a) == string(b) }
func normalizeConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
