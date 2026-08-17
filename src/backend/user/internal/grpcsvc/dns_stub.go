package grpcsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DNSResolverFromEnv returns an HTTP TXT fixture resolver when USER_DNS_STUB_URL is set
// (compose live ITs). Nil means use the system DNS resolver.
func DNSResolverFromEnv() DNSResolver {
	endpoint := strings.TrimSpace(os.Getenv("USER_DNS_STUB_URL"))
	if endpoint == "" {
		return nil
	}
	return NewHTTPTXTResolver(endpoint)
}

// NewHTTPTXTResolver looks up TXT records via GET {endpoint}?domain=.
func NewHTTPTXTResolver(endpoint string) DNSResolver {
	return &httpTXTResolver{
		endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

type httpTXTResolver struct {
	endpoint string
	client   *http.Client
}

type httpTXTResponse struct {
	Records []string `json:"records"`
}

func (r *httpTXTResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	u, err := url.Parse(r.endpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("domain", strings.TrimSpace(domain))
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dns stub status %d", resp.StatusCode)
	}
	var parsed httpTXTResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.Records == nil {
		return []string{}, nil
	}
	return parsed.Records, nil
}
