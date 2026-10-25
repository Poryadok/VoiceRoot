package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/gifcatalog"
)

func (s *ChatGRPC) SearchGifs(ctx context.Context, req *chatv1.SearchGifsRequest) (*chatv1.SearchGifsResponse, error) {
	if err := s.requireGIFProfile(ctx); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(req.GetQuery())
	if query == "" || !validGIFLimit(req.GetLimit()) {
		return nil, status.Error(codes.InvalidArgument, "invalid GIF search request")
	}
	if s == nil || s.GIFCatalog == nil {
		return nil, status.Error(codes.Unavailable, "GIF search unavailable")
	}
	page, err := s.GIFCatalog.Search(ctx, gifcatalog.SearchRequest{Query: query, Limit: gifLimit(req.GetLimit()), Cursor: req.GetCursor()})
	if err != nil {
		return nil, gifError(err)
	}
	return &chatv1.SearchGifsResponse{Items: gifResults(page.Items), NextCursor: page.NextCursor}, nil
}

func (s *ChatGRPC) GetTrendingGifs(ctx context.Context, req *chatv1.GetTrendingGifsRequest) (*chatv1.GetTrendingGifsResponse, error) {
	if err := s.requireGIFProfile(ctx); err != nil {
		return nil, err
	}
	if !validGIFLimit(req.GetLimit()) {
		return nil, status.Error(codes.InvalidArgument, "invalid GIF search request")
	}
	if s == nil || s.GIFCatalog == nil {
		return nil, status.Error(codes.Unavailable, "GIF search unavailable")
	}
	page, err := s.GIFCatalog.Trending(ctx, gifcatalog.PageRequest{Limit: gifLimit(req.GetLimit()), Cursor: req.GetCursor()})
	if err != nil {
		return nil, gifError(err)
	}
	return &chatv1.GetTrendingGifsResponse{Items: gifResults(page.Items), NextCursor: page.NextCursor}, nil
}

func (*ChatGRPC) requireGIFProfile(ctx context.Context) error {
	if id, ok := authctx.ProfileID(ctx); !ok || id == uuid.Nil {
		return status.Error(codes.Unauthenticated, "missing profile")
	}
	return nil
}

func validGIFLimit(limit int32) bool { return limit >= 0 && limit <= gifcatalog.MaxSearchLimit }
func gifLimit(limit int32) int {
	if limit == 0 {
		return gifcatalog.DefaultSearchLimit
	}
	return int(limit)
}

func gifError(err error) error {
	switch {
	case errors.Is(err, gifcatalog.ErrInvalidSearchRequest):
		return status.Error(codes.InvalidArgument, "invalid GIF search request")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "GIF search canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "GIF search deadline exceeded")
	case errors.Is(err, gifcatalog.ErrRateLimited), status.Code(err) == codes.ResourceExhausted:
		return status.Error(codes.ResourceExhausted, "GIF search rate limit exceeded")
	default:
		return status.Error(codes.Unavailable, "GIF search unavailable")
	}
}

func gifResults(items []gifcatalog.Result) []*chatv1.GifResult {
	out := make([]*chatv1.GifResult, 0, len(items))
	for _, item := range items {
		out = append(out, &chatv1.GifResult{Provider: item.Provider, ProviderId: item.ProviderID, FileId: item.FileID, PreviewUrl: item.PreviewURL, Width: int32(item.Width), Height: int32(item.Height), DurationSeconds: float32(item.DurationSeconds)})
	}
	return out
}
