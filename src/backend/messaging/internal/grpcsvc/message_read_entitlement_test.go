package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fixedMessageEntitlement struct {
	allowed bool
	seenAt  time.Time
}

func (f *fixedMessageEntitlement) MessageReadEntitled(_ context.Context, _, _ uuid.UUID, createdAt time.Time) (bool, error) {
	f.seenAt = createdAt
	return f.allowed, nil
}

func (*fixedMessageEntitlement) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (*fixedMessageEntitlement) DMOtherProfileID(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (*fixedMessageEntitlement) OtherMemberProfileIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (*fixedMessageEntitlement) MemberRole(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "member", nil
}

func TestRequireMessageReadEntitlementForwardsImmutableMessageTime(t *testing.T) {
	createdAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	guard := &fixedMessageEntitlement{allowed: true}
	service := &MessagingGRPC{ChatGuard: guard}
	chatID, profileID := uuid.New(), uuid.New()
	require.NoError(t, service.requireMessageReadEntitlement(context.Background(), chatID, profileID, createdAt))
	require.Equal(t, createdAt, guard.seenAt)

	guard.allowed = false
	err := service.requireMessageReadEntitlement(context.Background(), chatID, profileID, createdAt)
	require.Equal(t, codes.NotFound, status.Code(err))
}
