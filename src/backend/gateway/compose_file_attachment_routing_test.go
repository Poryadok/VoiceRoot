package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComposeRestartProofDownloadClientRoutesSignedHostPUT(t *testing.T) {
	t.Parallel()

	const (
		rawPath  = "/voice-attachments/a%2Fb%20upload.txt"
		rawQuery = "X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=test%2Fcredential&X-Amz-Signature=deadbeef"
		body     = "attachment upload bytes\n"
	)
	type observedRequest struct {
		method      string
		host        string
		rawPath     string
		rawQuery    string
		contentType string
		body        []byte
		err         error
	}
	observed := make(chan observedRequest, 1)
	var port int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, err := io.ReadAll(r.Body)
		observed <- observedRequest{
			method:      r.Method,
			host:        r.Host,
			rawPath:     r.URL.EscapedPath(),
			rawQuery:    r.URL.RawQuery,
			contentType: r.Header.Get("Content-Type"),
			body:        received,
			err:         err,
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	port = server.Listener.Addr().(*net.TCPAddr).Port
	target := (&url.URL{
		Scheme:   "http",
		Host:     net.JoinHostPort("host.docker.internal", strconv.Itoa(port)),
		Path:     "/voice-attachments/a/b upload.txt",
		RawPath:  rawPath,
		RawQuery: rawQuery,
	}).String()

	client := composeRestartProofDownloadClient()
	t.Cleanup(client.CloseIdleConnections)
	req, err := http.NewRequest(http.MethodPut, target, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	request := <-observed
	require.NoError(t, request.err)
	require.Equal(t, http.MethodPut, request.method)
	require.Equal(t, "host.docker.internal:"+strconv.Itoa(port), request.host)
	require.Equal(t, rawPath, request.rawPath)
	require.Equal(t, rawQuery, request.rawQuery)
	require.Equal(t, "text/plain", request.contentType)
	require.Equal(t, body, string(request.body))
}
