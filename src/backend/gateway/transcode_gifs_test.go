package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
)

type recordingGIFs struct {
	chatv1.UnimplementedChatServiceServer
	search   *chatv1.SearchGifsRequest
	trending *chatv1.GetTrendingGifsRequest
	profile  string
	err      error
}

func (s *recordingGIFs) SearchGifs(ctx context.Context, req *chatv1.SearchGifsRequest) (*chatv1.SearchGifsResponse, error) {
	s.search = req
	md, _ := metadata.FromIncomingContext(ctx)
	s.profile = strings.Join(md.Get("x-voice-profile-id"), ",")
	return &chatv1.SearchGifsResponse{Items: []*chatv1.GifResult{{Provider: "fixture", ProviderId: "one", PreviewUrl: "https://example.test/preview"}}, NextCursor: "opaque:next"}, s.err
}
func (s *recordingGIFs) GetTrendingGifs(_ context.Context, req *chatv1.GetTrendingGifsRequest) (*chatv1.GetTrendingGifsResponse, error) {
	s.trending = req
	return &chatv1.GetTrendingGifsResponse{}, s.err
}

func TestTranscodeGIFSearch(t *testing.T) {
	rec := &recordingGIFs{}
	conn, cleanup := startBufconnChatConn(t, rec)
	t.Cleanup(cleanup)
	h := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{"valid": {UserID: "account-1", ProfileID: "active-profile"}}, transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}}})
	headers := map[string]string{"Authorization": "Bearer valid"}
	response := performRequest(h, http.MethodGet, "/api/v1/gifs/search?q=cat%20dance&limit=2&cursor=opaque%3Afirst", "", headers)
	if response.Code != 200 || rec.search.GetQuery() != "cat dance" || rec.search.GetLimit() != 2 || rec.search.GetCursor() != "opaque:first" || rec.profile != "active-profile" {
		t.Fatalf("response=%d %s request=%v profile=%s", response.Code, response.Body, rec.search, rec.profile)
	}
	if !strings.Contains(response.Body.String(), `"next_cursor":"opaque:next"`) || !strings.Contains(response.Body.String(), `"provider_id":"one"`) {
		t.Fatalf("protobuf response=%s", response.Body)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache=%s", response.Header().Get("Cache-Control"))
	}
	trend := performRequest(h, http.MethodGet, "/api/v1/gifs/trending?limit=3&cursor=next", "", headers)
	if trend.Code != 200 || rec.trending.GetLimit() != 3 || rec.trending.GetCursor() != "next" {
		t.Fatalf("trend=%d req=%v", trend.Code, rec.trending)
	}
	rec.search = nil
	unauth := performRequest(h, http.MethodGet, "/api/v1/gifs/search?q=cat", "", nil)
	if unauth.Code != http.StatusUnauthorized || rec.search != nil {
		t.Fatalf("unauth=%d req=%v", unauth.Code, rec.search)
	}
	for _, path := range []string{"/search?q=cat&limit=abc", "/search?q=cat&limit=2147483648", "/search?q=cat&limit=-1", "/search?q=cat&limit=51", "/search?q=cat&limit=1&limit=2", "/search?q=cat&q=dog", "/search?q=cat&cursor=a&cursor=b", "/trending?limit=abc"} {
		got := performRequest(h, http.MethodGet, "/api/v1/gifs"+path, "", headers)
		if got.Code != http.StatusBadRequest || rec.search != nil {
			t.Fatalf("%s =>%d request=%v", path, got.Code, rec.search)
		}
	}
	for _, c := range []codes.Code{codes.ResourceExhausted, codes.Unavailable, codes.InvalidArgument} {
		rec.err = status.Error(c, "search unavailable")
		got := performRequest(h, http.MethodGet, "/api/v1/gifs/search?q=cat", "", headers)
		want := 503
		if c == codes.ResourceExhausted {
			want = 429
		}
		if c == codes.InvalidArgument {
			want = 400
		}
		if got.Code != want {
			t.Fatalf("%v =>%d %s", c, got.Code, got.Body)
		}
	}
}

func TestTranscodeGIFMissingClient(t *testing.T) {
	h := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{"valid": {UserID: "account-1", ProfileID: "profile-1"}}, transcoder: &transcoder{}})
	got := performRequest(h, http.MethodGet, "/api/v1/gifs/search?q=cat", "", map[string]string{"Authorization": "Bearer valid"})
	if got.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d %s", got.Code, got.Body)
	}
}
