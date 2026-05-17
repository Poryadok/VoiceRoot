package messageid

import (
	"github.com/google/uuid"
)

// NewMessageID returns a new UUID version 7 for messaging_db.messages.id
// (time-ordered, application-generated; see docs/DATA_MODEL.md).
func NewMessageID() (uuid.UUID, error) {
	return uuid.NewV7()
}
