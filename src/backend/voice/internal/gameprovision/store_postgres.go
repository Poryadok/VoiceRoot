package gameprovision

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
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

type Room struct {
	RoomID, ChatID, LiveKitRoomName string
	CreatedAt                       time.Time
}

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) CheckSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return errors.New("game session store unavailable")
	}
	var ready bool
	if err := s.pool.QueryRow(ctx, `SELECT to_regclass('voice_game_session_operations') IS NOT NULL
AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('voice_room_instances') AND conname='voice_room_instances_purpose_shape')
AND (SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='voice_room_instances'
     AND column_name = ANY($1::text[])) = 6`, []string{"purpose", "room_type", "chat_id", "owner_id", "creation_operation_id", "chat_creation_receipt_id"}).Scan(&ready); err != nil {
		return err
	}
	if !ready {
		return errors.New("voice game session schema is missing")
	}
	return nil
}

func (s *PostgresStore) GetRoom(ctx context.Context, roomID string) (Room, error) {
	if s == nil || s.pool == nil {
		return Room{}, errors.New("game session store unavailable")
	}
	id, err := uuid.Parse(roomID)
	if err != nil || id == uuid.Nil || id.String() != roomID {
		return Room{}, ErrNotFound
	}
	var room Room
	err = s.pool.QueryRow(ctx, `SELECT r.room_id::text,r.chat_id::text,r.livekit_room_name,r.created_at
FROM voice_game_session_operations o
JOIN voice_room_instances r ON r.room_id=o.room_id
WHERE r.room_id=$1 AND r.purpose='GAME_SESSION' AND r.state='active'`, id).Scan(&room.RoomID, &room.ChatID, &room.LiveKitRoomName, &room.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Room{}, ErrNotFound
	}
	if err != nil {
		return Room{}, err
	}
	return room, nil
}

func (s *PostgresStore) Provision(ctx context.Context, req *callsv1.ProvisionGameSessionRoomRequest) (*callsv1.ProvisionGameSessionRoomResponse, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("game session store unavailable")
	}
	app, env, op, chat, chatOp, err := validate(req)
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
		OperationId: op.String(), ApplicationId: app.String(), EnvironmentId: env.String(),
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
(operation_id,application_id,environment_id,resource_kind,external_resource_key,request_hash,chat_id,chat_creation_operation_id,room_id,voice_creation_receipt_id,response_bytes,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, op, app, env, int16(req.Resource.Kind), req.Resource.ExternalResourceKey, hash[:], chat, chatOp, roomID, receiptID, responseBytes, createdAt)
	if err != nil {
		return nil, normalizeConflict(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

func validate(req *callsv1.ProvisionGameSessionRoomRequest) (app, env, op, chat, chatOp uuid.UUID, err error) {
	if req == nil || req.Resource == nil {
		err = errors.New("game session request and resource are required")
		return
	}
	for _, item := range []struct {
		name, value string
		target      *uuid.UUID
	}{
		{"application_id", req.ApplicationId, &app}, {"environment_id", req.EnvironmentId, &env}, {"operation_id", req.OperationId, &op},
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
