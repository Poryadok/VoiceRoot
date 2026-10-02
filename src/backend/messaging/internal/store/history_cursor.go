package store

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

var ErrInvalidHistoryCursor = errors.New("invalid history cursor")

type historyCursorPayload struct {
	C string `json:"c"`           // chat that owns this history cursor
	B string `json:"b,omitempty"` // load older than this message id
	A string `json:"a,omitempty"` // load newer than this message id
}

func EncodeBeforeCursor(chatID, oldestMessageID uuid.UUID) string {
	p := historyCursorPayload{C: chatID.String(), B: oldestMessageID.String()}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

func EncodeAfterCursor(chatID, newestMessageID uuid.UUID) string {
	p := historyCursorPayload{C: chatID.String(), A: newestMessageID.String()}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodeHistoryCursor(raw string, chatID uuid.UUID) (beforeID, afterID *uuid.UUID, err error) {
	if raw == "" {
		return nil, nil, nil
	}
	b, decErr := base64.RawURLEncoding.DecodeString(raw)
	if decErr != nil {
		return nil, nil, ErrInvalidHistoryCursor
	}
	var p historyCursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, nil, ErrInvalidHistoryCursor
	}
	cursorChatID, err := uuid.Parse(p.C)
	if err != nil || cursorChatID != chatID {
		return nil, nil, ErrInvalidHistoryCursor
	}
	if p.B != "" && p.A != "" {
		return nil, nil, ErrInvalidHistoryCursor
	}
	if p.B != "" {
		id, err := uuid.Parse(p.B)
		if err != nil {
			return nil, nil, ErrInvalidHistoryCursor
		}
		return &id, nil, nil
	}
	if p.A != "" {
		id, err := uuid.Parse(p.A)
		if err != nil {
			return nil, nil, ErrInvalidHistoryCursor
		}
		return nil, &id, nil
	}
	return nil, nil, ErrInvalidHistoryCursor
}
