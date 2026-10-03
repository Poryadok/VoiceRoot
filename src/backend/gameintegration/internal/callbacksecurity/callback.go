package callbacksecurity

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var ErrUnsafeURL = errors.New("unsafe callback URL")

// Resolver is the minimal DNS interface used both at registration and at dial time.
type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

// Dialer is the minimal socket interface used by the pinned HTTP transport.
type Dialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type systemResolver struct{}

func (systemResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, network, host)
}

type systemDialer struct{ net.Dialer }

func (d systemDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.Dialer.DialContext(ctx, network, address)
}

// ValidateURL validates a game installation callback at registration time.
// NewClient repeats the DNS checks at connection time and pins the dialed IP.
func ValidateURL(ctx context.Context, raw string, resolver Resolver) error {
	u, err := parseURL(raw)
	if err != nil {
		return err
	}
	_, err = resolvePublic(ctx, u.Hostname(), resolver)
	return err
}

// NewClient creates an HTTP client for registered game callbacks. DNS is
// resolved again on every new TCP connection; redirects are returned as-is.
func NewClient(resolver Resolver, dialer Dialer) *http.Client {
	if resolver == nil {
		resolver = systemResolver{}
	}
	if dialer == nil {
		dialer = systemDialer{}
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" && network != "tcp4" && network != "tcp6" {
				return nil, fmt.Errorf("%w: unsupported network", ErrUnsafeURL)
			}
			host, port, err := net.SplitHostPort(address)
			if err != nil || port != "443" {
				return nil, fmt.Errorf("%w: callback dial must use port 443", ErrUnsafeURL)
			}
			addresses, err := resolvePublic(ctx, host, resolver)
			if err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, "tcp", net.JoinHostPort(addresses[0].String(), port))
		},
	}
	return &http.Client{
		Transport: requestPolicyTransport{base: transport},
		Timeout:   3 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type requestPolicyTransport struct {
	base http.RoundTripper
}

func (t requestPolicyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, fmt.Errorf("%w: request URL is missing", ErrUnsafeURL)
	}
	if _, err := parseURL(request.URL.String()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(request)
}

func parseURL(raw string) (*url.URL, error) {
	if raw == "" || len(raw) > 2048 || !isASCII(raw) {
		return nil, fmt.Errorf("%w: URL must be bounded ASCII", ErrUnsafeURL)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil || u.Host == "" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" ||
		strings.ContainsAny(raw, "\r\n\\#") {
		return nil, fmt.Errorf("%w: URL components are not allowed", ErrUnsafeURL)
	}
	host := u.Hostname()
	if host == "" || strings.Contains(host, "%") || strings.HasSuffix(host, ".") {
		return nil, fmt.Errorf("%w: invalid callback hostname", ErrUnsafeURL)
	}
	if port := u.Port(); port != "" && port != "443" {
		return nil, fmt.Errorf("%w: callback port is not allowed", ErrUnsafeURL)
	}
	if u.Port() == "" && strings.Contains(u.Host, ":") && net.ParseIP(host) == nil {
		return nil, fmt.Errorf("%w: malformed callback authority", ErrUnsafeURL)
	}
	if !canonicalPath(u) {
		return nil, fmt.Errorf("%w: callback path is not canonical", ErrUnsafeURL)
	}
	return u, nil
}

func canonicalPath(u *url.URL) bool {
	path := u.EscapedPath()
	if path == "" || path[0] != '/' || strings.Contains(path, "%") {
		return false
	}
	for _, segment := range strings.Split(path[1:], "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, b := range []byte(segment) {
			if (b < '0' || b > '9') && (b < 'A' || b > 'Z') && (b < 'a' || b > 'z') &&
				b != '-' && b != '.' && b != '_' && b != '~' {
				return false
			}
		}
	}
	return true
}

func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

func resolvePublic(ctx context.Context, host string, resolver Resolver) ([]netip.Addr, error) {
	if resolver == nil {
		resolver = systemResolver{}
	}
	if literal, err := netip.ParseAddr(host); err == nil {
		literal = literal.Unmap()
		if !publicAddress(literal) {
			return nil, fmt.Errorf("%w: destination address is not public", ErrUnsafeURL)
		}
		return []netip.Addr{literal}, nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	addresses, err := resolver.LookupNetIP(lookupCtx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("%w: destination lookup failed", ErrUnsafeURL)
	}
	for i, address := range addresses {
		address = address.Unmap()
		if !publicAddress(address) {
			return nil, fmt.Errorf("%w: DNS answer is not public", ErrUnsafeURL)
		}
		addresses[i] = address
	}
	return addresses, nil
}

func publicAddress(address netip.Addr) bool {
	if !address.IsValid() || address.Zone() != "" || !address.IsGlobalUnicast() ||
		address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range specialUsePrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

var specialUsePrefixes = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.31.196.0/24", "192.52.193.0/24",
	"192.88.99.0/24", "192.168.0.0/16", "192.175.48.0/24", "198.18.0.0/15", "198.51.100.0/24",
	"203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "::ffff:0:0/96", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64",
	"2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20", "5f00::/16", "fc00::/7", "fe80::/10",
	"fec0::/10", "ff00::/8",
)

func mustPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}
