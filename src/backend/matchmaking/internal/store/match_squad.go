package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

var (
	ErrMatchSquadConflict = errors.New("match squad operation conflicts with durable state")
	ErrMatchSquadPending  = errors.New("match squad operation is not ready for this transition")
)

// MatchSquadProvisioningOperation contains immutable service requests and the
// receipts observed so far. It is the durable retry source for provider calls.
type MatchSquadProvisioningOperation struct {
	MatchID                  uuid.UUID
	OperationID              uuid.UUID
	ParticipantManifestHash  []byte
	ParticipantManifestBytes []byte
	State                    string
	ChatOperationID          uuid.UUID
	ChatRequestHash          []byte
	ChatRequestBytes         []byte
	ChatReceiptID            *uuid.UUID
	ChatReceiptBytes         []byte
	ChatID                   *uuid.UUID
	VoiceOperationID         uuid.UUID
	VoiceRequestHash         []byte
	VoiceRequestBytes        []byte
	VoiceReceiptID           *uuid.UUID
	VoiceReceiptBytes        []byte
	VoiceRoomID              *uuid.UUID
}

type MatchSquadProvisioningIntent struct {
	MatchID                  uuid.UUID
	OperationID              uuid.UUID
	ParticipantManifestHash  []byte
	ParticipantManifestBytes []byte
	ChatOperationID          uuid.UUID
	ChatRequestHash          []byte
	ChatRequestBytes         []byte
	VoiceOperationID         uuid.UUID
}

func (s *MatchStore) GetMatchSquadProvisioningOperation(ctx context.Context, matchID uuid.UUID) (MatchSquadProvisioningOperation, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadProvisioningOperation{}, errors.New("match store unavailable")
	}
	operation, err := scanMatchSquadProvisioningOperation(s.Pool.QueryRow(ctx, `
		SELECT match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes, state,
		       chat_operation_id, chat_request_sha256, chat_request_bytes, chat_receipt_id, chat_receipt_bytes, chat_id,
		       voice_operation_id, voice_request_sha256, voice_request_bytes, voice_receipt_id, voice_receipt_bytes, voice_room_id
		FROM matchmaking_match_squad_operations WHERE match_id=$1
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadPending
	}
	return operation, err
}

// SaveMatchSquadProvisioningIntent persists the first provider request before
// any RPC. It accepts only the exact, locked, all-accepted match manifest.
func (s *MatchStore) SaveMatchSquadProvisioningIntent(ctx context.Context, intent MatchSquadProvisioningIntent) (MatchSquadProvisioningOperation, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadProvisioningOperation{}, errors.New("match store unavailable")
	}
	if intent.MatchID == uuid.Nil || intent.OperationID == uuid.Nil || intent.ChatOperationID == uuid.Nil || intent.VoiceOperationID == uuid.Nil ||
		len(intent.ParticipantManifestBytes) == 0 || len(intent.ChatRequestBytes) == 0 ||
		!bytes.Equal(sumSHA256(intent.ParticipantManifestBytes), intent.ParticipantManifestHash) ||
		!bytes.Equal(sumSHA256(intent.ChatRequestBytes), intent.ChatRequestHash) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	match, err := scanMatch(tx.QueryRow(ctx, `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches WHERE id = $1 FOR UPDATE
	`, intent.MatchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadProvisioningOperation{}, ErrMatchNotFound
	}
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	if match.Status != MatchStatusPendingAccept {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadPending
	}
	proposals, err := loadMatchProposalsForUpdate(ctx, tx, intent.MatchID)
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	sessions, err := loadMatchSessionsForUpdate(ctx, tx, intent.MatchID)
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	if len(proposals) == 0 || len(proposals) != len(match.Participants) || len(sessions) != len(proposals) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	for _, proposal := range proposals {
		if proposal.Response != ProposalResponseAccepted {
			return MatchSquadProvisioningOperation{}, ErrMatchSquadPending
		}
	}
	profileIDs := match.ProfileIDs()
	sort.Slice(profileIDs, func(i, j int) bool { return bytes.Compare(profileIDs[i][:], profileIDs[j][:]) < 0 })
	canonicalManifest := make([]byte, 0, len(profileIDs)*16)
	for _, profileID := range profileIDs {
		canonicalManifest = append(canonicalManifest, profileID[:]...)
	}
	if !bytes.Equal(canonicalManifest, intent.ParticipantManifestBytes) || !bytes.Equal(sumSHA256(canonicalManifest), intent.ParticipantManifestHash) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	chatRequest := new(chatv1.CreateMatchSquadChatRequest)
	if err := proto.Unmarshal(intent.ChatRequestBytes, chatRequest); err != nil || len(chatRequest.ProtoReflect().GetUnknown()) != 0 ||
		chatRequest.GetProtocolVersion() != 1 || chatRequest.GetOperationId() != intent.ChatOperationID.String() ||
		chatRequest.GetMatchId() != intent.MatchID.String() || !bytes.Equal(chatRequest.GetParticipantManifestSha256(), intent.ParticipantManifestHash) ||
		len(chatRequest.GetParticipants()) != len(profileIDs) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	for i, profileID := range profileIDs {
		if chatRequest.GetParticipants()[i] == nil || len(chatRequest.GetParticipants()[i].ProtoReflect().GetUnknown()) != 0 || chatRequest.GetParticipants()[i].GetProfileId() != profileID.String() {
			return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
		}
	}
	canonicalChatRequest, err := (proto.MarshalOptions{Deterministic: true}).Marshal(chatRequest)
	if err != nil || !bytes.Equal(canonicalChatRequest, intent.ChatRequestBytes) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_operations
		(match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes,
		 state, chat_operation_id, chat_request_sha256, chat_request_bytes, voice_operation_id)
		VALUES ($1,$2,$3,$4,'provisioning',$5,$6,$7,$8)
		ON CONFLICT (match_id) DO NOTHING
	`, intent.MatchID, intent.OperationID, intent.ParticipantManifestHash, intent.ParticipantManifestBytes,
		intent.ChatOperationID, intent.ChatRequestHash, intent.ChatRequestBytes, intent.VoiceOperationID)
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	operation, err := scanMatchSquadProvisioningOperation(tx.QueryRow(ctx, `
		SELECT match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes, state,
		       chat_operation_id, chat_request_sha256, chat_request_bytes, chat_receipt_id, chat_receipt_bytes, chat_id,
		       voice_operation_id, voice_request_sha256, voice_request_bytes, voice_receipt_id, voice_receipt_bytes, voice_room_id
		FROM matchmaking_match_squad_operations WHERE match_id = $1 FOR UPDATE
	`, intent.MatchID))
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	if operation.OperationID != intent.OperationID || operation.ChatOperationID != intent.ChatOperationID || operation.VoiceOperationID != intent.VoiceOperationID ||
		!bytes.Equal(operation.ParticipantManifestHash, intent.ParticipantManifestHash) || !bytes.Equal(operation.ParticipantManifestBytes, intent.ParticipantManifestBytes) ||
		!bytes.Equal(operation.ChatRequestHash, intent.ChatRequestHash) || !bytes.Equal(operation.ChatRequestBytes, intent.ChatRequestBytes) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	return operation, nil
}

