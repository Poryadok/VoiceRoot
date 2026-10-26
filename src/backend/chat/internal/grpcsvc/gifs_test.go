package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/gifcatalog"
)

func TestGIFRPCAuthenticationAndDisabled(t *testing.T) {
	for _, trending := range []bool{false, true} {
		for _, authenticated := range []bool{false, true} {
			ctx := context.Background()
			want := codes.Unauthenticated
			if authenticated {
				ctx = stickerContext(uuid.New())
				want = codes.Unavailable
			}
			s := &ChatGRPC{}
			var err error
			if trending {
				_, err = s.GetTrendingGifs(ctx, &chatv1.GetTrendingGifsRequest{})
			} else {
				_, err = s.SearchGifs(ctx, &chatv1.SearchGifsRequest{Query: "cat"})
			}
			if status.Code(err) != want {
				t.Fatalf("trending=%v auth=%v: got %v want %v", trending, authenticated, err, want)
			}
		}
	}
}

func TestGIFRPCFakePaginationAndBounds(t *testing.T) {
	ctx := stickerContext(uuid.New())
	s := &ChatGRPC{GIFCatalog: gifcatalog.NewFakeCatalog(
		gifcatalog.Fixture{Result: gifcatalog.Result{Provider: "fixture", ProviderID: "first", PreviewURL: "https://example.test/1", Width: 10, Height: 20, DurationSeconds: 1.5}, Tags: []string{"cat"}},
		gifcatalog.Fixture{Result: gifcatalog.Result{Provider: "fixture", ProviderID: "second"}, Tags: []string{"cat"}},
	)}
	first, err := s.SearchGifs(ctx, &chatv1.SearchGifsRequest{Query: " cat ", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.Items[0].ProviderId != "first" || first.Items[0].FileId != "" || first.Items[0].Width != 10 || first.Items[0].DurationSeconds != 1.5 || first.NextCursor == "" {
		t.Fatalf("first=%+v", first)
	}
	second, err := s.SearchGifs(ctx, &chatv1.SearchGifsRequest{Query: "cat", Limit: 1, Cursor: first.NextCursor})
	if err != nil || len(second.GetItems()) != 1 || second.Items[0].ProviderId != "second" || second.NextCursor != "" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	trend, err := s.GetTrendingGifs(ctx, &chatv1.GetTrendingGifsRequest{})
	if err != nil || len(trend.GetItems()) != 2 {
		t.Fatalf("trend=%+v err=%v", trend, err)
	}
	for _, req := range []*chatv1.SearchGifsRequest{{}, {Query: "  "}, {Query: "cat", Limit: -1}, {Query: "cat", Limit: 51}, {Query: "cat", Cursor: "foreign"}} {
		if _, err := s.SearchGifs(ctx, req); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("%+v: %v", req, err)
		}
	}
	for _, req := range []*chatv1.GetTrendingGifsRequest{{Limit: -1}, {Limit: 51}, {Cursor: "foreign"}} {
		if _, err := s.GetTrendingGifs(ctx, req); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("%+v: %v", req, err)
		}
	}
}

type failedGIFCatalog struct{ err error }

func (c failedGIFCatalog) Search(context.Context, gifcatalog.SearchRequest) (gifcatalog.Page, error) {
	return gifcatalog.Page{}, c.err
}
func (c failedGIFCatalog) Trending(context.Context, gifcatalog.PageRequest) (gifcatalog.Page, error) {
	return gifcatalog.Page{}, c.err
}

func TestGIFRPCErrorsAreSanitized(t *testing.T) {
	for _, err := range []error{errors.New("provider secret"), gifcatalog.ErrRateLimited, status.Error(codes.ResourceExhausted, "provider secret"), context.DeadlineExceeded, context.Canceled} {
		s := &ChatGRPC{GIFCatalog: failedGIFCatalog{err}}
		_, got := s.SearchGifs(stickerContext(uuid.New()), &chatv1.SearchGifsRequest{Query: "cat"})
		want := codes.Unavailable
		if status.Code(err) == codes.ResourceExhausted || errors.Is(err, gifcatalog.ErrRateLimited) {
			want = codes.ResourceExhausted
		}
		if errors.Is(err, context.DeadlineExceeded) {
			want = codes.DeadlineExceeded
		}
		if errors.Is(err, context.Canceled) {
			want = codes.Canceled
		}
		if status.Code(got) != want || status.Convert(got).Message() == "provider secret" {
			t.Fatalf("err=%v got=%v", err, got)
		}
	}
}
