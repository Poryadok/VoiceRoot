package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotificationHTTPHandlerDoesNotRegisterDebugRecorderByDefault(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/debug/recorded-pushes", nil)
	recorder := httptest.NewRecorder()

	notificationHTTPHandler(serviceName).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}
