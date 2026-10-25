package grpcsvc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/mentions"
)

// Fingerprint the documented effective request, including defaults/derived
// content type. JSON object key order is irrelevant; array order is retained.
// File-dependent enrichment is deliberately outside the replay identity.
func sendRequestFingerprint(req *messagingv1.SendMessageRequest, chatID uuid.UUID, chatType chatv1.ChatType, thread *uuid.UUID) (string, error) {
	content := strings.TrimSpace(req.GetContent())
	attachments := strings.TrimSpace(req.GetAttachmentsJson())
	if attachments == "" {
		attachments = "[]"
	}
	attachmentJSON, err := jsoncanonicalizer.Transform([]byte(attachments))
	if err != nil {
		return "", err
	}
	mentionText := strings.TrimSpace(req.GetMentionsJson())
	if mentionText == "" || mentionText == "[]" {
		mentionText = mentions.EntriesJSONFromContent(content)
	}
	mentionJSON, err := jsoncanonicalizer.Transform([]byte(mentionText))
	if err != nil {
		return "", err
	}
	kind, _ := messageKindToWire(req.GetMessageKind())
	value := struct {
		ChatID                          string
		ChatType                        int32
		Content, Kind, ContentType      string
		Attachments, Mentions           json.RawMessage
		Thread                          *uuid.UUID
		PostedAsChat, IsE2E, SendSilent bool
	}{chatID.String(), int32(chatType), content, kind, resolveSendContentType(req, content, attachments), attachmentJSON, mentionJSON, thread, req.GetPostedAsChat(), req.GetIsE2E(), req.GetSendSilent()}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
