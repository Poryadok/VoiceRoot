package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

// A3 requires validated exact receipts before either provider can contribute to
// completion. These tests exercise the production validators without a database;
// aggregate completion, retry and outbox atomicity remain integration concerns.
func TestMatchSquadTeardownReceiptRejectsEveryMismatchedBinding(t *testing.T) {
	matchID, operationID, receiptID := uuid.New(), uuid.New(), uuid.New()
	resourceID, creationID := uuid.NewString(), uuid.NewString()
	manifest := bytes.Repeat([]byte{0x42}, 32)
	completedAt := timestamppb.New(time.Unix(1700000000, 0))
	for _, provider := range []string{"chat", "voice"} {
		t.Run(provider, func(t *testing.T) {
			var request, receipt proto.Message
			item := MatchSquadTeardownAggregate{MatchID: matchID, ChatOperationID: operationID, VoiceOperationID: operationID}
			if provider == "chat" {
				request = &chatv1.TeardownMatchSquadChatRequest{ProtocolVersion: 1, TeardownOperationId: operationID.String(), MatchId: matchID.String(), ChatId: resourceID, CreationReceiptId: creationID, ParticipantManifestSha256: manifest}
				receipt = &chatv1.MatchSquadChatTeardownReceipt{ProtocolVersion: 1, ReceiptId: receiptID.String(), TeardownOperationId: operationID.String(), MatchId: matchID.String(), ChatId: resourceID, CreationReceiptId: creationID, ParticipantManifestSha256: manifest, Status: chatv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: completedAt}
			} else {
				request = &callsv1.TeardownMatchSquadRoomRequest{ProtocolVersion: 1, TeardownOperationId: operationID.String(), MatchId: matchID.String(), RoomId: resourceID, CreationReceiptId: creationID, ParticipantManifestSha256: manifest}
				receipt = &callsv1.MatchSquadRoomTeardownReceipt{ProtocolVersion: 1, ReceiptId: receiptID.String(), TeardownOperationId: operationID.String(), MatchId: matchID.String(), RoomId: resourceID, CreationReceiptId: creationID, ParticipantManifestSha256: manifest, Status: callsv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: completedAt}
			}
			requestBytes, err := proto.Marshal(request)
			require.NoError(t, err)
			item.ChatRequestBytes, item.VoiceRequestBytes = requestBytes, requestBytes
			item.ChatRequestHash, item.VoiceRequestHash = sumSHA256(requestBytes), sumSHA256(requestBytes)
			setReceiptField(receipt, "request_sha256", protoreflect.ValueOfBytes(sumSHA256(requestBytes)))
			validate := func(value proto.Message) error {
				raw, marshalErr := proto.Marshal(value)
				require.NoError(t, marshalErr)
				if provider == "chat" {
					return validateChatTeardownReceipt(item, receiptID, raw)
				}
				return validateVoiceTeardownReceipt(item, receiptID, raw)
			}
			require.NoError(t, validate(receipt), "positive control must be an exact valid receipt")
			resourceField := "chat_id"
			if provider == "voice" {
				resourceField = "room_id"
			}
			for _, field := range []string{"receipt_id", "teardown_operation_id", "match_id", resourceField, "creation_receipt_id"} {
				t.Run(field, func(t *testing.T) {
					changed := proto.Clone(receipt)
					setReceiptField(changed, field, protoreflect.ValueOfString(uuid.NewString()))
					require.ErrorIs(t, validate(changed), ErrMatchSquadConflict)
				})
			}
			for _, field := range []string{"participant_manifest_sha256", "request_sha256"} {
				t.Run(field, func(t *testing.T) {
					changed := proto.Clone(receipt)
					setReceiptField(changed, field, protoreflect.ValueOfBytes(bytes.Repeat([]byte{0x99}, 32)))
					require.ErrorIs(t, validate(changed), ErrMatchSquadConflict)
				})
			}
			for _, field := range []string{"protocol_version", "status", "completed_at"} {
				t.Run("missing_"+field, func(t *testing.T) {
					changed := proto.Clone(receipt)
					changed.ProtoReflect().Clear(changed.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(field)))
					require.ErrorIs(t, validate(changed), ErrMatchSquadConflict)
				})
			}
			t.Run("corrupt_stored_request_digest", func(t *testing.T) {
				item.ChatRequestHash, item.VoiceRequestHash = make([]byte, 32), make([]byte, 32)
				require.ErrorIs(t, validate(receipt), ErrMatchSquadConflict)
			})
		})
	}
}

func setReceiptField(message proto.Message, name string, value protoreflect.Value) {
	message.ProtoReflect().Set(message.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(name)), value)
}
