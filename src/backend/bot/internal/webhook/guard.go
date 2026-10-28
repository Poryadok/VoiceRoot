package webhook

import (
	"context"
	"errors"
	"net/http"
)

type guardedTransport struct {
	delegate http.RoundTripper
	check    func(context.Context) error
}

func (g guardedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if g.check == nil {
		return nil, errors.New("webhook authority guard required")
	}
	if err := g.check(request.Context()); err != nil {
		return nil, err
	}
	return g.delegate.RoundTrip(request)
}

// GuardedClient checks the still-live authority before every actual transport
// attempt, including delivery retries and redirects. It preserves client policy.
func GuardedClient(client *http.Client, check func(context.Context) error) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{Transport: guardedTransport{delegate: transport, check: check}, CheckRedirect: client.CheckRedirect, Jar: client.Jar, Timeout: client.Timeout}
}