// RecordMatchSquadChatReceipt makes the exact Chat receipt and the immutable
// Voice request durable before the next provider RPC can begin.
func (s *MatchStore) RecordMatchSquadChatReceipt(ctx context.Context, matchID, receiptID, chatID uuid.UUID, receiptBytes, voiceRequestHash, voiceRequestBytes []byte) (MatchSquadProvisioningOperation, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadProvisioningOperation{}, errors.New("match store unavailable")
	}
	if matchID == uuid.Nil || receiptID == uuid.Nil || chatID == uuid.Nil || len(receiptBytes) == 0 || len(voiceRequestBytes) == 0 || !bytes.Equal(sumSHA256(voiceRequestBytes), voiceRequestHash) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	operation, err := scanMatchSquadProvisioningOperation(tx.QueryRow(ctx, `
		SELECT match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes, state,
		       chat_operation_id, chat_request_sha256, chat_request_bytes, chat_receipt_id, chat_receipt_bytes, chat_id,
		       voice_operation_id, voice_request_sha256, voice_request_bytes, voice_receipt_id, voice_receipt_bytes, voice_room_id
		FROM matchmaking_match_squad_operations WHERE match_id = $1 FOR UPDATE
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadPending
	}
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	if operation.State != "provisioning" {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadPending
	}
	chatReceipt := new(chatv1.MatchSquadChatReceipt)
	voiceRequest := new(callsv1.CreateMatchSquadRoomRequest)
	if err := proto.Unmarshal(receiptBytes, chatReceipt); err != nil || len(chatReceipt.ProtoReflect().GetUnknown()) != 0 ||
		chatReceipt.GetProtocolVersion() != 1 || chatReceipt.GetOperationId() != operation.ChatOperationID.String() ||
		chatReceipt.GetMatchId() != matchID.String() || chatReceipt.GetChatId() != chatID.String() ||
		chatReceipt.GetReceiptId() != receiptID.String() ||
		!bytes.Equal(chatReceipt.GetParticipantManifestSha256(), operation.ParticipantManifestHash) ||
		!bytes.Equal(chatReceipt.GetRequestSha256(), operation.ChatRequestHash) ||
		proto.Unmarshal(voiceRequestBytes, voiceRequest) != nil || len(voiceRequest.ProtoReflect().GetUnknown()) != 0 ||
		voiceRequest.GetProtocolVersion() != 1 || voiceRequest.GetOperationId() != operation.VoiceOperationID.String() ||
		voiceRequest.GetMatchId() != matchID.String() || !bytes.Equal(voiceRequest.GetParticipantManifestSha256(), operation.ParticipantManifestHash) ||
		len(voiceRequest.GetParticipants()) != len(profileIDsFromManifest(operation.ParticipantManifestBytes)) || voiceRequest.GetChatCreationReceipt() == nil {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	for i, profileID := range profileIDsFromManifest(operation.ParticipantManifestBytes) {
		if voiceRequest.GetParticipants()[i] == nil || len(voiceRequest.GetParticipants()[i].ProtoReflect().GetUnknown()) != 0 || voiceRequest.GetParticipants()[i].GetProfileId() != profileID.String() {
			return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
		}
	}
	canonicalReceipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(chatReceipt)
	if err != nil || !bytes.Equal(canonicalReceipt, receiptBytes) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	canonicalVoiceRequest, err := (proto.MarshalOptions{Deterministic: true}).Marshal(voiceRequest)
	if err != nil || !bytes.Equal(canonicalVoiceRequest, voiceRequestBytes) || !proto.Equal(voiceRequest.GetChatCreationReceipt(), chatReceipt) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	if operation.ChatReceiptID != nil {
		if *operation.ChatReceiptID != receiptID || operation.ChatID == nil || *operation.ChatID != chatID || !bytes.Equal(operation.ChatReceiptBytes, receiptBytes) || !bytes.Equal(operation.VoiceRequestHash, voiceRequestHash) || !bytes.Equal(operation.VoiceRequestBytes, voiceRequestBytes) {
			return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
		}
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE matchmaking_match_squad_operations
			SET chat_receipt_id=$2, chat_receipt_bytes=$3, chat_id=$4,
			    voice_request_sha256=$5, voice_request_bytes=$6, updated_at=clock_timestamp()
			WHERE match_id=$1 AND state='provisioning' AND chat_receipt_id IS NULL
		`, matchID, receiptID, receiptBytes, chatID, voiceRequestHash, voiceRequestBytes)
		if err != nil {
			return MatchSquadProvisioningOperation{}, err
		}
		operation.ChatReceiptID = &receiptID
		operation.ChatReceiptBytes = append([]byte(nil), receiptBytes...)
		operation.ChatID = &chatID
		operation.VoiceRequestHash = append([]byte(nil), voiceRequestHash...)
		operation.VoiceRequestBytes = append([]byte(nil), voiceRequestBytes...)
	}
	if err := tx.Commit(ctx); err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	return operation, nil
}

