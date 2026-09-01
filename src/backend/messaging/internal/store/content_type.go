package store

import (
	"encoding/json"
	"strings"
)

// EffectiveContentType prefers the durable column; falls back to attachment/body inference.
func EffectiveContentType(storedType, content, attachmentsJSON string) string {
	if t := strings.TrimSpace(storedType); t != "" {
		return t
	}
	return inferLastMessageContentType(content, attachmentsJSON)
}

// inferLastMessageContentType derives list-preview content type from message body and attachments.
// Until messages.content_type column ships, attachments[].type is the primary signal.
func inferLastMessageContentType(content, attachmentsJSON string) string {
	if t := contentTypeFromAttachments(attachmentsJSON); t != "" {
		return t
	}
	if strings.TrimSpace(content) != "" {
		return "text"
	}
	return ""
}

func contentTypeFromAttachments(attachmentsJSON string) string {
	attachmentsJSON = strings.TrimSpace(attachmentsJSON)
	if attachmentsJSON == "" || attachmentsJSON == "[]" || attachmentsJSON == "null" {
		return ""
	}
	var items []struct {
		Type string `json:"type"`
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal([]byte(attachmentsJSON), &items); err != nil || len(items) == 0 {
		return ""
	}
	typ := strings.ToLower(strings.TrimSpace(items[0].Type))
	if typ == "" {
		typ = strings.ToLower(strings.TrimSpace(items[0].Kind))
	}
	switch typ {
	case "image":
		return "photo"
	case "video":
		return "video"
	case "video_note":
		return "video_note"
	case "document", "file":
		return "document"
	case "audio", "voice_message", "voice":
		return "voice"
	case "sticker":
		return "sticker"
	case "gif":
		return "gif"
	case "music":
		return "music"
	case "article", "link":
		return "article"
	case "location":
		return "location"
	default:
		return ""
	}
}
