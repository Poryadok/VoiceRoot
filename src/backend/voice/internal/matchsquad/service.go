package matchsquad

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/matchsquadprincipal"
	"voice/backend/voice/internal/store"
)

type RoomEffects interface {
	EnsureRoom(context.Context, string) error
	CloseRoom(context.Context, string) error
}

type Service struct {
	Pool    *pgxpool.Pool
	Calls   store.CallStore
	Effects RoomEffects
	Members interface {
		ReadyForTeardown(context.Context, uuid.UUID, uuid.UUID) error
		PrepareForTeardown(context.Context, uuid.UUID, uuid.UUID) error
		FinalizeAfterRoomAbsent(context.Context, uuid.UUID, uuid.UUID) error
	}
	Now func() time.Time
}

func (s *Service) Create(ctx context.Context, req *callsv1.CreateMatchSquadRoomRequest) ([]byte, error) {
	if err := matchsquadprincipal.RequireCreate(ctx, req); err != nil {
		return nil, err
	}
	operation, match, participants, manifest, chatReceipt, requestBytes, err := validateCreate(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad creation binding")
	}
	requestHash, err := principalHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad creation request hash")
	}
	chatReceiptBytes, err := marshal(chatReceipt)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Chat creation receipt")
	}
	chatReceiptHash := sha256.Sum256(chatReceiptBytes)
	if s == nil || s.Pool == nil || s.Calls == nil || s.Effects == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad provider unavailable")
	}
	roomID, err := uuid.NewRandom()
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad provider unavailable")
	}
	receiptID, err := uuid.NewRandom()
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad provider unavailable")
	}
	createdAt := s.now()
	receipt := &callsv1.MatchSquadRoomReceipt{
		ProtocolVersion: 1, ReceiptId: receiptID.String(), OperationId: operation.String(), MatchId: match.String(),
		RoomId: roomID.String(), ChatId: chatReceipt.GetChatId(), ChatCreationReceiptId: chatReceipt.GetReceiptId(),
		ChatCreationReceiptSha256: chatReceiptHash[:], ParticipantManifestSha256: manifest[:],
		RequestSha256: requestHash[:], CreatedAt: timestamppb.New(createdAt),
	}
	receiptBytes, err := marshal(receipt)
	if err != nil {
		return nil, status.Error(codes.Internal, "Voice MatchSquad receipt encoding failed")
	}
	roomID, receiptBytes, err = s.reserveCreate(ctx, operation, match, roomID, receiptID, match, chatReceipt, manifest, chatReceiptHash, requestHash, requestBytes, receiptBytes, createdAt)
	if err != nil {
		return nil, err
	}
	if err = s.applyCreate(ctx, roomID.String(), chatReceipt.GetChatId(), match, participants); err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad creation effects pending")
	}
	return receiptBytes, nil
}

