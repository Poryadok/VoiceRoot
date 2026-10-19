package store

import (
	"bytes"
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

// The caller holds match and operation. A known creation is converted into
// exactly one executable cleanup request in the same receipt transaction.
func materializeMatchSquadCompensationTx(ctx context.Context, tx pgx.Tx, operation MatchSquadProvisioningOperation, provider string) error {
	if provider != "chat" && provider != "voice" {
		return ErrMatchSquadConflict
	}
	var aggregateID, teardownID uuid.UUID
	var purpose, state string
	var required []string
	var storedHash, storedBytes []byte
	if err := tx.QueryRow(ctx, `SELECT aggregate_id,purpose,state,required_providers,
		CASE WHEN $2='chat' THEN chat_teardown_operation_id ELSE voice_teardown_operation_id END,
		CASE WHEN $2='chat' THEN chat_teardown_request_sha256 ELSE voice_teardown_request_sha256 END,
		CASE WHEN $2='chat' THEN chat_teardown_request_bytes ELSE voice_teardown_request_bytes END
		FROM matchmaking_match_squad_teardowns WHERE match_id=$1 FOR UPDATE`, operation.MatchID, provider).
		Scan(&aggregateID, &purpose, &state, &required, &teardownID, &storedHash, &storedBytes); err != nil {
		return err
	}
	wanted := false
	for _, p := range required {
		wanted = wanted || p == provider
	}
	if purpose != "PROVISION_COMPENSATION" || !wanted || teardownID == uuid.Nil {
		return ErrMatchSquadConflict
	}
	var request proto.Message
	if provider == "chat" {
		if operation.ChatID == nil || operation.ChatReceiptID == nil || len(operation.ChatReceiptBytes) == 0 {
			return ErrMatchSquadConflict
		}
		request = &chatv1.TeardownMatchSquadChatRequest{ProtocolVersion: 1, TeardownOperationId: teardownID.String(),
			MatchId: operation.MatchID.String(), ChatId: operation.ChatID.String(), CreationReceiptId: operation.ChatReceiptID.String(),
			ParticipantManifestSha256: append([]byte(nil), operation.ParticipantManifestHash...), CreationRequestSha256: append([]byte(nil), operation.ChatRequestHash...)}
	} else {
		if operation.VoiceRoomID == nil || operation.VoiceReceiptID == nil || len(operation.VoiceReceiptBytes) == 0 {
			return ErrMatchSquadConflict
		}
		request = &callsv1.TeardownMatchSquadRoomRequest{ProtocolVersion: 1, TeardownOperationId: teardownID.String(),
			MatchId: operation.MatchID.String(), RoomId: operation.VoiceRoomID.String(), CreationReceiptId: operation.VoiceReceiptID.String(),
			ParticipantManifestSha256: append([]byte(nil), operation.ParticipantManifestHash...), CreationRequestSha256: append([]byte(nil), operation.VoiceRequestHash...)}
	}
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return err
	}
	requestHash := sumSHA256(requestBytes)
	if storedBytes != nil {
		if !bytes.Equal(storedBytes, requestBytes) || !bytes.Equal(storedHash, requestHash) {
			return ErrMatchSquadConflict
		}
	} else {
		if state != "pending" {
			return ErrMatchSquadConflict
		}
		query := `UPDATE matchmaking_match_squad_teardowns SET chat_teardown_request_sha256=$2,chat_teardown_request_bytes=$3,updated_at=clock_timestamp() WHERE aggregate_id=$1`
		if provider == "voice" {
			query = `UPDATE matchmaking_match_squad_teardowns SET voice_teardown_request_sha256=$2,voice_teardown_request_bytes=$3,updated_at=clock_timestamp() WHERE aggregate_id=$1`
		}
		if _, err := tx.Exec(ctx, query, aggregateID, requestHash, requestBytes); err != nil {
			return err
		}
	}
	var participantID uuid.UUID
	var participantState string
	var participantHash, participantBytes []byte
	if err := tx.QueryRow(ctx, `SELECT operation_id,state,request_sha256,request_bytes
		FROM matchmaking_match_squad_teardown_participants WHERE aggregate_id=$1 AND provider=$2 FOR UPDATE`, aggregateID, provider).
		Scan(&participantID, &participantState, &participantHash, &participantBytes); err != nil {
		return err
	}
	if participantID != teardownID {
		return ErrMatchSquadConflict
	}
	if participantState == "WAITING_FOR_CREATION" {
		_, err := tx.Exec(ctx, `UPDATE matchmaking_match_squad_teardown_participants
			SET state='NOT_STARTED',request_sha256=$3,request_bytes=$4,updated_at=clock_timestamp(),last_progress_at=clock_timestamp()
			WHERE aggregate_id=$1 AND provider=$2 AND state='WAITING_FOR_CREATION'`, aggregateID, provider, requestHash, requestBytes)
		return err
	}
	if !bytes.Equal(participantHash, requestHash) || !bytes.Equal(participantBytes, requestBytes) {
		return ErrMatchSquadConflict
	}
	return nil
}
