package grpcsvc

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/pdfproc"
	"voice/backend/file/internal/store"
)

type boundedPDFThumbnailReader struct {
	max int64
	key string
}

func (p *boundedPDFThumbnailReader) ReadObject(_ context.Context, key string, max int64) ([]byte, error) {
	p.max = max
	p.key = key
	if max < 1024 {
		return nil, errors.New("thumbnail exceeds original size")
	}
	return bytes.Repeat([]byte("x"), 1024), nil
}

func TestPDFRevocableThumbnailUsesPNGAndDerivativeBound(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	id := insertR23ReadyFile(t, ctx, pool, "tiny-pdf")
	_, err := pool.Exec(ctx, `UPDATE files SET mime_type='application/pdf',file_type='document',size_bytes=100,thumbnail_r2_key='private/pdf.png' WHERE id=$1`, id)
	require.NoError(t, err)
	reference := &filev1.FileReferenceKey{FileId: id.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: uuid.NewString()}
	insertR23Reference(t, ctx, pool, reference)
	key := []byte("0123456789abcdef0123456789abcdef")
	reader := &boundedPDFThumbnailReader{}
	service := New(Deps{Files: store.NewFilesStore(pool), Presigner: gatePresigner{}, ChatGuard: &fixedMessageReferenceGuard{allowed: true}, Reader: reader, ReferenceAuthorityActive: true, RevocableDownloadKey: key})
	issued, err := service.GetFileURL(r23UserContext(ctx, uuid.New(), uuid.New()), &filev1.GetFileURLRequest{FileId: id.String(), Access: &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{Reference: reference}}, Variant: filev1.FileURLVariant_FILE_URL_VARIANT_THUMBNAIL})
	require.NoError(t, err)
	handler := NewRevocableDownloadHTTPHandler(key, nil, service)
	fetch := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, issued.PresignedGetUrl, nil))
		return w
	}
	response := fetch()
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "image/png", response.Header().Get("Content-Type"))
	require.Equal(t, int64(pdfproc.MaxThumbnailBytes), reader.max)
	require.Equal(t, "private/pdf.png", reader.key)
	row, err := service.files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "application/pdf", row.MimeType)
	require.EqualValues(t, 100, row.SizeBytes)
	_, err = pool.Exec(ctx, `UPDATE file_references SET released_at=now() WHERE file_id=$1`, id)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, fetch().Code)
}
