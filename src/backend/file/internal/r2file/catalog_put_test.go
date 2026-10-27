package r2file

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCatalogSourcePutHasOneAttemptAndNoRedirect(t *testing.T) {
	for _, mode := range []string{"success", "unavailable", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			var calls, targetCalls atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1); w.WriteHeader(200) }))
			defer target.Close()
			key := "catalog/" + uuid.NewString() + "/" + uuid.NewString() + "/source.gif"
			body := []byte("GIF89a-source")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, http.MethodPut, r.Method)
				require.Equal(t, "/bucket/"+key, r.URL.Path)
				require.Equal(t, "image/gif", r.Header.Get("Content-Type"))
				data, e := io.ReadAll(r.Body)
				require.NoError(t, e)
				require.Equal(t, body, data)
				switch mode {
				case "success":
					w.Header().Set("ETag", "fixture")
					w.WriteHeader(200)
				case "unavailable":
					w.WriteHeader(503)
				case "redirect":
					w.Header().Set("Location", target.URL+"/other")
					w.WriteHeader(307)
				}
			}))
			defer server.Close()
			writer, e := NewS3R2Presigner(S3R2Config{Endpoint: server.URL, Region: "auto", AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "bucket"})
			require.NoError(t, e)
			e = writer.PutCatalogSource(context.Background(), key, body)
			if mode == "success" {
				require.NoError(t, e)
			} else {
				require.Error(t, e)
			}
			require.EqualValues(t, 1, calls.Load())
			require.Zero(t, targetCalls.Load())
		})
	}
}