func (s *Service) Teardown(ctx context.Context, req *callsv1.TeardownMatchSquadRoomRequest) ([]byte, error) {
	if err := matchsquadprincipal.RequireTeardown(ctx, req); err != nil {
		return nil, err
	}
	requestBytes, err := marshal(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad teardown request")
	}
	ids, err := parseUUIDs(req.GetTeardownOperationId(), req.GetMatchId(), req.GetRoomId(), req.GetCreationReceiptId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad teardown binding")
	}
	operation, match, room, creationReceipt := ids[0], ids[1], ids[2], ids[3]
	if req.GetProtocolVersion() != 1 || len(req.GetParticipantManifestSha256()) != sha256.Size || len(req.GetCreationRequestSha256()) != sha256.Size {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad teardown binding")
	}
	requestHash, err := principalHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad teardown request hash")
	}
	if s == nil || s.Pool == nil || s.Calls == nil || s.Effects == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad provider unavailable")
	}
	var teardownReceipt []byte
	err = s.beginTeardown(ctx, req, operation, match, room, creationReceipt, requestHash, requestBytes, &teardownReceipt)
	if err != nil || len(teardownReceipt) != 0 {
		return teardownReceipt, err
	}
	chatID, livekitRoom, resourceState, err := s.currentResourceBinding(ctx, room, match)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad resource state unavailable")
	}
	if resourceState != "closing" && resourceState != "closed" {
		return nil, status.Error(codes.FailedPrecondition, "MatchSquad resource is not in its current teardown state")
	}
	if s.Members == nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad member drain unavailable")
	}
	if err := s.Members.ReadyForTeardown(ctx, match, room); err != nil {
		return nil, err
	}
	if err := s.Members.PrepareForTeardown(ctx, match, room); err != nil {
		return nil, err
	}
	call, callErr := s.Calls.GetCall(ctx, room.String())
	if callErr == nil {
		if call.RoomID != room.String() || call.ChatID != chatID || call.LivekitRoomName != livekitRoom || call.MatchSquadMatchID != match.String() || (call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE && call.Status != callsv1.CallStatus_CALL_STATUS_ENDED) {
			return nil, status.Error(codes.FailedPrecondition, "MatchSquad projection diverged from current database state")
		}
		if call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			_, callErr = s.Calls.SetStatus(ctx, room.String(), callsv1.CallStatus_CALL_STATUS_ENDED, s.now())
		}
		if callErr != nil {
			return nil, status.Error(codes.Unavailable, "Voice MatchSquad teardown projection pending")
		}
	} else if !errors.Is(callErr, store.ErrNotFound) {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad teardown projection pending")
	}
	if err = s.Effects.CloseRoom(ctx, livekitRoom); err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad teardown media effect pending")
	}
	if err = s.Members.FinalizeAfterRoomAbsent(ctx, match, room); err != nil {
		return nil, err
	}
	completedAt := s.now()
	receiptID, err := uuid.NewRandom()
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad teardown receipt pending")
	}
	result := &callsv1.MatchSquadRoomTeardownReceipt{
		ProtocolVersion: 1, ReceiptId: receiptID.String(), TeardownOperationId: operation.String(),
		MatchId: match.String(), RoomId: room.String(), CreationReceiptId: creationReceipt.String(),
		ParticipantManifestSha256: req.GetParticipantManifestSha256(), RequestSha256: requestHash[:],
		Status: callsv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: timestamppb.New(completedAt),
	}
	teardownReceipt, err = marshal(result)
	if err != nil {
		return nil, status.Error(codes.Internal, "Voice MatchSquad teardown receipt encoding failed")
	}
	if err = s.completeTeardown(ctx, room, operation, receiptID, teardownReceipt, completedAt); err != nil {
		return nil, status.Error(codes.Unavailable, "Voice MatchSquad teardown commit pending")
	}
	return teardownReceipt, nil
}

