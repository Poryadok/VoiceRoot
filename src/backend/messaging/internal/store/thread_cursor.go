package store

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidThreadCursor = errors.New("invalid thread cursor")

// ThreadCursor identifies the final item of a descending ListThreads page.
// Its pair matches the stable SQL ordering exactly.
type ThreadCursor struct {
	LastReplyAt time.Time
	ThreadID    uuid.UUID
}

type threadCursorPayload struct {
	T string `json:"t"`
	I string `json:"i"`
}

func EncodeThreadCursor(lastReplyAt time.Time, threadID uuid.UUID) string {
	p := threadCursorPayload{T: lastReplyAt.UTC().Format(time.RFC3339Nano), I: threadID.String()}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodeThreadCursor(raw string) (*ThreadCursor, error) {
	if raw == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalidThreadCursor
	}
	var p threadCursorPayload
	if err := json.Unmarshal(b, &p); err != nil || p.T == "" || p.I == "" {
		return nil, ErrInvalidThreadCursor
	}
	lastReplyAt, err := time.Parse(time.RFC3339Nano, p.T)
	if err != nil {
		return nil, ErrInvalidThreadCursor
	}
	threadID, err := uuid.Parse(p.I)
	if err != nil {
		return nil, ErrInvalidThreadCursor
	}
	return &ThreadCursor{LastReplyAt: lastReplyAt, ThreadID: threadID}, nil
}
