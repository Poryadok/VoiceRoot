// Package gifcatalog defines Chat's provider-neutral GIF search boundary.
package gifcatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
)

const (
	DefaultSearchLimit = 20
	MaxSearchLimit     = 50
)

var ErrInvalidSearchRequest = errors.New("invalid GIF search request")

// ErrRateLimited lets provider adapters report throttling without gRPC coupling.
var ErrRateLimited = errors.New("GIF provider rate limit exceeded")

// Catalog is implemented by a selected provider adapter or a deterministic local fake.
// Chat owns the cursor in Page; File and Messaging do not paginate GIF search.
type Catalog interface {
	Search(context.Context, SearchRequest) (Page, error)
	Trending(context.Context, PageRequest) (Page, error)
}

type SearchRequest struct {
	Query  string
	Limit  int
	Cursor string
}

type PageRequest struct {
	Limit  int
	Cursor string
}

// Result is the provider-neutral form of the documented Chat GifResult sketch.
// FileID is empty while a later import has not made the GIF ready yet.
type Result struct {
	Provider        string
	ProviderID      string
	FileID          string
	PreviewURL      string
	Width           int
	Height          int
	DurationSeconds float64
}

type Page struct {
	Items      []Result
	NextCursor string
}

// Fixture keeps deterministic local search metadata separate from the result
// returned to consumers. It is only for fake-provider tests and local development.
type Fixture struct {
	Result Result
	Tags   []string
}

type fakeCatalog struct {
	fixtures []Fixture
}

var _ Catalog = fakeCatalog{}

func NewFakeCatalog(fixtures ...Fixture) Catalog {
	cloned := make([]Fixture, len(fixtures))
	for i, fixture := range fixtures {
		cloned[i] = Fixture{Result: fixture.Result, Tags: append([]string(nil), fixture.Tags...)}
	}
	return fakeCatalog{fixtures: cloned}
}

func (c fakeCatalog) Search(_ context.Context, request SearchRequest) (Page, error) {
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return Page{}, ErrInvalidSearchRequest
	}
	return page(c.matches(query), PageRequest{Limit: request.Limit, Cursor: request.Cursor}, "search:"+strings.ToLower(query))
}

func (c fakeCatalog) Trending(_ context.Context, request PageRequest) (Page, error) {
	results := make([]Result, len(c.fixtures))
	for i, fixture := range c.fixtures {
		results[i] = fixture.Result
	}
	return page(results, request, "trending")
}

func page(results []Result, request PageRequest, scope string) (Page, error) {
	if request.Limit < 0 || request.Limit > MaxSearchLimit {
		return Page{}, ErrInvalidSearchRequest
	}
	limit := request.Limit
	if limit == 0 {
		limit = DefaultSearchLimit
	}
	start, ok := cursorOffset(request.Cursor, len(results), scope)
	if !ok {
		return Page{}, ErrInvalidSearchRequest
	}
	end := min(start+limit, len(results))
	page := Page{Items: append([]Result(nil), results[start:end]...)}
	if end < len(results) {
		page.NextCursor = cursorPrefix(scope) + strconv.Itoa(end)
	}
	return page, nil
}

func (c fakeCatalog) matches(query string) []Result {
	needle := strings.ToLower(strings.TrimSpace(query))
	matches := make([]Result, 0, len(c.fixtures))
	for _, fixture := range c.fixtures {
		for _, tag := range fixture.Tags {
			if strings.EqualFold(tag, needle) {
				matches = append(matches, fixture.Result)
				break
			}
		}
	}
	return matches
}

func cursorPrefix(scope string) string {
	digest := sha256.Sum256([]byte(scope))
	return "fake:" + hex.EncodeToString(digest[:]) + ":"
}

func cursorOffset(cursor string, size int, scope string) (int, bool) {
	if cursor == "" {
		return 0, true
	}
	prefix := cursorPrefix(scope)
	if !strings.HasPrefix(cursor, prefix) {
		return 0, false
	}
	offset, err := strconv.Atoi(strings.TrimPrefix(cursor, prefix))
	if err != nil {
		return 0, false
	}
	return offset, offset >= 0 && offset < size
}