func (s *Service) reserveCreate(ctx context.Context, operation, match, roomID, receiptID, owner uuid.UUID, chat *chatv1.MatchSquadChatReceipt, manifest, chatReceiptHash, requestHash [32]byte, request, receipt []byte, now time.Time) (uuid.UUID, []byte, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, operation.String()); err != nil {
		return uuid.Nil, nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	var prior []byte
	var priorReceipt []byte
	var priorRoom uuid.UUID
	var priorHash []byte
	var compacted bool
	err = tx.QueryRow(ctx, `SELECT room_id,create_request_bytes,create_receipt_bytes,create_request_sha256,compaction_operation_id IS NOT NULL FROM voice_match_squad_operations WHERE operation_id=$1 FOR UPDATE`, operation).Scan(&priorRoom, &prior, &priorReceipt, &priorHash, &compacted)
	if err == nil {
		if compacted {
			if !bytes.Equal(priorHash, requestHash[:]) {
				return uuid.Nil, nil, status.Error(codes.AlreadyExists, "MatchSquad operation id conflicts with another request")
			}
			return uuid.Nil, nil, status.Error(codes.FailedPrecondition, "MatchSquad creation evidence has been compacted")
		}
		if !bytes.Equal(prior, request) {
			return uuid.Nil, nil, status.Error(codes.AlreadyExists, "MatchSquad operation id conflicts with another request")
		}
		if err = tx.Commit(ctx); err != nil {
			return uuid.Nil, nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
		}
		return priorRoom, priorReceipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	created := now.UTC()
	_, err = tx.Exec(ctx, `INSERT INTO voice_room_instances(room_id,room_type,purpose,chat_id,owner_id,creation_operation_id,creation_manifest_hash,creation_receipt_id,chat_creation_receipt_id,livekit_room_name,state,roster_version,created_at,updated_at)
VALUES($1,'group_voice','MATCH_SQUAD',$2,$3,$4,$5,$6,$7,$8,'active',0,$9,$9)`, roomID, mustUUID(chat.GetChatId()), owner, operation, manifest[:], receiptID, mustUUID(chat.GetReceiptId()), "match-squad-"+roomID.String(), created)
	if err != nil {
		return uuid.Nil, nil, status.Error(codes.FailedPrecondition, "MatchSquad match or resource already exists")
	}
	_, err = tx.Exec(ctx, `INSERT INTO voice_match_squad_operations(operation_id,match_id,room_id,chat_id,owner_id,creation_receipt_id,chat_creation_receipt_id,participant_manifest_sha256,chat_creation_receipt_sha256,create_request_sha256,create_request_bytes,create_receipt_bytes,state,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'pending',$13,$13)`, operation, match, roomID, mustUUID(chat.GetChatId()), owner, receiptID, mustUUID(chat.GetReceiptId()), manifest[:], chatReceiptHash[:], requestHash[:], request, receipt, created)
	if err != nil {
		return uuid.Nil, nil, status.Error(codes.FailedPrecondition, "MatchSquad operation or match already exists")
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, nil, status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	return roomID, receipt, nil
}

func (s *Service) applyCreate(ctx context.Context, room, chatID string, match uuid.UUID, participants []string) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state string
	var operationState string
	err = tx.QueryRow(ctx, `SELECT r.state,o.state FROM voice_room_instances r JOIN voice_match_squad_operations o USING(room_id) WHERE r.room_id=$1 AND o.match_id=$2 FOR UPDATE OF r,o`, mustUUID(room), match).Scan(&state, &operationState)
	if err != nil {
		return err
	}
	if state != "active" || (operationState != "pending" && operationState != "active") {
		if state == "closing" || state == "closed" {
			return nil
		}
		return errors.New("MatchSquad resource is not current and active")
	}
	call := newMatchSquadProjection(room, chatID, match, participants[0], s.now())
	if _, err = s.Calls.CreateCall(ctx, call); err != nil {
		current, readErr := s.Calls.GetCall(ctx, room)
		if readErr != nil || !sameProjection(current, call) {
			return fmt.Errorf("MatchSquad Redis projection conflicts with durable resource: %w", err)
		}
	}
	if err = s.Effects.EnsureRoom(ctx, call.LivekitRoomName); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE voice_match_squad_operations SET state='active',updated_at=$2 WHERE room_id=$1 AND state='pending'`, mustUUID(room), s.now().UTC())
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// newMatchSquadProjection contains no active participants. The creation
// manifest is durable entitlement; only a verified member Join may create a
// current participant projection.
func newMatchSquadProjection(room, chatID string, match uuid.UUID, initiator string, now time.Time) store.Call {
	return store.Call{
		RoomID: room, LivekitRoomName: "match-squad-" + room, ChatID: chatID, MatchSquadMatchID: match.String(),
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		InitiatorProfileID: initiator, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status: callsv1.CallStatus_CALL_STATUS_ACTIVE, StartedAt: now, States: make(map[string]store.ParticipantState),
	}
}

func (s *Service) CheckSchema(ctx context.Context) error {
	if s == nil || s.Pool == nil {
		return errors.New("Voice MatchSquad database unavailable")
	}
	for _, query := range []string{
		`SELECT 1 FROM voice_room_instances LIMIT 0`,
		`SELECT compaction_operation_id,teardown_aggregate_id,compaction_request_sha256,compaction_request_bytes,compaction_receipt_id,compaction_receipt_bytes,teardown_receipt_sha256,aggregate_completed_at,compaction_authorized_at FROM voice_match_squad_operations LIMIT 0`,
	} {
		rows, err := s.Pool.Query(ctx, query)
		if err != nil {
			return errors.New("Voice MatchSquad database schema unavailable")
		}
		rows.Close()
		if rows.Err() != nil {
			return errors.New("Voice MatchSquad database schema unavailable")
		}
	}
	return nil
}

func (s *Service) currentResourceBinding(ctx context.Context, room, match uuid.UUID) (string, string, string, error) {
	var chatID, livekitRoom, state string
	err := s.Pool.QueryRow(ctx, `SELECT r.chat_id,r.livekit_room_name,r.state FROM voice_room_instances r JOIN voice_match_squad_operations o USING(room_id) WHERE r.room_id=$1 AND o.match_id=$2 AND r.purpose='MATCH_SQUAD' AND r.room_type='group_voice'`, room, match).Scan(&chatID, &livekitRoom, &state)
	return chatID, livekitRoom, state, err
}

func (s *Service) beginTeardown(ctx context.Context, req *callsv1.TeardownMatchSquadRoomRequest, operation, match, room, creation uuid.UUID, requestHash [32]byte, request []byte, replay *[]byte) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owner uuid.UUID
	var storedManifest, storedRequestHash, priorRequest, priorReceipt, priorTeardownHash []byte
	var state string
	var priorOperation *uuid.UUID
	var compacted bool
	err = tx.QueryRow(ctx, `SELECT owner_id,participant_manifest_sha256,create_request_sha256,state,teardown_operation_id,teardown_request_bytes,teardown_receipt_bytes,teardown_request_sha256,compaction_operation_id IS NOT NULL FROM voice_match_squad_operations WHERE room_id=$1 AND match_id=$2 AND creation_receipt_id=$3 FOR UPDATE`, room, match, creation).Scan(&owner, &storedManifest, &storedRequestHash, &state, &priorOperation, &priorRequest, &priorReceipt, &priorTeardownHash, &compacted)
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, "MatchSquad resource not found")
	}
	if err != nil {
		return status.Error(codes.Unavailable, "Voice MatchSquad database unavailable")
	}
	if owner != match || !bytes.Equal(storedManifest, req.GetParticipantManifestSha256()) || !bytes.Equal(storedRequestHash, req.GetCreationRequestSha256()) {
		return status.Error(codes.FailedPrecondition, "MatchSquad teardown binding mismatch")
	}
	if priorOperation != nil {
		if *priorOperation != operation {
			return status.Error(codes.AlreadyExists, "MatchSquad teardown operation conflicts")
		}
		if compacted && len(priorRequest) == 0 && len(priorReceipt) == 0 {
			if !bytes.Equal(priorTeardownHash, requestHash[:]) {
				return status.Error(codes.AlreadyExists, "MatchSquad teardown operation conflicts")
			}
			return status.Error(codes.FailedPrecondition, "MatchSquad teardown evidence has been compacted")
		}
		if !bytes.Equal(priorRequest, request) {
			return status.Error(codes.AlreadyExists, "MatchSquad teardown operation conflicts")
		}
		if state == "closed" && len(priorReceipt) > 0 {
			*replay = priorReceipt
			return tx.Commit(ctx)
		}
	} else {
		_, err = tx.Exec(ctx, `UPDATE voice_match_squad_operations SET state='closing',teardown_operation_id=$2,teardown_request_sha256=$3,teardown_request_bytes=$4 WHERE room_id=$1 AND state IN ('active','pending')`, room, operation, requestHash[:], request)
		if err != nil {
			return status.Error(codes.FailedPrecondition, "MatchSquad resource cannot enter teardown")
		}
		_, err = tx.Exec(ctx, `UPDATE voice_room_instances SET state='closing',updated_at=$2 WHERE room_id=$1 AND purpose='MATCH_SQUAD' AND state='active'`, room, s.now().UTC())
		if err != nil {
			return status.Error(codes.FailedPrecondition, "MatchSquad resource cannot enter teardown")
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) completeTeardown(ctx context.Context, room, operation, receiptID uuid.UUID, receipt []byte, now time.Time) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	command, err := tx.Exec(ctx, `UPDATE voice_room_instances SET state='closed',closed_at=$2,updated_at=$2 WHERE room_id=$1 AND purpose='MATCH_SQUAD' AND state='closing'`, room, now.UTC())
	if err != nil || command.RowsAffected() != 1 {
		return errors.New("MatchSquad room terminal transition rejected")
	}
	command, err = tx.Exec(ctx, `UPDATE voice_match_squad_operations SET state='closed',teardown_receipt_id=$2,teardown_receipt_bytes=$3,effects_confirmed_at=$4 WHERE room_id=$1 AND teardown_operation_id=$5 AND state='closing'`, room, receiptID, receipt, now.UTC(), operation)
	if err != nil || command.RowsAffected() != 1 {
		return errors.New("MatchSquad teardown receipt transition rejected")
	}
	return tx.Commit(ctx)
}

func validateCreate(req *callsv1.CreateMatchSquadRoomRequest) (uuid.UUID, uuid.UUID, []string, [32]byte, *chatv1.MatchSquadChatReceipt, []byte, error) {
	var empty [32]byte
	if req == nil || req.GetProtocolVersion() != 1 || len(req.GetParticipantManifestSha256()) != 32 || req.GetChatCreationReceipt() == nil || len(req.GetParticipants()) < 2 || len(req.GetParticipants()) > 64 {
		return uuid.Nil, uuid.Nil, nil, empty, nil, nil, errors.New("invalid request shape")
	}
	parsedIDs, err := parseUUIDs(req.GetOperationId(), req.GetMatchId())
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, empty, nil, nil, err
	}
	operation, match := parsedIDs[0], parsedIDs[1]
	participantIDs := make([]string, 0, len(req.GetParticipants()))
	manifestBytes := make([]byte, 0, 16*len(req.GetParticipants()))
	var previous uuid.UUID
	for i, participant := range req.GetParticipants() {
		if participant == nil {
			return uuid.Nil, uuid.Nil, nil, empty, nil, nil, errors.New("nil participant")
		}
		id, parseErr := uuid.Parse(participant.GetProfileId())
		if parseErr != nil || id == uuid.Nil || id.String() != participant.GetProfileId() || (i > 0 && bytes.Compare(previous[:], id[:]) >= 0) {
			return uuid.Nil, uuid.Nil, nil, empty, nil, nil, errors.New("participant order or id is not canonical")
		}
		previous = id
		participantIDs = append(participantIDs, id.String())
		manifestBytes = append(manifestBytes, id[:]...)
	}
	manifest := sha256.Sum256(manifestBytes)
	if !bytes.Equal(manifest[:], req.GetParticipantManifestSha256()) {
		return uuid.Nil, uuid.Nil, nil, empty, nil, nil, errors.New("participant manifest mismatch")
	}
	chat := req.GetChatCreationReceipt()
	chatIDs, err := parseUUIDs(chat.GetMatchId(), chat.GetOperationId(), chat.GetReceiptId(), chat.GetChatId())
	if err != nil || chat.GetProtocolVersion() != 1 || chatIDs[0] != match || !bytes.Equal(chat.GetParticipantManifestSha256(), manifest[:]) || len(chat.GetRequestSha256()) != sha256.Size {
		return uuid.Nil, uuid.Nil, nil, empty, nil, nil, errors.New("Chat receipt mismatch")
	}
	request, err := marshal(req)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, empty, nil, nil, err
	}
	return operation, match, participantIDs, manifest, chat, request, nil
}

func sameProjection(got, want store.Call) bool {
	if got.RoomID != want.RoomID || got.LivekitRoomName != want.LivekitRoomName || got.ChatID != want.ChatID || got.MatchSquadMatchID != want.MatchSquadMatchID || got.SessionKind != want.SessionKind || got.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE || len(got.States) != len(want.States) {
		return false
	}
	for id := range want.States {
		if _, ok := got.States[id]; !ok {
			return false
		}
	}
	return true
}

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func marshal(value proto.Message) ([]byte, error) {
	return (proto.MarshalOptions{Deterministic: true}).Marshal(value)
}
func principalHash(request proto.Message) ([32]byte, error) {
	encoded, err := principal.RequestHash(request)
	if err != nil {
		return [32]byte{}, err
	}
	var result [32]byte
	decoded, err := hex.DecodeString(strings.TrimPrefix(encoded, "sha256:"))
	if err != nil || len(decoded) != sha256.Size {
		return result, errors.New("invalid principal request hash")
	}
	copy(result[:], decoded)
	return result, nil
}
func parseUUIDs(values ...string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(values))
	for i, v := range values {
		id, err := uuid.Parse(v)
		if err != nil || id == uuid.Nil || id.String() != v {
			return nil, errors.New("noncanonical UUID")
		}
		ids[i] = id
	}
	return ids, nil
}
func mustUUID(value string) uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil
	}
	return id
}
