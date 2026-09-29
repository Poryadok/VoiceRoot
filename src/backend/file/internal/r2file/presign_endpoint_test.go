package r2file

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBrowserPresignsUsePublicEndpointWhileObjectIOStaysInternal(t *testing.T) {
	var internalRequests atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		internalRequests.Add(1)
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/voice-files/attachments/example.txt", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "hello", string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer internal.Close()

	t.Setenv("FILE_R2_ENDPOINT", internal.URL)
	t.Setenv("FILE_R2_SIGNING_ENDPOINT", "https://storage.example.test")
	t.Setenv("FILE_R2_REGION", "us-east-1")
	t.Setenv("FILE_R2_ACCESS_KEY_ID", "test-access")
	t.Setenv("FILE_R2_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("FILE_R2_BUCKET", "voice-files")
	p, err := NewS3R2Presigner(EnvConfigFromOSEnv())
	require.NoError(t, err)

	put, err := p.PresignPut(context.Background(), PutPresignInput{
		Key: "attachments/example.txt", ContentType: "text/plain", ContentLength: 5,
	})
	require.NoError(t, err)
	get, err := p.PresignGet(context.Background(), GetPresignInput{Key: "attachments/example.txt"})
	require.NoError(t, err)
	getURL, err := url.Parse(get)
	require.NoError(t, err)
	require.Equal(t, "no-store", getURL.Query().Get("response-cache-control"), "Cloudflare must not cache an expired or revoked signed GET")
	for _, item := range []struct{ method, raw string }{{http.MethodPut, put}, {http.MethodGet, get}} {
		raw := item.raw
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.Equal(t, "https", u.Scheme)
		require.Equal(t, "storage.example.test", u.Host)
		require.Equal(t, "/voice-files/attachments/example.txt", u.Path)
		require.Contains(t, u.Query().Get("X-Amz-SignedHeaders"), "host")
		require.NotEmpty(t, u.Query().Get("X-Amz-Signature"))
		require.Equal(t, u.Query().Get("X-Amz-Signature"), testSigV4(u, item.method, "test-secret", map[string]string{"content-type": "text/plain", "content-length": "5"}), "signature must bind the public host and path")
	}
	require.Zero(t, internalRequests.Load(), "presigning must not call the storage endpoint")
	require.NoError(t, p.PutObject(context.Background(), "attachments/example.txt", "text/plain", []byte("hello")))
	require.EqualValues(t, 1, internalRequests.Load())
}

func testSigV4(u *url.URL, method, secret string, headers map[string]string) string {
	q := u.Query()
	q.Del("X-Amz-Signature")
	signed := q.Get("X-Amz-SignedHeaders")
	var canonicalHeaders strings.Builder
	for _, name := range strings.Split(signed, ";") {
		value := headers[name]
		if name == "host" {
			value = u.Host
		}
		canonicalHeaders.WriteString(name + ":" + strings.TrimSpace(value) + "\n")
	}
	canonicalRequest := method + "\n" + u.EscapedPath() + "\n" + q.Encode() + "\n" + canonicalHeaders.String() + "\n" + signed + "\nUNSIGNED-PAYLOAD"
	credential := strings.Split(q.Get("X-Amz-Credential"), "/")
	scope := strings.Join(credential[1:], "/")
	requestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" + q.Get("X-Amz-Date") + "\n" + scope + "\n" + hex.EncodeToString(requestHash[:])
	key := []byte("AWS4" + secret)
	for _, part := range credential[1:] {
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(part))
		key = mac.Sum(nil)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestFilePresignDefaultsToStorageEndpointWhenNoSigningEndpointConfigured(t *testing.T) {
	t.Setenv("FILE_R2_ENDPOINT", "http://minio:9000")
	t.Setenv("FILE_R2_SIGNING_ENDPOINT", "")
	t.Setenv("FILE_R2_REGION", "us-east-1")
	t.Setenv("FILE_R2_ACCESS_KEY_ID", "test-access")
	t.Setenv("FILE_R2_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("FILE_R2_BUCKET", "voice-files")
	p, err := NewS3R2Presigner(EnvConfigFromOSEnv())
	require.NoError(t, err)
	raw, err := p.PresignPut(context.Background(), PutPresignInput{Key: "attachments/example.txt", ContentType: "text/plain", ContentLength: 5})
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "http", u.Scheme)
	require.Equal(t, "minio:9000", u.Host)
	require.Equal(t, u.Query().Get("X-Amz-Signature"), testSigV4(u, http.MethodPut, "test-secret", map[string]string{"content-type": "text/plain", "content-length": "5"}))
}

func TestFileRejectsNonHTTPSOrPathfulSigningEndpoint(t *testing.T) {
	for _, endpoint := range []string{"http://storage.example.test", "https://storage.example.test/prefix", "https://storage.example.test?token=oops"} {
		t.Run(endpoint, func(t *testing.T) {
			_, err := NewS3R2Presigner(S3R2Config{Endpoint: "http://minio:9000", SigningEndpoint: endpoint, Region: "us-east-1", AccessKeyID: "ak", SecretAccessKey: "sk", Bucket: "files"})
			require.ErrorContains(t, err, "signing endpoint")
		})
	}
}
