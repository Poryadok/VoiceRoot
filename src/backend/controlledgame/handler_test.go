package controlledgame

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCallbackHandlerRejectsNoncanonicalRouteAndMethod(t *testing.T) {
	handler := NewHandler(HandlerConfig{
		Keys:  map[string][]byte{vectorKeyID: testKey()},
		Clock: func() time.Time { return time.Unix(1790500000, 0).UTC() },
	})
	for _, test := range []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "percent-encoded path", method: http.MethodPost, path: "/callbacks/game-commands/%76%31", status: http.StatusNotFound},
		{name: "query", method: http.MethodPost, path: vectorPath + "?x=1", status: http.StatusNotFound},
		{name: "wrong method", method: http.MethodGet, path: vectorPath, status: http.StatusMethodNotAllowed},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewReader([]byte(canonicalCommand)))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			require.Equal(t, test.status, recorder.Code)
		})
	}
}

func TestCallbackHandlerRejectsDuplicateAuthenticationAndContentTypeHeaders(t *testing.T) {
	key := testKey()
	handler := NewHandler(HandlerConfig{
		Keys:  map[string][]byte{vectorKeyID: key},
		Clock: func() time.Time { return time.Unix(1790500000, 0).UTC() },
	})
	for _, header := range []string{"X-Voice-Signature", "X-Voice-Key-Id", "X-Voice-Timestamp", "Content-Type"} {
		t.Run(header, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, vectorPath, bytes.NewReader([]byte(canonicalCommand)))
			request.Header.Set("Content-Type", "application/vnd.voice.game-command+json;version=1")
			request.Header.Set("X-Voice-Key-Id", vectorKeyID)
			request.Header.Set("X-Voice-Timestamp", vectorTimestamp)
			request.Header.Set("X-Voice-Signature", testSignature(key, http.MethodPost, vectorPath,
				vectorTimestamp, vectorKeyID, []byte(canonicalCommand)))
			request.Header.Add(header, request.Header.Get(header))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
