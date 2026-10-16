package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"
)

func TestRecordedPushHandlerIsAbsentWithoutOptIn(t *testing.T) {
	t.Setenv("NOTIFICATION_RECORD_PUSHES", "false")
	handler := notificationHTTPHandler(serviceName)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/debug/recorded-pushes?profile_id="+uuid.NewString(), nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected disabled debug route to return %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestRecordedPushHandlerReturnsProfileRecordWhenOptedIn(t *testing.T) {
	t.Setenv("NOTIFICATION_RECORD_PUSHES", " true ")
	profileID := uuid.New()
	fcm.GlobalPushRecorder.Record(profileID, store.DeviceToken{Token: "test-token"}, fcm.PushPayload{
		Title: "Test push",
		Body:  "Recorded test payload",
		Data:  map[string]string{"type": "test_event"},
	})

	handler := notificationHTTPHandler(serviceName)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/debug/recorded-pushes?profile_id="+profileID.String(), nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected opted-in debug route to return %d, got %d", http.StatusOK, recorder.Code)
	}
	var got fcm.RecordedPush
	if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
		t.Fatalf("decode recorded push: %v", err)
	}
	if got.ProfileID != profileID || got.Token != "test-token" || got.Type != "test_event" || got.Title != "Test push" || got.Body != "Recorded test payload" {
		t.Fatalf("unexpected recorded push response: %#v", got)
	}
}
