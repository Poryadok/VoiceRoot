package r2file

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStorageDeleteHasOneAttemptAndNoRedirect(t *testing.T) {
	for _, response := range []int{http.StatusInternalServerError, http.StatusTemporaryRedirect} {
		t.Run(http.StatusText(response), func(t *testing.T) {
			var calls, redirects atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirects.Add(1); w.WriteHeader(http.StatusNoContent) }))
			defer target.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", target.URL+"/bucket/key")
				w.WriteHeader(response)
			}))
			defer origin.Close()
			client, err := NewS3R2Presigner(S3R2Config{Endpoint: origin.URL, AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "fixture"})
			require.NoError(t, err)
			require.Error(t, client.DeleteObject(context.Background(), "exact/key"))
			require.EqualValues(t, 1, calls.Load())
			require.Zero(t, redirects.Load())
		})
	}
}
