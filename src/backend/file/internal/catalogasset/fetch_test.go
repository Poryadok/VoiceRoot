package catalogasset

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

const gifBytes = "GIF89a\x01\x00\x01\x00\x00\x00\x00;"

func testFetcher(t *testing.T, handler http.Handler) (*Fetcher, *int, *[]string) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	f, err := NewFetcher(map[string][]string{"fixture": {"example.com", "cdn.example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	f.tlsConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	calls := 0
	addresses := []string{}
	f.lookup = func(context.Context, string) ([]netip.Addr, error) {
		calls++
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	f.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		addresses = append(addresses, address)
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	return f, &calls, &addresses
}

func TestFetchPinsValidatedAddressAndKeepsTLSHostname(t *testing.T) {
	f, calls, addresses := testFetcher(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "example.com" {
			t.Errorf("host = %s", r.Host)
		}
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write([]byte(gifBytes))
	}))
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	for range 2 {
		body, err := f.FetchGIF(context.Background(), "fixture", "https://example.com/asset.gif", 1024)
		if err != nil || string(body) != gifBytes {
			t.Fatalf("body %q, err %v", body, err)
		}
	}
	if *calls != 2 || len(*addresses) != 2 || (*addresses)[0] != "8.8.8.8:443" {
		t.Fatalf("DNS %d, connections %v", *calls, *addresses)
	}
}

func TestFetchRejectsUntrustedURLBeforeDNS(t *testing.T) {
	f, calls, addresses := testFetcher(t, http.NotFoundHandler())
	for _, raw := range []string{"http://example.com/x", "https://example.com:8443/x", "https://example.com@evil.test/x", "https://example.com.evil.test/x", "https://127.0.0.1/x", "https://[::1]/x", "https://example.com./x", "https://example.com/x#fragment"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := f.FetchGIF(context.Background(), "fixture", raw, 1024); !errors.Is(err, ErrSource) {
				t.Fatalf("err %v", err)
			}
		})
	}
	if _, err := f.FetchGIF(context.Background(), "other", "https://example.com/x", 1024); !errors.Is(err, ErrSource) {
		t.Fatal(err)
	}
	if *calls != 0 || len(*addresses) != 0 {
		t.Fatal("untrusted URL reached network")
	}
}

func TestFetchRejectsEveryUnsafeDNSAnswerBeforeConnect(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "192.0.0.9", "192.0.2.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "240.0.0.1", "::1", "::ffff:127.0.0.1", "fe80::1", "fc00::1", "2001:db8::1", "2002:7f00:1::1", "64:ff9b::7f00:1"} {
		t.Run(address, func(t *testing.T) {
			f, _, addresses := testFetcher(t, http.NotFoundHandler())
			f.lookup = func(context.Context, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr(address)}, nil
			}
			if _, err := f.FetchGIF(context.Background(), "fixture", "https://example.com/x", 1024); !errors.Is(err, ErrSource) {
				t.Fatalf("err %v", err)
			}
			if len(*addresses) != 0 {
				t.Fatal("unsafe DNS answer reached dialer")
			}
		})
	}
}

func TestFetchRevalidatesRedirectAndDNSRebinding(t *testing.T) {
	for _, location := range []string{"http://example.com/plain", "https://evil.test/x", "https://127.0.0.1/x", "https://cdn.example.com/x", "/again"} {
		t.Run(location, func(t *testing.T) {
			f, calls, addresses := testFetcher(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, location, http.StatusFound) }))
			f.lookup = func(context.Context, string) ([]netip.Addr, error) {
				*calls++
				if *calls > 1 {
					return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
				}
				return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
			}
			if _, err := f.FetchGIF(context.Background(), "fixture", "https://example.com/x", 1024); !errors.Is(err, ErrSource) {
				t.Fatalf("err %v", err)
			}
			if len(*addresses) != 1 {
				t.Fatalf("unsafe redirect dialed: %v", *addresses)
			}
		})
	}
}

func TestFetchBoundsBodyAndRejectsNonGIF(t *testing.T) {
	for _, tc := range []struct {
		name, mime, encoding, body string
		status                     int
		limit                      int64
	}{
		{"too-large", "image/gif", "", gifBytes + strings.Repeat("x", 1024), 200, 32},
		{"wrong-mime", "text/plain", "", gifBytes, 200, 1024},
		{"wrong-bytes", "image/gif", "", "not a GIF", 200, 1024},
		{"compressed", "image/gif", "gzip", gifBytes, 200, 1024},
		{"bad-status", "image/gif", "", gifBytes, 500, 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _, _ := testFetcher(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.mime)
				w.Header().Set("Content-Encoding", tc.encoding)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			if _, err := f.FetchGIF(context.Background(), "fixture", "https://example.com/x", tc.limit); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

func TestFetchHonorsCancellationAndRejectsEmptyDNS(t *testing.T) {
	f, _, addresses := testFetcher(t, http.NotFoundHandler())
	f.lookup = func(context.Context, string) ([]netip.Addr, error) { return nil, nil }
	if _, err := f.FetchGIF(context.Background(), "fixture", "https://example.com/x", 1024); !errors.Is(err, ErrSource) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	if _, err := f.FetchGIF(ctx, "fixture", "https://example.com/x", 1024); err == nil {
		t.Fatal("cancel ignored")
	}
	if len(*addresses) != 0 {
		t.Fatal("unexpected dial")
	}
}

func TestFetchNetworkErrorsDoNotExposeSignedSourceURL(t *testing.T) {
	f, _, _ := testFetcher(t, http.NotFoundHandler())
	f.dial = func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("network unavailable") }
	_, err := f.FetchGIF(context.Background(), "fixture", "https://example.com/x?secret=private-provider-token", 1024)
	if !errors.Is(err, ErrSource) || strings.Contains(err.Error(), "private-provider-token") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestFetchUsesCopiedExactHostPolicyAndRejectsInvalidLimits(t *testing.T) {
	policy := map[string][]string{"fixture": {"example.com"}}
	f, err := NewFetcher(policy)
	if err != nil {
		t.Fatal(err)
	}
	policy["fixture"][0] = "evil.test"
	for _, limit := range []int64{0, -1, (50 << 20) + 1} {
		if _, err := f.FetchGIF(context.Background(), "fixture", "https://example.com/x", limit); !errors.Is(err, ErrSource) {
			t.Fatal(err)
		}
	}
	if _, err := f.FetchGIF(context.Background(), "fixture", "https://evil.test/x", 1024); !errors.Is(err, ErrSource) {
		t.Fatal("mutated host policy accepted")
	}
}
