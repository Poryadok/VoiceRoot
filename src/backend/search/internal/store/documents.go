package store

import (
	"time"

	"github.com/google/uuid"
)

// MessageDocument is a projection row for full-text message search.
type MessageDocument struct {
	MessageID       uuid.UUID
	ChatID          uuid.UUID
	SenderProfileID uuid.UUID
	Body            string
	CreatedAt       time.Time
}

// MessageHit is a ranked search result for a message.
type MessageHit struct {
	MessageID uuid.UUID
	ChatID    uuid.UUID
	Snippet   string
	Score     float64
}

// ProfileDocument is a projection row for profile discovery search.
type ProfileDocument struct {
	ProfileID         uuid.UUID
	AccountID         uuid.UUID
	Username          string
	Discriminator     string
	DisplayName       string
	VerificationType  string
}

// ProfileHit is a profile search result row.
type ProfileHit struct {
	ProfileID uuid.UUID
}

// SpaceDocument is a projection row for public space catalog search.
type SpaceDocument struct {
	SpaceID     uuid.UUID
	Name        string
	Description string
	Visibility  string
	MemberCount int
}

// SpaceHit is a space search result row.
type SpaceHit struct {
	SpaceID uuid.UUID
}
