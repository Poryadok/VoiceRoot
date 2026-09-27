package callbacksecurity

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func TestValidateURLRequiresCanonicalHTTPS443AndPublicDNS(t *testing.T) {
	resolver := resolverFunc(func(_ context.Context, network, host string) ([]netip.Addr, error) {
		require.Equal(t, "ip", network)
		require.NotEmpty(t, host)
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	})

	require.NoError(t, ValidateURL(context.Background(), "https://game.example:443/callback-v1", resolver))
	for _, raw := range []string{
		"http://game.example/callback-v1",
		"https://game.example:8443/callback-v1",
		"https://user:pass@game.example/callback-v1",
		"https://game.example/callback-v1?token=secret",
		"https://game.example/callback-v1#fragment",
		"https://game.example/callback-v1#",
		"https://game.example/callback%2fv1",
		"https://game.example/a/../callback",
		"https://game.example//callback",
		"https://game.example/callback/é",
		"https://game.example/callback;v1",
		"https://game.example/callback:v1",
		"https://game.example/callback?",
		"https://gáme.example/callback",
	} {
		t.Run(raw, func(t *testing.T) {
			require.ErrorIs(t, ValidateURL(context.Background(), raw, resolver), ErrUnsafeURL)
		})
	}
}

func TestValidateURLRejectsNonPublicDNSAnswers(t *testing.T) {
	addresses := []string{
		"0.0.0.0", "10.1.2.3", "100.64.0.1", "127.0.0.1", "169.254.169.254",
		"172.16.0.1", "192.0.0.1", "192.0.2.1", "192.88.99.1", "192.168.1.1",
		"198.18.0.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "240.0.0.1",
		"::", "::1", "::ffff:127.0.0.1", "64:ff9b::a00:1", "100::1",
		"2001::1", "2001:db8::1", "2002:7f00:1::1", "3fff::1", "5f00::1",
		"fc00::1", "fe80::1", "fec0::1", "ff02::1",
	}
	for _, address := range addresses {
		t.Run(address, func(t *testing.T) {
			resolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr(address)}, nil
			})
			require.ErrorIs(t, ValidateURL(context.Background(), "https://callback.example/hook", resolver), ErrUnsafeURL)
		})
	}

	mixedResolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("10.0.0.7")}, nil
	})
	require.ErrorIs(t, ValidateURL(context.Background(), "https://callback.example/hook", mixedResolver), ErrUnsafeURL)

	failingResolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return nil, errors.New("DNS unavailable")
	})
	require.ErrorIs(t, ValidateURL(context.Background(), "https://callback.example/hook", failingResolver), ErrUnsafeURL)
	emptyResolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{}, nil
	})
	require.ErrorIs(t, ValidateURL(context.Background(), "https://callback.example/hook", emptyResolver), ErrUnsafeURL)
}

func TestClientRevalidatesAndPinsAddressAtDialTime(t *testing.T) {
	lookup := 0
	resolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		lookup++
		if lookup == 1 {
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("10.0.0.9")}, nil
	})
	dialAddresses := make([]string, 0, 1)
	dialer := dialerFunc(func(_ context.Context, _, address string) (net.Conn, error) {
		dialAddresses = append(dialAddresses, address)
		return nil, errors.New("stop after observing dial target")
	})
	client := NewClient(resolver, dialer)

	_, firstErr := client.Get("https://callback.example/hook")
	require.Error(t, firstErr)
	_, secondErr := client.Get("https://callback.example/hook")
	require.ErrorIs(t, secondErr, ErrUnsafeURL)
	require.Equal(t, 2, lookup, "each new connection must resolve DNS again")
	require.Equal(t, []string{"93.184.216.34:443"}, dialAddresses,
		"the socket must connect to the vetted address rather than resolving the hostname itself")
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	var targetRequests int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetRequests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	callback, roots := newCallbackTLSServer(t, "callback.example", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	})
	defer callback.Close()

	var dialAddresses []string
	client := NewClient(resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}), dialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		dialAddresses = append(dialAddresses, address)
		return (&net.Dialer{}).DialContext(ctx, network, callback.Listener.Addr().String())
	}))
	client.Transport.(requestPolicyTransport).base.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: roots}

	response, err := client.Get("https://callback.example/redirect")
	require.NoError(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, response.StatusCode)
	require.NoError(t, response.Body.Close())
	require.Zero(t, targetRequests, "redirect target must not receive an authenticated callback")
	require.Equal(t, []string{"93.184.216.34:443"}, dialAddresses,
		"a redirect must not trigger a second connection attempt")
}

func TestClientVerifiesTLSAgainstCallbackHostnameAfterIPPinning(t *testing.T) {
	server, roots := newCallbackTLSServer(t, "callback.example", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()
	resolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	})
	dialer := dialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		require.Equal(t, "93.184.216.34:443", address)
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	client := NewClient(resolver, dialer)
	client.Transport.(requestPolicyTransport).base.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: roots}

	response, err := client.Get("https://callback.example/hook")
	require.NoError(t, err, "a certificate valid for the original callback host must pass")
	require.Equal(t, http.StatusNoContent, response.StatusCode)
	require.NoError(t, response.Body.Close())

	response, err = client.Get("https://other.example/hook")
	require.Error(t, err, "a certificate for another hostname must not pass because the TCP dial was pinned")
	if response != nil {
		_ = response.Body.Close()
	}
}

func TestClientRejectsPlainHTTPOnPort443BeforeDNSOrDial(t *testing.T) {
	lookups := 0
	dials := 0
	client := NewClient(resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		lookups++
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}), dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		dials++
		return nil, errors.New("unexpected dial")
	}))

	_, err := client.Get("http://callback.example:443/hook")
	require.ErrorIs(t, err, ErrUnsafeURL)
	require.Zero(t, lookups, "non-HTTPS destinations must be rejected before DNS")
	require.Zero(t, dials, "non-HTTPS destinations must be rejected before socket creation")
}

func newCallbackTLSServer(t *testing.T, dnsName string, handler http.HandlerFunc) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: dnsName},
		DNSNames:              []string{dnsName},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	privateKey, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKey})
	serverCert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}}
	server.StartTLS()
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	return server, roots
}
