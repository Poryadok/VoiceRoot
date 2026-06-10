package mentions_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/mentions"
	"voice/backend/role/permissions"
)

type stubRoles struct {
	allowed map[string]bool
	err     error
}

func (s stubRoles) HasSpacePermission(_ context.Context, _, _ uuid.UUID, permission string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.allowed[permission], nil
}

type stubPresence struct {
	online []uuid.UUID
	err    error
}

func (s stubPresence) FilterOnlineProfileIDs(context.Context, []uuid.UUID) ([]uuid.UUID, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]uuid.UUID(nil), s.online...), nil
}

func TestProcess_userMentionInDM(t *testing.T) {
	ctx := context.Background()
	profA := uuid.New()
	profB := uuid.New()
	meta := mentions.ChatMeta{
		ChatID:   uuid.New(),
		ChatType: "dm",
		Members:  []uuid.UUID{profA, profB},
	}
	normalized, targets, err := mentions.Process(ctx, meta, profA, `[{"type":"user","target_id":"`+profB.String()+`"}]`, nil, nil)
	require.NoError(t, err)
	require.JSONEq(t, `[{"type":"user","target_id":"`+profB.String()+`"}]`, normalized)
	require.Equal(t, []uuid.UUID{profB}, targets)
}

func TestProcess_everyoneDeniedWithoutPermission(t *testing.T) {
	ctx := context.Background()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	meta := mentions.ChatMeta{
		ChatID:   uuid.New(),
		ChatType: "group",
		SpaceID:  &spaceID,
		Members:  []uuid.UUID{profA, profB},
	}
	_, _, err := mentions.Process(ctx, meta, profA, `[{"type":"everyone"}]`, stubRoles{allowed: map[string]bool{}}, nil)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestProcess_everyoneExpandsWithPermission(t *testing.T) {
	ctx := context.Background()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	meta := mentions.ChatMeta{
		ChatID:   uuid.New(),
		ChatType: "channel",
		SpaceID:  &spaceID,
		Members:  []uuid.UUID{profA, profB},
	}
	roles := stubRoles{allowed: map[string]bool{permissions.TextChatMentionAllInChat: true}}
	normalized, targets, err := mentions.Process(ctx, meta, profA, `[{"type":"everyone"}]`, roles, nil)
	require.NoError(t, err)
	require.JSONEq(t, `[{"type":"everyone"}]`, normalized)
	require.ElementsMatch(t, []uuid.UUID{profB}, targets)
}

func TestProcess_hereUsesOnlinePresence(t *testing.T) {
	ctx := context.Background()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	meta := mentions.ChatMeta{
		ChatID:   uuid.New(),
		ChatType: "group",
		SpaceID:  &spaceID,
		Members:  []uuid.UUID{profA, profB, profC},
	}
	roles := stubRoles{allowed: map[string]bool{permissions.TextChatMentionAllOnline: true}}
	presence := stubPresence{online: []uuid.UUID{profB}}
	_, targets, err := mentions.Process(ctx, meta, profA, `[{"type":"here"}]`, roles, presence)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{profB}, targets)
}

func TestProcess_invalidJSON(t *testing.T) {
	ctx := context.Background()
	meta := mentions.ChatMeta{ChatID: uuid.New(), ChatType: "dm", Members: []uuid.UUID{uuid.New()}}
	_, _, err := mentions.Process(ctx, meta, uuid.New(), `{bad`, nil, nil)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestProcess_userNotMember(t *testing.T) {
	ctx := context.Background()
	profA := uuid.New()
	meta := mentions.ChatMeta{ChatID: uuid.New(), ChatType: "dm", Members: []uuid.UUID{profA}}
	_, _, err := mentions.Process(ctx, meta, profA, `[{"type":"user","target_id":"`+uuid.New().String()+`"}]`, nil, nil)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestProcess_roleUnavailable(t *testing.T) {
	ctx := context.Background()
	spaceID := uuid.New()
	meta := mentions.ChatMeta{
		ChatID: uuid.New(), ChatType: "group", SpaceID: &spaceID, Members: []uuid.UUID{uuid.New()},
	}
	_, _, err := mentions.Process(ctx, meta, uuid.New(), `[{"type":"everyone"}]`, stubRoles{err: status.Error(codes.Unavailable, "down")}, nil)
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestProcess_hereRequiresPresence(t *testing.T) {
	ctx := context.Background()
	spaceID := uuid.New()
	profA := uuid.New()
	meta := mentions.ChatMeta{
		ChatID: uuid.New(), ChatType: "group", SpaceID: &spaceID, Members: []uuid.UUID{profA},
	}
	roles := stubRoles{allowed: map[string]bool{permissions.TextChatMentionAllOnline: true}}
	_, _, err := mentions.Process(ctx, meta, profA, `[{"type":"here"}]`, roles, nil)
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestProcess_broadcastForbiddenInDM(t *testing.T) {
	ctx := context.Background()
	profA := uuid.New()
	profB := uuid.New()
	meta := mentions.ChatMeta{
		ChatID:   uuid.New(),
		ChatType: "dm",
		Members:  []uuid.UUID{profA, profB},
	}
	_, _, err := mentions.Process(ctx, meta, profA, `[{"type":"everyone"}]`, stubRoles{allowed: map[string]bool{permissions.TextChatMentionAllInChat: true}}, nil)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
