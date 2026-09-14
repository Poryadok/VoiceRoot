package store

import (
	"time"

	"github.com/google/uuid"
)

// ScheduledMessageScheduleKind is the durable representation of the delivery
// schedule oneof. It deliberately excludes worker behaviour and presence.
type ScheduledMessageScheduleKind string

const (
	ScheduledMessageScheduleAt         ScheduledMessageScheduleKind = "at"
	ScheduledMessageScheduleWhenOnline ScheduledMessageScheduleKind = "when_online"
)

// ScheduledMessageStatus is the durable lifecycle owned by future schedule
// handlers and dispatch workers.
type ScheduledMessageStatus string

const (
	ScheduledMessagePending   ScheduledMessageStatus = "pending"
	ScheduledMessageSent      ScheduledMessageStatus = "sent"
	ScheduledMessageCancelled ScheduledMessageStatus = "cancelled"
	ScheduledMessageFailed    ScheduledMessageStatus = "failed"
)

// ScheduledMessageRow maps scheduled_messages. Retry, lease and outbox IDs are
// intentionally storage-only and are not exposed through the gRPC contract.
type ScheduledMessageRow struct {
	ID                     uuid.UUID
	ChatID                 uuid.UUID
	SenderProfileID        uuid.UUID
	ClientMessageID        *uuid.UUID
	PayloadJSON            string
	ScheduleKind           ScheduledMessageScheduleKind
	ScheduledAt            *time.Time
	Status                 ScheduledMessageStatus
	SentMessageID          *uuid.UUID
	DispatchEventID        *uuid.UUID
	AttemptCount           int
	NextAttemptAt          *time.Time
	LastErrorCode          *string
	FailedAt               *time.Time
	DispatchLeaseOwner     *string
	DispatchLeaseExpiresAt *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
