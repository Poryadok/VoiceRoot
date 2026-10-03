package grpcsvc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestT16AuthGISReadinessRecoversThroughActualChallengeRoute(t *testing.T) {
	application, environment, challenge := uuid.New(), uuid.New(), uuid.New()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/auth/sdk/challenges", r.URL.Path)
		require.Empty(t, r.Header.Get("Authorization"))
		var request map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, map[string]string{"applicationId": application.String(), "environmentId": environment.String(), "devicePublicJwk": "public-jwk"}, request)
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"challengeId": challenge.String()})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	id, err := probeT16AuthGISReadiness(ctx, server.Client(), server.URL, application, environment, "public-jwk", time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, challenge, id)
	require.EqualValues(t, 3, attempts.Load())
}

func TestT16AuthGISReadinessNeverAcceptsPersistentDenial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("sensitive-denial-body"))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	id, err := probeT16AuthGISReadiness(ctx, server.Client(), server.URL, uuid.New(), uuid.New(), "public-jwk", time.Millisecond)
	require.Error(t, err)
	require.Equal(t, uuid.Nil, id)
	require.NotContains(t, err.Error(), "sensitive-denial-body")
}

func TestT16AuthGISReadinessRejectsInvalidResponsesWithoutRetry(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"challengeId":"invalid-sensitive-response"}`))
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := probeT16AuthGISReadiness(ctx, server.Client(), server.URL, uuid.New(), uuid.New(), "public-jwk", time.Millisecond)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "invalid-sensitive-response")
			require.EqualValues(t, 1, attempts.Load())
		})
	}
}