// RecordMatchSquadVoiceReceipt persists the exact final receipt before match
// activation can be acknowledged to a participant.
func (s *MatchStore) RecordMatchSquadVoiceReceipt(ctx context.Context, matchID, receiptID, roomID uuid.UUID, receiptBytes []byte) (MatchSquadProvisioningOperation, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadProvisioningOperation{}, errors.New("match store unavailable")
	}
	if matchID == uuid.Nil || receiptID == uuid.Nil || roomID == uuid.Nil || len(receiptBytes) == 0 {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	operation, err := scanMatchSquadProvisioningOperation(tx.QueryRow(ctx, `
		SELECT match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes, state,
		       chat_operation_id, chat_request_sha256, chat_request_bytes, chat_receipt_id, chat_receipt_bytes, chat_id,
		       voice_operation_id, voice_request_sha256, voice_request_bytes, voice_receipt_id, voice_receipt_bytes, voice_room_id
		FROM matchmaking_match_squad_operations WHERE match_id = $1 FOR UPDATE
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadPending
	}
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	if operation.State != "provisioning" || operation.ChatReceiptID == nil || len(operation.VoiceRequestBytes) == 0 {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadPending
	}
	if operation.ChatID == nil {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadPending
	}
	voiceReceipt := new(callsv1.MatchSquadRoomReceipt)
	if err := proto.Unmarshal(receiptBytes, voiceReceipt); err != nil || len(voiceReceipt.ProtoReflect().GetUnknown()) != 0 ||
		voiceReceipt.GetProtocolVersion() != 1 || voiceReceipt.GetOperationId() != operation.VoiceOperationID.String() ||
		voiceReceipt.GetMatchId() != matchID.String() || voiceReceipt.GetRoomId() != roomID.String() ||
		voiceReceipt.GetChatId() != operation.ChatID.String() || voiceReceipt.GetChatCreationReceiptId() != operation.ChatReceiptID.String() ||
		!bytes.Equal(voiceReceipt.GetChatCreationReceiptSha256(), sumSHA256(operation.ChatReceiptBytes)) ||
		!bytes.Equal(voiceReceipt.GetParticipantManifestSha256(), operation.ParticipantManifestHash) ||
		!bytes.Equal(voiceReceipt.GetRequestSha256(), sumSHA256(operation.VoiceRequestBytes)) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	canonicalReceipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(voiceReceipt)
	if err != nil || !bytes.Equal(canonicalReceipt, receiptBytes) {
		return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
	}
	if operation.VoiceReceiptID != nil {
		if *operation.VoiceReceiptID != receiptID || operation.VoiceRoomID == nil || *operation.VoiceRoomID != roomID || !bytes.Equal(operation.VoiceReceiptBytes, receiptBytes) {
			return MatchSquadProvisioningOperation{}, ErrMatchSquadConflict
		}
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE matchmaking_match_squad_operations
			SET voice_receipt_id=$2, voice_receipt_bytes=$3, voice_room_id=$4, updated_at=clock_timestamp()
			WHERE match_id=$1 AND state='provisioning' AND voice_receipt_id IS NULL AND chat_receipt_id IS NOT NULL
		`, matchID, receiptID, receiptBytes, roomID)
		if err != nil {
			return MatchSquadProvisioningOperation{}, err
		}
		operation.VoiceReceiptID = &receiptID
		operation.VoiceReceiptBytes = append([]byte(nil), receiptBytes...)
		operation.VoiceRoomID = &roomID
	}
	if err := tx.Commit(ctx); err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	return operation, nil
}

