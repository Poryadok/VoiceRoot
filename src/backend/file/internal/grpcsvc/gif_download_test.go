package grpcsvc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
)

type gifEntitlementGuard struct{ allow *bool }

func (g gifEntitlementGuard) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (g gifEntitlementGuard) ChatE2EState(context.Context, uuid.UUID) (string, bool, error) {
	return "dm", false, nil
}
func (g gifEntitlementGuard) MessageReadEntitledForMessage(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return *g.allow, nil
}

type gifBoundedReader map[string][]byte

func (r gifBoundedReader) ReadObject(_ context.Context, key string, max int64) ([]byte, error) {
	b, ok := r[key]
	if !ok || int64(len(b)) > max {
		return nil, errors.New("missing or oversized object")
	}
	return b, nil
}

func TestGIFRevocableDownloadSelectsOutputBytesHeadersAndReadBound(t *testing.T) {
	ctx, files, id := gifQueuedFixture(t)
	for _, name := range []string{"000004_file_reference_lifecycle.up.sql", "000005_reference_gc_receipts.up.sql"} {
		applyFileGateSQL(t, ctx, files.Pool, filepath.Join("src", "backend", "migrations", "file_db", name))
	}
	job, err := files.ClaimGIFJob(ctx)
	require.NoError(t, err)
	fullBytes, thumbBytes := []byte(strings.Repeat("mp4", 500)), []byte("webp-thumbnail")
	full, thumb := gifRepresentations(job)
	full.SizeBytes = int64(len(fullBytes))
	full.SHA256 = shaHex(fullBytes)
	thumb.SizeBytes = int64(len(thumbBytes))
	thumb.SHA256 = shaHex(thumbBytes)
	ok, err := files.FinishGIFJob(ctx, job, full, thumb, 16, 16, 1, false)
	require.NoError(t, err)
	require.True(t, ok)
	message, viewer := uuid.New(), uuid.New()
	_, err = files.Pool.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id) VALUES($1,1,$2)`, id, message)
	require.NoError(t, err)
	allow := true
	service := New(Deps{Files: files, Reader: gifBoundedReader{full.Key: fullBytes, thumb.Key: thumbBytes}, ChatGuard: gifEntitlementGuard{&allow}})
	key := []byte(strings.Repeat("k", 32))
	handler := NewRevocableDownloadHTTPHandler(key, time.Now, service)
	access := &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{Reference: &filev1.FileReferenceKey{FileId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: message.String()}}}
	for _, variant := range []filev1.FileURLVariant{filev1.FileURLVariant_FILE_URL_VARIANT_UNSPECIFIED, filev1.FileURLVariant_FILE_URL_VARIANT_THUMBNAIL} {
		token, err := signRevocableDownloadCapability(revocableDownloadCapability{ProfileID: viewer, FileID: id, Access: access, Variant: variant, ExpiresAt: time.Now().Add(time.Minute)}, key)
		require.NoError(t, err)
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/files/download/"+token, nil)
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		if variant == 0 {
			require.Equal(t, "video/mp4", response.Header().Get("Content-Type"))
			require.Equal(t, fullBytes, response.Body.Bytes())
			require.Contains(t, response.Header().Get("Content-Disposition"), "animation.mp4")
		} else {
			require.Equal(t, "image/webp", response.Header().Get("Content-Type"))
			require.Equal(t, thumbBytes, response.Body.Bytes())
		}
		allow = false
		denied := httptest.NewRecorder()
		handler.ServeHTTP(denied, request)
		require.Equal(t, http.StatusForbidden, denied.Code)
		allow = true
	}
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "image/gif", row.MimeType)
	require.EqualValues(t, 42, row.SizeBytes)
}
