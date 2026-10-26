package gifcatalog

import (
	"context"
	"errors"
	"sync"
	"time"
)

// NewTrendingCache bounds server-side metadata caching. It never grants File
// authority. Provider activation supplies its own TTL and maximum page count.
func NewTrendingCache(source Catalog, ttl time.Duration, maxPages int) (Catalog, error) {
	if source == nil || ttl <= 0 || maxPages <= 0 {
		return nil, errors.New("GIF cache requires source, positive TTL and capacity")
	}
	return &trendingCache{source: source, ttl: ttl, maxPages: maxPages, now: time.Now, pages: make(map[PageRequest]cachedPage)}, nil
}

type cachedPage struct {
	page    Page
	expires time.Time
}
type trendingCache struct {
	source   Catalog
	ttl      time.Duration
	maxPages int
	now      func() time.Time
	mu       sync.Mutex
	pages    map[PageRequest]cachedPage
}

func (c *trendingCache) Search(ctx context.Context, req SearchRequest) (Page, error) {
	return c.source.Search(ctx, req)
}
func (c *trendingCache) Trending(ctx context.Context, req PageRequest) (Page, error) {
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	if req.Limit < 0 || req.Limit > MaxSearchLimit {
		return Page{}, ErrInvalidSearchRequest
	}
	if req.Limit == 0 {
		req.Limit = DefaultSearchLimit
	}
	c.mu.Lock()
	entry, ok := c.pages[req]
	if ok && c.now().Before(entry.expires) {
		c.mu.Unlock()
		return copyPage(entry.page), nil
	}
	c.mu.Unlock()
	page, err := c.source.Trending(ctx, req)
	if err != nil {
		return Page{}, err
	}
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for key, value := range c.pages {
		if !now.Before(value.expires) {
			delete(c.pages, key)
		}
	}
	if _, exists := c.pages[req]; !exists && len(c.pages) >= c.maxPages {
		// Evict the earliest expiry; never grow with arbitrary cursor cardinality.
		var oldest PageRequest
		var expiry time.Time
		for key, value := range c.pages {
			if expiry.IsZero() || value.expires.Before(expiry) {
				oldest = key
				expiry = value.expires
			}
		}
		delete(c.pages, oldest)
	}
	c.pages[req] = cachedPage{page: copyPage(page), expires: now.Add(c.ttl)}
	return copyPage(page), nil
}
func copyPage(page Page) Page { page.Items = append([]Result(nil), page.Items...); return page }
