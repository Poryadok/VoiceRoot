package grpcsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/store"
)

type revocableDownloadFetchFunc func(context.Context, revocableDownloadCapability) (store.FileRow, []byte, error)

func (f revocableDownloadFetchFunc) FetchRevocableDownload(ctx context.Context, capability revocableDownloadCapability) (store.FileRow, []byte, error) {
	return f(ctx, capability)
}

func TestRevocableDownloadHTTPChecksCurrentEntitlementForEveryFetch(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	key := []byte("0123456789abcdef0123456789abcdef")
	profileID, fileID, messageID, chatID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	capability := revocableDownloadCapability{
		ProfileID: profileID, FileID: fileID, Variant: filev1.FileURLVariant_FILE_URL_VARIANT_UNSPECIFIED,
		Access: &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{
			Reference: &filev1.FileReferenceKey{FileId: fileID.String(),
				OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE,
				OwnerId:   messageID.String(), ScopeSpaceId: stringPtr(chatID.String())},
		}}, ExpiresAt: now.Add(time.Minute),
	}
	token, err := signRevocableDownloadCapability(capability, key)
	require.NoError(t, err)
	_, err = verifyRevocableDownloadCapability(token, key, now)
	require.NoError(t, err)

	allowed := false
	checks := 0
	handler := NewRevocableDownloadHTTPHandler(key, func() time.Time { return now }, revocableDownloadFetchFunc(
		func(_ context.Context, received revocableDownloadCapability) (store.FileRow, []byte, error) {
			checks++
			require.Equal(t, profileID, received.ProfileID)
			require.True(t, received.ExpiresAt.Equal(capability.ExpiresAt))
			if !allowed {
				return store.FileRow{}, nil, errDownloadEntitlementDenied
			}
			return store.FileRow{ID: fileID, Status: "ready", OriginalName: "proof.txt", MimeType: "text/plain"}, []byte("content"), nil
		}),
	)

	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/api/v1/files/download/"+token, nil))
	require.Equal(t, http.StatusForbidden, denied.Code, "revocation after URL issuance denies the later fetch")
	allowed = true
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, httptest.NewRequest(http.MethodGet, "/api/v1/files/download/"+token, nil))
	require.Equal(t, http.StatusOK, allowedResponse.Code)
	require.Equal(t, "content", allowedResponse.Body.String())
	require.Equal(t, 2, checks, "the same URL rechecks authority on every fetch")
	require.Equal(t, "no-store", allowedResponse.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", allowedResponse.Header().Get("X-Content-Type-Options"))
}

func TestRevocableDownloadHTTPRejectsInvalidMethodAndMalformedCapability(t *testing.T) {
	called := false
	handler := NewRevocableDownloadHTTPHandler([]byte(strings.Repeat("k", 32)), time.Now, revocableDownloadFetchFunc(
		func(context.Context, revocableDownloadCapability) (store.FileRow, []byte, error) {
			called = true
			return store.FileRow{}, nil, nil
		}),
	)

	wrongMethod := httptest.NewRecorder()
	handler.ServeHTTP(wrongMethod, httptest.NewRequest(http.MethodPost, "/api/v1/files/download/not-a-token", nil))
	require.Equal(t, http.StatusMethodNotAllowed, wrongMethod.Code)
	malformed := httptest.NewRecorder()
	handler.ServeHTTP(malformed, httptest.NewRequest(http.MethodGet, "/api/v1/files/download/not-a-token", nil))
	require.Equal(t, http.StatusNotFound, malformed.Code)
	require.False(t, called, "invalid bearer capabilities never reach the owner store")
}
