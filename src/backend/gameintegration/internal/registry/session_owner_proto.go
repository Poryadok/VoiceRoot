package registry

import (
	"encoding/hex"
	"errors"
	"slices"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
)

func principalHashBytes(value string) ([]byte, error) {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 {
		return nil, errors.New("invalid canonical owner request hash")
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("invalid canonical owner request hash")
	}
	return decoded, nil
}

func BuildSessionOwnerProto(stage string, request SessionOwnerRequest) (proto.Message, error) {
	if request.ApplicationID == uuid.Nil || request.EnvironmentID == uuid.Nil || request.SessionID == uuid.Nil || request.OperationID == uuid.Nil {
		return nil, errors.New("owner request scope and operation are required")
	}
	switch stage {
	case "chat_create":
		if request.ExternalKey == "" || request.DisplayName == "" {
			return nil, errors.New("chat creation fields are required")
		}
		return &chatv1.ProvisionManagedChatRequest{ApplicationId: request.ApplicationID.String(), EnvironmentId: request.EnvironmentID.String(),
			OperationId: request.OperationID.String(), ExternalChatKey: request.ExternalKey, Name: request.DisplayName}, nil
	case "chat_roster":
		if request.ChatID == uuid.Nil {
			return nil, errors.New("Chat ID is required for roster sync")
		}
		return &chatv1.SyncManagedChatMembersRequest{ApplicationId: request.ApplicationID.String(), EnvironmentId: request.EnvironmentID.String(),
			OperationId: request.OperationID.String(), ChatId: request.ChatID.String(), ProfileIds: canonicalProfileIDs(request.Members)}, nil
	case "voice_provision":
		if request.ChatID == uuid.Nil || request.ChatCreateReceiptID == uuid.Nil {
			return nil, errors.New("Chat identity and receipt are required for Voice")
		}
		return &callsv1.ProvisionGameSessionRoomRequest{OperationId: request.OperationID.String(), ApplicationId: request.ApplicationID.String(),
			EnvironmentId: request.EnvironmentID.String(), Resource: &callsv1.GameSessionResourceRef{Kind: gameSessionKind(request.Kind), ExternalResourceKey: request.ExternalKey},
			ChatId: request.ChatID.String(), ChatCreationOperationId: request.ChatCreateReceiptID.String(), SessionId: request.SessionID.String()}, nil
	case "role_apply":
		if request.VoiceRoomID == uuid.Nil || request.RosterRevision <= 0 {
			return nil, errors.New("Voice room and roster revision are required for Role")
		}
		return &rolev1.ApplyGameSessionGrantsRequest{ApplicationId: request.ApplicationID.String(), EnvironmentId: request.EnvironmentID.String(),
			SessionId: request.SessionID.String(), VoiceRoomId: request.VoiceRoomID.String(), OperationId: request.OperationID.String(),
			RosterRevision: uint64(request.RosterRevision), ProfileIds: canonicalProfileIDs(request.Members)}, nil
	case "terminalize_voice_close":
		if request.VoiceRoomID == uuid.Nil || request.ChatID == uuid.Nil || request.ChatCreateReceiptID == uuid.Nil {
			return nil, errors.New("Voice room and Chat receipt are required for close")
		}
		return &callsv1.CloseGameSessionRoomRequest{OperationId: request.OperationID.String(), ApplicationId: request.ApplicationID.String(),
			EnvironmentId: request.EnvironmentID.String(), Resource: &callsv1.GameSessionResourceRef{Kind: gameSessionKind(request.Kind), ExternalResourceKey: request.ExternalKey},
			ChatId: request.ChatID.String(), ChatCreationOperationId: request.ChatCreateReceiptID.String(), SessionId: request.SessionID.String()}, nil
	case "terminalize_role_revoke":
		return &rolev1.RevokeGameSessionGrantsRequest{ApplicationId: request.ApplicationID.String(), EnvironmentId: request.EnvironmentID.String(),
			SessionId: request.SessionID.String(), OperationId: request.OperationID.String()}, nil
	default:
		return nil, errors.New("unknown owner request stage")
	}
}

func canonicalProfileIDs(members []uuid.UUID) []string {
	profiles := make([]string, len(members))
	for i, member := range members {
		profiles[i] = member.String()
	}
	slices.Sort(profiles)
	return profiles
}

func gameSessionKind(kind string) callsv1.GameSessionResourceKind {
	switch kind {
	case "party":
		return callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_PARTY
	case "match":
		return callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_MATCH
	case "fleet":
		return callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_FLEET_SESSION
	default:
		return callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_UNSPECIFIED
	}
}
