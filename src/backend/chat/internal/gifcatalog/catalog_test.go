package gifcatalog

import (
	"context"
	"errors"
	"testing"
)

func TestFakeCatalogSearchRejectsInvalidRequests(t *testing.T) {
	catalog := NewFakeCatalog()

	for _, request := range []SearchRequest{
		{Query: " \t"},
		{Query: "cat", Limit: -1},
		{Query: "cat", Limit: MaxSearchLimit + 1},
	} {
		_, err := catalog.Search(context.Background(), request)
		if !errors.Is(err, ErrInvalidSearchRequest) {
			t.Fatalf("Search(%+v) error = %v, want ErrInvalidSearchRequest", request, err)
		}
	}
}

func TestFakeCatalogSearchReturnsStableProviderNeutralPages(t *testing.T) {
	catalog := NewFakeCatalog(
		Fixture{Result: Result{Provider: "fixture", ProviderID: "cat-1", PreviewURL: "https://fixture.invalid/cat-1.gif", Width: 320, Height: 180, DurationSeconds: 1.2}, Tags: []string{"cat", "hello"}},
		Fixture{Result: Result{Provider: "fixture", ProviderID: "cat-2", PreviewURL: "https://fixture.invalid/cat-2.gif", Width: 400, Height: 300, DurationSeconds: 2.4}, Tags: []string{"cat", "wave"}},
		Fixture{Result: Result{Provider: "fixture", ProviderID: "dog-1", PreviewURL: "https://fixture.invalid/dog-1.gif", Width: 480, Height: 270, DurationSeconds: 3.6}, Tags: []string{"dog"}},
	)

	first, err := catalog.Search(context.Background(), SearchRequest{Query: "CAT", Limit: 1})
	if err != nil {
		t.Fatalf("first Search() error = %v", err)
	}
	if got, want := first.Items, []Result{{Provider: "fixture", ProviderID: "cat-1", PreviewURL: "https://fixture.invalid/cat-1.gif", Width: 320, Height: 180, DurationSeconds: 1.2}}; !equalResults(got, want) {
		t.Fatalf("first items = %#v, want %#v", got, want)
	}
	if first.NextCursor == "" {
		t.Fatal("first page did not provide a next cursor")
	}

	second, err := catalog.Search(context.Background(), SearchRequest{Query: "cat", Limit: 1, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("second Search() error = %v", err)
	}
	if got, want := second.Items, []Result{{Provider: "fixture", ProviderID: "cat-2", PreviewURL: "https://fixture.invalid/cat-2.gif", Width: 400, Height: 300, DurationSeconds: 2.4}}; !equalResults(got, want) {
		t.Fatalf("second items = %#v, want %#v", got, want)
	}
	if second.NextCursor != "" {
		t.Fatalf("second next cursor = %q, want empty", second.NextCursor)
	}
}

func TestFakeCatalogSearchUsesDocumentedDefaultLimit(t *testing.T) {
	catalog := NewFakeCatalog(
		Fixture{Result: Result{Provider: "fixture", ProviderID: "one"}, Tags: []string{"cat"}},
		Fixture{Result: Result{Provider: "fixture", ProviderID: "two"}, Tags: []string{"cat"}},
	)

	page, err := catalog.Search(context.Background(), SearchRequest{Query: "cat"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(page.Items) != 2 || page.NextCursor != "" {
		t.Fatalf("default page = %#v, want both fixtures without cursor", page)
	}
}

func TestFakeCatalogTrendingUsesStableFixtureOrderAndPagination(t *testing.T) {
	catalog := NewFakeCatalog(
		Fixture{Result: Result{Provider: "fixture", ProviderID: "first"}},
		Fixture{Result: Result{Provider: "fixture", ProviderID: "second"}},
	)

	first, err := catalog.Trending(context.Background(), PageRequest{Limit: 1})
	if err != nil {
		t.Fatalf("first Trending() error = %v", err)
	}
	if got, want := first.Items, []Result{{Provider: "fixture", ProviderID: "first"}}; !equalResults(got, want) {
		t.Fatalf("first items = %#v, want %#v", got, want)
	}
	if first.NextCursor == "" {
		t.Fatal("first trending page did not provide a next cursor")
	}

	second, err := catalog.Trending(context.Background(), PageRequest{Limit: 1, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("second Trending() error = %v", err)
	}
	if got, want := second.Items, []Result{{Provider: "fixture", ProviderID: "second"}}; !equalResults(got, want) {
		t.Fatalf("second items = %#v, want %#v", got, want)
	}
	if second.NextCursor != "" {
		t.Fatalf("second next cursor = %q, want empty", second.NextCursor)
	}
}

func equalResults(got, want []Result) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
