package gifcatalog

import (
	"context"
	"errors"
	"testing"
	"time"
)

type countingCatalog struct {
	calls int
	err   error
}

func (c *countingCatalog) Search(context.Context, SearchRequest) (Page, error) {
	c.calls++
	return Page{}, c.err
}
func (c *countingCatalog) Trending(context.Context, PageRequest) (Page, error) {
	c.calls++
	return Page{Items: []Result{{ProviderID: "one"}}}, c.err
}

func TestTrendingCacheCopiesBoundsAndDoesNotCacheErrors(t *testing.T) {
	ctx := context.Background()
	source := &countingCatalog{}
	cached, err := NewTrendingCache(source, time.Minute, 1)
	if err != nil {
		t.Fatal(err)
	}
	page, err := cached.Trending(ctx, PageRequest{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	page.Items[0].ProviderID = "mutated"
	again, err := cached.Trending(ctx, PageRequest{Limit: 1})
	if err != nil || source.calls != 1 || again.Items[0].ProviderID != "one" {
		t.Fatalf("calls=%d page=%v err=%v", source.calls, again, err)
	}
	_, _ = cached.Trending(ctx, PageRequest{Limit: 2})
	_, _ = cached.Trending(ctx, PageRequest{Limit: 1})
	if source.calls != 3 {
		t.Fatalf("capacity calls=%d", source.calls)
	}
	source.err = errors.New("temporary")
	for range 2 {
		_, err = cached.Trending(ctx, PageRequest{Cursor: "next", Limit: 1})
		if !errors.Is(err, source.err) {
			t.Fatal(err)
		}
	}
	if source.calls != 5 {
		t.Fatalf("error cached calls=%d", source.calls)
	}
	for range 2 {
		_, _ = cached.Search(ctx, SearchRequest{Query: "cat"})
	}
	if source.calls != 7 {
		t.Fatalf("search cached calls=%d", source.calls)
	}
}

func TestTrendingCacheExpiryCancellationAndValidation(t *testing.T) {
	source := &countingCatalog{}
	cached, err := NewTrendingCache(source, time.Minute, 2)
	if err != nil {
		t.Fatal(err)
	}
	cache := cached.(*trendingCache)
	now := time.Unix(100, 0)
	cache.now = func() time.Time { return now }
	ctx := context.Background()
	_, _ = cached.Trending(ctx, PageRequest{})
	_, _ = cached.Trending(ctx, PageRequest{Limit: DefaultSearchLimit})
	if source.calls != 1 {
		t.Fatalf("default limit key calls=%d", source.calls)
	}
	now = now.Add(time.Minute)
	_, _ = cached.Trending(ctx, PageRequest{})
	if source.calls != 2 {
		t.Fatalf("expired calls=%d", source.calls)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := cached.Trending(canceled, PageRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := cached.Trending(ctx, PageRequest{Limit: MaxSearchLimit + 1}); !errors.Is(err, ErrInvalidSearchRequest) {
		t.Fatal(err)
	}
	if source.calls != 2 {
		t.Fatalf("invalid/canceled consulted source calls=%d", source.calls)
	}
	for _, in := range []struct {
		source Catalog
		ttl    time.Duration
		pages  int
	}{{nil, time.Minute, 1}, {source, 0, 1}, {source, time.Minute, 0}} {
		if _, err := NewTrendingCache(in.source, in.ttl, in.pages); err == nil {
			t.Fatal("accepted invalid cache configuration")
		}
	}
}