// BeginMatchSquadProvisionCompensation freezes Chat-only teardown after the
// protected Voice provider has durably rejected the exact create request.
// Callers must classify that provider result before entering this transition.
func (s *MatchStore) BeginMatchSquadProvisionCompensation(ctx context.Context, matchID uuid.UUID) (MatchSquadTeardownAggregate, error) {
	if s == nil || s.Pool == nil {
		return MatchSquadTeardownAggregate{}, errors.New("match store unavailable")
	}
	if matchID == uuid.Nil {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	operation, err := scanMatchSquadProvisioningOperation(tx.QueryRow(ctx, `
		SELECT match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes, state,
		       chat_operation_id, chat_request_sha256, chat_request_bytes, chat_receipt_id, chat_receipt_bytes, chat_id,
		       voice_operation_id, voice_request_sha256, voice_request_bytes, voice_receipt_id, voice_receipt_bytes, voice_room_id
		FROM matchmaking_match_squad_operations WHERE match_id=$1 FOR UPDATE
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadPending
	}
	if err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	if operation.State == "compensating" {
		item, err := scanMatchSquadTeardownAggregate(tx.QueryRow(ctx, `
			SELECT aggregate_id, match_id, state,
			       chat_teardown_operation_id, chat_teardown_request_sha256, chat_teardown_request_bytes,
			       chat_teardown_receipt_id, chat_teardown_receipt_bytes,
			       voice_teardown_operation_id, voice_teardown_request_sha256, voice_teardown_request_bytes,
			       voice_teardown_receipt_id, voice_teardown_receipt_bytes, aggregate_completed_at, purpose, required_providers
			FROM matchmaking_match_squad_teardowns WHERE match_id=$1 FOR UPDATE
		`, matchID))
		if err != nil || item.Purpose != "PROVISION_COMPENSATION" || len(item.RequiredProviders) != 1 || item.RequiredProviders[0] != "chat" {
			if err != nil {
				return MatchSquadTeardownAggregate{}, err
			}
			return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return MatchSquadTeardownAggregate{}, err
		}
		return item, nil
	}
	if operation.State != "provisioning" || operation.ChatReceiptID == nil || operation.ChatID == nil || operation.VoiceReceiptID != nil ||
		len(operation.VoiceRequestBytes) == 0 || len(operation.VoiceRequestHash) != 32 ||
		!bytes.Equal(sumSHA256(operation.VoiceRequestBytes), operation.VoiceRequestHash) {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadPending
	}
	chatCreation := new(chatv1.MatchSquadChatReceipt)
	if len(operation.ChatRequestHash) != 32 || len(operation.ChatReceiptBytes) == 0 ||
		!bytes.Equal(sumSHA256(operation.ChatRequestBytes), operation.ChatRequestHash) ||
		proto.Unmarshal(operation.ChatReceiptBytes, chatCreation) != nil || len(chatCreation.ProtoReflect().GetUnknown()) != 0 ||
		chatCreation.GetProtocolVersion() != 1 || chatCreation.GetReceiptId() != operation.ChatReceiptID.String() ||
		chatCreation.GetOperationId() != operation.ChatOperationID.String() || chatCreation.GetMatchId() != matchID.String() ||
		chatCreation.GetChatId() != operation.ChatID.String() || !bytes.Equal(chatCreation.GetRequestSha256(), operation.ChatRequestHash) ||
		!bytes.Equal(chatCreation.GetParticipantManifestSha256(), operation.ParticipantManifestHash) {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
	}
	voiceCreate := new(callsv1.CreateMatchSquadRoomRequest)
	if proto.Unmarshal(operation.VoiceRequestBytes, voiceCreate) != nil || len(voiceCreate.ProtoReflect().GetUnknown()) != 0 ||
		voiceCreate.GetProtocolVersion() != 1 || voiceCreate.GetOperationId() != operation.VoiceOperationID.String() ||
		voiceCreate.GetMatchId() != matchID.String() || !bytes.Equal(voiceCreate.GetParticipantManifestSha256(), operation.ParticipantManifestHash) ||
		voiceCreate.GetChatCreationReceipt() == nil || !proto.Equal(voiceCreate.GetChatCreationReceipt(), chatCreation) {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
	}
	canonicalChatReceipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(chatCreation)
	if err != nil || !bytes.Equal(canonicalChatReceipt, operation.ChatReceiptBytes) {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
	}

	chatOperationID, aggregateID := uuid.New(), uuid.New()
	request := &chatv1.TeardownMatchSquadChatRequest{
		ProtocolVersion: 1, TeardownOperationId: chatOperationID.String(), MatchId: matchID.String(),
		ChatId: operation.ChatID.String(), CreationReceiptId: operation.ChatReceiptID.String(),
		ParticipantManifestSha256: append([]byte(nil), operation.ParticipantManifestHash...),
		CreationRequestSha256:     append([]byte(nil), operation.ChatRequestHash...),
	}
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_teardowns
		(aggregate_id,match_id,state,chat_teardown_operation_id,chat_teardown_request_sha256,chat_teardown_request_bytes,
		 purpose,required_providers)
		VALUES ($1,$2,'pending',$3,$4,$5,'PROVISION_COMPENSATION',ARRAY['chat']::TEXT[])
	`, aggregateID, matchID, chatOperationID, sumSHA256(requestBytes), requestBytes)
	if err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET state='compensating',updated_at=clock_timestamp() WHERE match_id=$1 AND state='provisioning' AND chat_receipt_id IS NOT NULL AND voice_receipt_id IS NULL`, matchID)
	if err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	if tag.RowsAffected() != 1 {
		return MatchSquadTeardownAggregate{}, ErrMatchSquadConflict
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_teardown_participants
		(aggregate_id,provider,operation_id,state,request_sha256,request_bytes)
		VALUES ($1,'chat',$2,'NOT_STARTED',$3,$4)
	`, aggregateID, chatOperationID, sumSHA256(requestBytes), requestBytes)
	if err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	item := MatchSquadTeardownAggregate{
		AggregateID: aggregateID, MatchID: matchID, State: "pending", Purpose: "PROVISION_COMPENSATION",
		RequiredProviders: []string{"chat"}, ChatOperationID: chatOperationID,
		ChatRequestHash: sumSHA256(requestBytes), ChatRequestBytes: requestBytes,
	}
	if err := tx.Commit(ctx); err != nil {
		return MatchSquadTeardownAggregate{}, err
	}
	return item, nil
}

// ActivateProvisionedMatch is the only new path that transitions a match to
// active: both locally validated, persisted provider receipts must exist first.
func (s *MatchStore) ActivateProvisionedMatch(ctx context.Context, matchID uuid.UUID) (Match, error) {
	if s == nil || s.Pool == nil {
		return Match{}, errors.New("match store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Match{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	match, err := scanMatch(tx.QueryRow(ctx, `
		SELECT id, game_id, mode, region, participants, left_profile_ids, voice_room_id, chat_id, status, created_at, completed_at
		FROM matches WHERE id=$1 FOR UPDATE
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, ErrMatchNotFound
	}
	if err != nil {
		return Match{}, err
	}
	proposals, err := loadMatchProposalsForUpdate(ctx, tx, matchID)
	if err != nil {
		return Match{}, err
	}
	sessions, err := loadMatchSessionsForUpdate(ctx, tx, matchID)
	if err != nil {
		return Match{}, err
	}
	operation, err := scanMatchSquadProvisioningOperation(tx.QueryRow(ctx, `
		SELECT match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes, state,
		       chat_operation_id, chat_request_sha256, chat_request_bytes, chat_receipt_id, chat_receipt_bytes, chat_id,
		       voice_operation_id, voice_request_sha256, voice_request_bytes, voice_receipt_id, voice_receipt_bytes, voice_room_id
		FROM matchmaking_match_squad_operations WHERE match_id=$1 FOR UPDATE
	`, matchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, ErrMatchSquadPending
	}
	if err != nil {
		return Match{}, err
	}
	if operation.State == "active" {
		if match.Status != MatchStatusActive || match.ChatID == nil || match.VoiceRoomID == nil || operation.ChatID == nil || operation.VoiceRoomID == nil ||
			*match.ChatID != operation.ChatID.String() || *match.VoiceRoomID != operation.VoiceRoomID.String() {
			return Match{}, ErrMatchSquadConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Match{}, err
		}
		return match, nil
	}
	if operation.State != "provisioning" || operation.ChatReceiptID == nil || operation.ChatID == nil || operation.VoiceReceiptID == nil || operation.VoiceRoomID == nil ||
		len(operation.ChatReceiptBytes) == 0 || len(operation.VoiceReceiptBytes) == 0 || len(proposals) == 0 || len(sessions) != len(proposals) {
		return Match{}, ErrMatchSquadPending
	}
	for _, proposal := range proposals {
		if proposal.Response != ProposalResponseAccepted {
			return Match{}, ErrMatchSquadPending
		}
	}
	if match.Status != MatchStatusPendingAccept {
		return Match{}, ErrMatchSquadConflict
	}
	now, err := databaseNowTx(ctx, tx)
	if err != nil {
		return Match{}, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE matches SET status=$2, chat_id=$3, voice_room_id=$4
		WHERE id=$1 AND status=$5
	`, matchID, MatchStatusActive, operation.ChatID.String(), operation.VoiceRoomID.String(), MatchStatusPendingAccept)
	if err != nil {
		return Match{}, err
	}
	if tag.RowsAffected() != 1 {
		return Match{}, ErrMatchSquadConflict
	}
	sessionsUpdated, err := tx.Exec(ctx, `
		UPDATE search_sessions SET status=$2, updated_at=$3, recovery_generation=recovery_generation+1
		WHERE match_id=$1 AND status=$4
	`, matchID, SessionStatusMatched, now, SessionStatusPendingAccept)
	if err != nil {
		return Match{}, err
	}
	if sessionsUpdated.RowsAffected() != int64(len(sessions)) {
		return Match{}, ErrMatchSquadConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET state='active',updated_at=clock_timestamp() WHERE match_id=$1 AND state='provisioning'`, matchID); err != nil {
		return Match{}, err
	}
	match.Status, match.ChatID, match.VoiceRoomID = MatchStatusActive, uuidString(operation.ChatID), uuidString(operation.VoiceRoomID)
	if err := tx.Commit(ctx); err != nil {
		return Match{}, err
	}
	return match, nil
}

// persistMatchSquadTeardownIntentTx starts the provider teardown only after
// Matchmaking has durably completed the whole match. A participant's own
// Voice Leave remains a separate membership transition and never closes the
// shared room by itself.
func persistMatchSquadTeardownIntentTx(ctx context.Context, tx pgx.Tx, match Match) error {
	operation, err := scanMatchSquadProvisioningOperation(tx.QueryRow(ctx, `
		SELECT match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes, state,
		       chat_operation_id, chat_request_sha256, chat_request_bytes, chat_receipt_id, chat_receipt_bytes, chat_id,
		       voice_operation_id, voice_request_sha256, voice_request_bytes, voice_receipt_id, voice_receipt_bytes, voice_room_id
		FROM matchmaking_match_squad_operations WHERE match_id=$1 FOR UPDATE
	`, match.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		// A legacy match without provider-owned resource IDs needs no provider
		// teardown. If resource IDs exist without their immutable receipt ledger,
		// fail closed instead of guessing which resource may be deleted.
		if match.ChatID == nil && match.VoiceRoomID == nil {
			return nil
		}
		return ErrMatchSquadPending
	}
	if err != nil {
		return err
	}
	if operation.State != "active" || operation.ChatID == nil || operation.VoiceRoomID == nil ||
		operation.ChatReceiptID == nil || operation.VoiceReceiptID == nil ||
		match.ChatID == nil || match.VoiceRoomID == nil ||
		*match.ChatID != operation.ChatID.String() || *match.VoiceRoomID != operation.VoiceRoomID.String() {
		return ErrMatchSquadConflict
	}
	chatCreation := new(chatv1.MatchSquadChatReceipt)
	voiceCreation := new(callsv1.MatchSquadRoomReceipt)
	if len(operation.ChatRequestHash) != 32 || len(operation.VoiceRequestHash) != 32 ||
		proto.Unmarshal(operation.ChatReceiptBytes, chatCreation) != nil ||
		proto.Unmarshal(operation.VoiceReceiptBytes, voiceCreation) != nil ||
		chatCreation.GetProtocolVersion() != 1 || chatCreation.GetReceiptId() != operation.ChatReceiptID.String() ||
		chatCreation.GetOperationId() != operation.ChatOperationID.String() || chatCreation.GetMatchId() != match.ID.String() ||
		chatCreation.GetChatId() != operation.ChatID.String() || !bytes.Equal(chatCreation.GetRequestSha256(), operation.ChatRequestHash) ||
		!bytes.Equal(chatCreation.GetParticipantManifestSha256(), operation.ParticipantManifestHash) ||
		voiceCreation.GetProtocolVersion() != 1 || voiceCreation.GetReceiptId() != operation.VoiceReceiptID.String() ||
		voiceCreation.GetOperationId() != operation.VoiceOperationID.String() || voiceCreation.GetMatchId() != match.ID.String() ||
		voiceCreation.GetRoomId() != operation.VoiceRoomID.String() || voiceCreation.GetChatId() != operation.ChatID.String() ||
		voiceCreation.GetChatCreationReceiptId() != operation.ChatReceiptID.String() ||
		!bytes.Equal(voiceCreation.GetChatCreationReceiptSha256(), sumSHA256(operation.ChatReceiptBytes)) ||
		!bytes.Equal(voiceCreation.GetRequestSha256(), operation.VoiceRequestHash) ||
		!bytes.Equal(voiceCreation.GetParticipantManifestSha256(), operation.ParticipantManifestHash) {
		return ErrMatchSquadConflict
	}
	chatOperationID, voiceOperationID, aggregateID := uuid.New(), uuid.New(), uuid.New()
	chatRequest := &chatv1.TeardownMatchSquadChatRequest{
		ProtocolVersion: 1, TeardownOperationId: chatOperationID.String(), MatchId: match.ID.String(),
		ChatId: operation.ChatID.String(), CreationReceiptId: operation.ChatReceiptID.String(),
		ParticipantManifestSha256: append([]byte(nil), operation.ParticipantManifestHash...),
		CreationRequestSha256:     append([]byte(nil), operation.ChatRequestHash...),
	}
	voiceRequest := &callsv1.TeardownMatchSquadRoomRequest{
		ProtocolVersion: 1, TeardownOperationId: voiceOperationID.String(), MatchId: match.ID.String(),
		RoomId: operation.VoiceRoomID.String(), CreationReceiptId: operation.VoiceReceiptID.String(),
		ParticipantManifestSha256: append([]byte(nil), operation.ParticipantManifestHash...),
		CreationRequestSha256:     append([]byte(nil), operation.VoiceRequestHash...),
	}
	chatBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(chatRequest)
	if err != nil {
		return err
	}
	voiceBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(voiceRequest)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_teardowns
		(aggregate_id, match_id, state, chat_teardown_operation_id,
		 chat_teardown_request_sha256, chat_teardown_request_bytes,
		 voice_teardown_operation_id, voice_teardown_request_sha256,
		 voice_teardown_request_bytes)
		VALUES ($1,$2,'pending',$3,$4,$5,$6,$7,$8)
		ON CONFLICT (match_id) DO NOTHING
	`, aggregateID, match.ID, chatOperationID, sumSHA256(chatBytes), chatBytes,
		voiceOperationID, sumSHA256(voiceBytes), voiceBytes)
	if err != nil {
		return err
	}
	var storedAggregate, storedChatOperation, storedVoiceOperation uuid.UUID
	var storedChatRequest, storedVoiceRequest []byte
	if err := tx.QueryRow(ctx, `
		SELECT aggregate_id, chat_teardown_operation_id, chat_teardown_request_bytes,
		       voice_teardown_operation_id, voice_teardown_request_bytes
		FROM matchmaking_match_squad_teardowns WHERE match_id=$1 FOR UPDATE
	`, match.ID).Scan(&storedAggregate, &storedChatOperation, &storedChatRequest, &storedVoiceOperation, &storedVoiceRequest); err != nil {
		return err
	}
	if storedAggregate != aggregateID || storedChatOperation != chatOperationID || storedVoiceOperation != voiceOperationID ||
		!bytes.Equal(storedChatRequest, chatBytes) || !bytes.Equal(storedVoiceRequest, voiceBytes) {
		return ErrMatchSquadConflict
	}
	for _, participant := range []struct {
		provider    string
		operationID uuid.UUID
		request     []byte
	}{{"chat", chatOperationID, chatBytes}, {"voice", voiceOperationID, voiceBytes}} {
		requestHash := sumSHA256(participant.request)
		_, err := tx.Exec(ctx, `
			INSERT INTO matchmaking_match_squad_teardown_participants
			(aggregate_id,provider,operation_id,state,request_sha256,request_bytes)
			VALUES ($1,$2,$3,'NOT_STARTED',$4,$5) ON CONFLICT (aggregate_id,provider) DO NOTHING
		`, storedAggregate, participant.provider, participant.operationID, requestHash, participant.request)
		if err != nil {
			return err
		}
		var storedOperation uuid.UUID
		var storedHash, storedBytes []byte
		if err := tx.QueryRow(ctx, `
			SELECT operation_id,request_sha256,request_bytes FROM matchmaking_match_squad_teardown_participants
			WHERE aggregate_id=$1 AND provider=$2 FOR UPDATE
		`, storedAggregate, participant.provider).Scan(&storedOperation, &storedHash, &storedBytes); err != nil {
			return err
		}
		if storedOperation != participant.operationID || !bytes.Equal(storedHash, requestHash) || !bytes.Equal(storedBytes, participant.request) {
			return ErrMatchSquadConflict
		}
	}
	return nil
}

func databaseNowTx(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	var now time.Time
	err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now)
	return now, err
}

func uuidString(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}

func scanMatchSquadProvisioningOperation(row pgx.Row) (MatchSquadProvisioningOperation, error) {
	var operation MatchSquadProvisioningOperation
	var chatReceiptID, chatID, voiceReceiptID, voiceRoomID *uuid.UUID
	err := row.Scan(
		&operation.MatchID, &operation.OperationID, &operation.ParticipantManifestHash, &operation.ParticipantManifestBytes, &operation.State,
		&operation.ChatOperationID, &operation.ChatRequestHash, &operation.ChatRequestBytes, &chatReceiptID, &operation.ChatReceiptBytes, &chatID,
		&operation.VoiceOperationID, &operation.VoiceRequestHash, &operation.VoiceRequestBytes, &voiceReceiptID, &operation.VoiceReceiptBytes, &voiceRoomID,
	)
	if err != nil {
		return MatchSquadProvisioningOperation{}, err
	}
	operation.ChatReceiptID, operation.ChatID, operation.VoiceReceiptID, operation.VoiceRoomID = chatReceiptID, chatID, voiceReceiptID, voiceRoomID
	return operation, nil
}

func sumSHA256(value []byte) []byte {
	hash := sha256.Sum256(value)
	return hash[:]
}

func profileIDsFromManifest(manifest []byte) []uuid.UUID {
	if len(manifest) == 0 || len(manifest)%16 != 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(manifest)/16)
	for offset := 0; offset < len(manifest); offset += 16 {
		var id uuid.UUID
		copy(id[:], manifest[offset:offset+16])
		ids = append(ids, id)
	}
	return ids
}
