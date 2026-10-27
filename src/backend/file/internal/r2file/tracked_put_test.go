package r2file

import (
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTrackedPutChecksDeclaredBytesAndNeverRetries(t *testing.T) {
	var calls atomic.Int32
	body := []byte("measured-thumbnail")
	digest := sha256.Sum256(body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		b, e := io.ReadAll(r.Body)
		require.NoError(t, e)
		require.Equal(t, body, b)
		require.Equal(t, "image/png", r.Header.Get("Content-Type"))
		w.WriteHeader(503)
	}))
	defer server.Close()
	p, err := NewS3R2Presigner(S3R2Config{Endpoint: server.URL, Region: "auto", AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "bucket"})
	require.NoError(t, err)
	intent := TrackedPut{Key: "pdf/" + uuid.NewString() + "/" + uuid.NewString() + "/first.png", Kind: "pdf_thumbnail", ContentType: "image/png", ExpectedSHA256: digest[:], ExpectedSize: int64(len(body))}
	wrong := intent
	wrong.ExpectedSize++
	require.Error(t, p.PutTrackedObject(context.Background(), wrong, body))
	require.Zero(t, calls.Load())
	wrong = intent
	wrong.ExpectedSHA256 = make([]byte, 32)
	require.Error(t, p.PutTrackedObject(context.Background(), wrong, body))
	require.Zero(t, calls.Load())
	require.Error(t, p.PutTrackedObject(context.Background(), intent, body))
	require.EqualValues(t, 1, calls.Load())
}
