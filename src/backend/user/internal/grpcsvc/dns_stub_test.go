package grpcsvc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPTXTResolver_LookupTXTReturnsPublishedRecords(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "riotgames.com", r.URL.Query().Get("domain"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"records": []string{"voice-verify=abc123", "unrelated=1"},
		})
	}))
	t.Cleanup(srv.Close)

	got, err := NewHTTPTXTResolver(srv.URL).LookupTXT(context.Background(), "riotgames.com")
	require.NoError(t, err)
	require.Equal(t, []string{"voice-verify=abc123", "unrelated=1"}, got)
}

func TestHTTPTXTResolver_LookupTXTEmptyWhenUnpublished(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"records": []string{}})
	}))
	t.Cleanup(srv.Close)

	got, err := NewHTTPTXTResolver(srv.URL).LookupTXT(context.Background(), "example.com")
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestDNSResolverFromEnv_NilWhenUnset(t *testing.T) {
	t.Setenv("USER_DNS_STUB_URL", "")
	require.Nil(t, DNSResolverFromEnv())
}

func TestDNSResolverFromEnv_UsesStubURL(t *testing.T) {
	t.Setenv("USER_DNS_STUB_URL", "http://verification-stub:4180/dns-txt")
	r, ok := DNSResolverFromEnv().(*httpTXTResolver)
	require.True(t, ok)
	require.Equal(t, "http://verification-stub:4180/dns-txt", r.endpoint)
}
