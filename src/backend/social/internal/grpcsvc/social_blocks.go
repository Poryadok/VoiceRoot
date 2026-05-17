package grpcsvc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/social/internal/authctx"
	"voice/backend/social/internal/store"

	socialv1 "voice.app/voice/social/v1"
)

const (
	blocksDefaultPage = 20
	blocksMaxPage     = 50
)

type blocksCursorPayload struct {
	CreatedAtUnixNano int64  `json:"ts"`
	ID                string `json:"id"`
}

func encodeBlocksCursor(createdAt time.Time, id uuid.UUID) (string, error) {
	p := blocksCursorPayload{CreatedAtUnixNano: createdAt.UnixNano(), ID: id.String()}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return "b1." + base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeBlocksCursor(s string) (*store.BlocksListCursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, "b1.") {
		return nil, errors.New("invalid blocks cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(s, "b1."))
	if err != nil {
		return nil, errors.New("invalid blocks cursor")
	}
	var p blocksCursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.New("invalid blocks cursor")
	}
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return nil, errors.New("invalid blocks cursor")
	}
	ts := time.Unix(0, p.CreatedAtUnixNano).UTC()
	return &store.BlocksListCursor{CreatedAt: ts, ID: id}, nil
}

// BlockAccount implements voice.social.v1.SocialService.
func (s *SocialGRPC) BlockAccount(ctx context.Context, req *socialv1.BlockAccountRequest) (*socialv1.BlockAccountResponse, error) {
	blocker, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	blocked, err := parseUUIDField("blocked_account_id", req.GetBlockedAccountId())
	if err != nil {
		return nil, err
	}
	if s.Blocks == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	err = s.Blocks.BlockAccount(ctx, blocker, blocked)
	switch {
	case err == nil:
		return &socialv1.BlockAccountResponse{}, nil
	case errors.Is(err, store.ErrSelfBlock):
		return nil, status.Error(codes.InvalidArgument, err.Error())
	default:
		return nil, status.Error(codes.Internal, err.Error())
	}
}

// UnblockAccount implements voice.social.v1.SocialService.
func (s *SocialGRPC) UnblockAccount(ctx context.Context, req *socialv1.UnblockAccountRequest) (*socialv1.UnblockAccountResponse, error) {
	blocker, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	blocked, err := parseUUIDField("blocked_account_id", req.GetBlockedAccountId())
	if err != nil {
		return nil, err
	}
	if s.Blocks == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	err = s.Blocks.UnblockAccount(ctx, blocker, blocked)
	if err != nil {
		if errors.Is(err, store.ErrBlockNotFound) {
			return nil, status.Error(codes.NotFound, "block not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.UnblockAccountResponse{}, nil
}

// ListBlocked implements voice.social.v1.SocialService.
func (s *SocialGRPC) ListBlocked(ctx context.Context, req *socialv1.ListBlockedRequest) (*socialv1.ListBlockedResponse, error) {
	blocker, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	if s.Blocks == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	page := req.GetPage()
	pageSize := blocksDefaultPage
	if page != nil && page.GetPageSize() > 0 {
		pageSize = int(page.GetPageSize())
	}
	if pageSize > blocksMaxPage {
		pageSize = blocksMaxPage
	}
	cursorIn := ""
	if page != nil {
		cursorIn = page.GetCursor()
	}
	after, err := decodeBlocksCursor(cursorIn)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rows, err := s.Blocks.ListBlocked(ctx, blocker, after, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	out := make([]*socialv1.BlockedAccount, 0, len(rows))
	var next string
	for i, r := range rows {
		out = append(out, &socialv1.BlockedAccount{
			BlockedAccountId: r.BlockedAccountID.String(),
			CreatedAt:        timestamppb.New(r.CreatedAt.UTC()),
		})
		if hasMore && i == len(rows)-1 {
			next, err = encodeBlocksCursor(r.CreatedAt, r.BlockID)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	return &socialv1.ListBlockedResponse{
		BlockedList: &socialv1.BlockedList{
			Blocked:    out,
			NextCursor: next,
		},
	}, nil
}

// IsBlocked implements voice.social.v1.SocialService (internal S2S: Chat/Messaging/User).
// Returns whether account_id_a has blocked account_id_b (ordered); callers check both directions for mutual exclusion.
// Does not require end-user gRPC metadata.
func (s *SocialGRPC) IsBlocked(ctx context.Context, req *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
	blocker, err := parseUUIDField("account_id_a", req.GetAccountIdA())
	if err != nil {
		return nil, err
	}
	blocked, err := parseUUIDField("account_id_b", req.GetAccountIdB())
	if err != nil {
		return nil, err
	}
	if s.Blocks == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	if blocker == blocked {
		return &socialv1.IsBlockedResponse{Blocked: false}, nil
	}
	ok, err := s.Blocks.DirectedBlockExists(ctx, blocker, blocked)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.IsBlockedResponse{Blocked: ok}, nil
}
