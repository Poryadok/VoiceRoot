package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComposeLiveObjectClientPreservesSignedMinIOHost(t *testing.T) {
	const signedQuery = "X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=abc123"
	type receivedRequest struct {
		method, host, path, query, body string
		err                            error
	}
	received := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		received <- receivedRequest{r.Method, r.Host, r.URL.Path, r.URL.RawQuery, string(body), err}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	t.Setenv("MINIO_PORT", port)
	client := composeLiveObjectClient(5 * time.Second)
	defer client.CloseIdleConnections()
	req, err := http.NewRequest(http.MethodPut, "http://minio:9000/voice-files/attachment.txt?"+signedQuery, strings.NewReader("signed bytes"))
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	got := <-received
	require.NoError(t, got.err)
	require.Equal(t, http.MethodPut, got.method)
	require.Equal(t, "minio:9000", got.host)
	require.Equal(t, "/voice-files/attachment.txt", got.path)
	require.Equal(t, signedQuery, got.query)
	require.Equal(t, "signed bytes", got.body)
}

func TestComposeLiveObjectDialAddressOnlyMapsComposeMinIO(t *testing.T) {
	t.Setenv("MINIO_PORT", "19000")
	require.Equal(t, "127.0.0.1:19000", composeLiveObjectDialAddress("minio:9000"))
	for _, address := range []string{"minio:9001", "minio.evil:9000", "example.com:9000", "127.0.0.1:9000"} {
		require.Equal(t, address, composeLiveObjectDialAddress(address))
	}
	// A non-numeric published port must not change the signed destination.
	t.Setenv("MINIO_PORT", "bad-port")
	require.Equal(t, "minio:9000", composeLiveObjectDialAddress("minio:9000"))
}
