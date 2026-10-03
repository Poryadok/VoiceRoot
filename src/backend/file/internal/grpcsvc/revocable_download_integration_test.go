package grpcsvc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/store"
)

func TestMessageFileURLUsesRevocableProxyAndRechecksLiveReference(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	fileID := insertR23ReadyFile(t, ctx, pool, "revocable-message")
	messageID, profileID := uuid.New(), uuid.New()
	reference := &filev1.FileReferenceKey{
		FileId: fileID.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE,
		OwnerId: messageID.String(),
	}
	insertR23Reference(t, ctx, pool, reference)
	guard := &fixedMessageReferenceGuard{allowed: true}
	key := []byte("0123456789abcdef0123456789abcdef")
	service := New(Deps{
		Files: store.NewFilesStore(pool), Presigner: gatePresigner{}, ChatGuard: guard,
		Reader: gateObjectReader{}, ReferenceAuthorityActive: true,
		RevocableDownloadKey: key,
	})
	selector := &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{Reference: reference}}
	issued, err := service.GetFileURL(r23UserContext(ctx, uuid.New(), profileID), &filev1.GetFileURLRequest{
		FileId: fileID.String(), Access: selector,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(issued.GetPresignedGetUrl(), "/api/v1/files/download/"), "message files must use File's revocable proxy")
	require.NotContains(t, issued.GetPresignedGetUrl(), "r2.example", "message selectors must never receive a direct object-store URL")

	handler := NewRevocableDownloadHTTPHandler(key, nil, service)
	fetch := func() *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, issued.GetPresignedGetUrl(), nil))
		return response
	}
	allowed := fetch()
	require.Equal(t, http.StatusOK, allowed.Code)
	require.Equal(t, string(gateUploadBytes), allowed.Body.String())

	_, err = pool.Exec(ctx, `UPDATE file_references SET released_at=clock_timestamp() WHERE file_id=$1 AND owner_type=$2 AND owner_id=$3`,
		fileID, int32(reference.GetOwnerType()), messageID)
	require.NoError(t, err)
	revoked := fetch()
	require.Equal(t, http.StatusNotFound, revoked.Code, "a previously issued bearer URL must fail closed after its exact reference is released")
}
