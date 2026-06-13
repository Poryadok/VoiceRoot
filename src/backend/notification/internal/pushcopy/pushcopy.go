package pushcopy

import (
	"strings"
)

const maxPreviewLen = 120

// MessageBody formats a push notification body for a new chat message.
func MessageBody(preview string) string {
	preview = strings.TrimSpace(preview)
	if preview == "" {
		return "You have a new message"
	}
	return truncate(preview, maxPreviewLen)
}

// MentionBody formats a push notification body for an @mention.
func MentionBody(preview string) string {
	preview = strings.TrimSpace(preview)
	if preview == "" {
		return "You were mentioned"
	}
	return "Mentioned you: " + truncate(preview, maxPreviewLen)
}

// TitleForSender uses a short sender label when available.
func TitleForSender(senderLabel, fallback string) string {
	label := strings.TrimSpace(senderLabel)
	if label != "" {
		return label
	}
	return fallback
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
