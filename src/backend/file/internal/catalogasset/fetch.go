// Package catalogasset implements byte retrieval for protected catalog imports.
// It grants neither source authority nor permission to send to a destination.
package catalogasset

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var ErrSource = errors.New("catalog asset source is not allowed")

const maxImportBytes int64 = 50 << 20
const maxImportTime = 20 * time.Second

// Fetcher has no caller-supplied HTTP client, proxy or transport. Host policy is
// deployment-owned and copied on construction; it never comes from a selection.
type Fetcher struct {
	hosts     map[string]map[string]bool
	lookup    func(context.Context, string) ([]netip.Addr, error)
	dial      func(context.Context, string, string) (net.Conn, error)
	tlsConfig *tls.Config
}

func NewFetcher(providerHosts map[string][]string) (*Fetcher, error) {
	f := &Fetcher{hosts: make(map[string]map[string]bool), lookup: func(ctx context.Context, host string) ([]netip.Addr, error) {
		return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	}, dial: (&net.Dialer{Timeout: 5 * time.Second}).DialContext}
	for provider, hosts := range providerHosts {
		if provider == "" || provider != strings.TrimSpace(provider) || len(hosts) == 0 {
			return nil, ErrSource
		}
		f.hosts[provider] = make(map[string]bool, len(hosts))
		for _, host := range hosts {
			if !validHostname(host) {
				return nil, ErrSource
			}
			f.hosts[provider][host] = true
		}
	}
	return f, nil
}

func validHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 || host != strings.ToLower(host) || !strings.Contains(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func (f *Fetcher) validateURL(provider string, u *url.URL) error {
	if f == nil || u == nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil || u.Fragment != "" || u.RawFragment != "" || (u.Port() != "" && u.Port() != "443") || !validHostname(u.Hostname()) || !f.hosts[provider][u.Hostname()] {
		return ErrSource
	}
	return nil
}

var deniedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/3"),
	netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("3fff::/20"),
}

func publicAddress(address netip.Addr) bool {
	if !address.IsValid() || address.Zone() != "" {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() {
		return false
	}
	if address.Is6() && !netip.MustParsePrefix("2000::/3").Contains(address) {
		return false
	}
	for _, network := range deniedNetworks {
		if network.Contains(address) {
			return false
		}
	}
	return true
}

// FetchGIF receives an internal Chat-resolved URL only after principal and
// source binding validation. It pins each connection to the just-validated IP;
// TLS still verifies the original hostname. All DNS answers must be public.
func (f *Fetcher) FetchGIF(ctx context.Context, provider, sourceURL string, maxBytes int64) ([]byte, error) {
	u, err := url.Parse(sourceURL)
	if err != nil || f.validateURL(provider, u) != nil || maxBytes <= 0 || maxBytes > maxImportBytes {
		return nil, ErrSource
	}
	ctx, cancel := context.WithTimeout(ctx, maxImportTime)
	defer cancel()
	transport := &http.Transport{
		Proxy: nil, DisableKeepAlives: true, DisableCompression: true,
		TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second,
		MaxResponseHeaderBytes: 16 << 10,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil || port != "443" || !f.hosts[provider][host] {
				return nil, ErrSource
			}
			addresses, err := f.lookup(ctx, host)
			if err != nil || len(addresses) == 0 {
				return nil, ErrSource
			}
			for _, ip := range addresses {
				if !publicAddress(ip) {
					return nil, ErrSource
				}
			}
			// Do not give the hostname to a second resolver in the dialer.
			return f.dial(ctx, network, net.JoinHostPort(addresses[0].Unmap().String(), port))
		},
	}
	if f.tlsConfig != nil {
		transport.TLSClientConfig = f.tlsConfig.Clone()
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: maxImportTime, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Go otherwise forwards the previous full URL, including signed queries.
		req.Header.Del("Referer")
		if len(via) > 3 {
			return ErrSource
		}
		return f.validateURL(provider, req.URL)
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrSource
	}
	req.Header.Set("Accept", "image/gif")
	response, err := client.Do(req)
	if err != nil {
		// net/http errors contain the full URL, including provider query secrets.
		return nil, errors.Join(ErrSource, ctx.Err())
	}
	defer func() { _ = response.Body.Close() }()
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	encoding := response.Header.Get("Content-Encoding")
	if err != nil || mediaType != "image/gif" || response.StatusCode != http.StatusOK || response.ContentLength > maxBytes || (encoding != "" && encoding != "identity") {
		return nil, ErrSource
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, errors.Join(ErrSource, ctx.Err())
	}
	if int64(len(body)) > maxBytes || http.DetectContentType(body) != "image/gif" {
		return nil, ErrSource
	}
	return body, nil
}
