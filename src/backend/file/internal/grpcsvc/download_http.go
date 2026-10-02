package grpcsvc

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/file/internal/store"
)

var errDownloadEntitlementDenied = errors.New("download entitlement denied")

type revocableDownloadFetcher interface {
	FetchRevocableDownload(context.Context, revocableDownloadCapability) (store.FileRow, []byte, error)
}

type RevocableDownloadHTTPHandler struct {
	key     []byte
	now     func() time.Time
	fetcher revocableDownloadFetcher
}

func NewRevocableDownloadHTTPHandler(key []byte, now func() time.Time, fetcher revocableDownloadFetcher) *RevocableDownloadHTTPHandler {
	if now == nil {
		now = time.Now
	}
	return &RevocableDownloadHTTPHandler{key: append([]byte(nil), key...), now: now, fetcher: fetcher}
}

func (h *RevocableDownloadHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/files/download/"
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h == nil || h.fetcher == nil || len(h.key) < minimumDownloadCapabilityKeyBytes {
		http.Error(w, "download unavailable", http.StatusServiceUnavailable)
		return
	}
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, prefix)
	if token == "" || strings.Contains(token, "/") {
		http.NotFound(w, r)
		return
	}
	capability, err := verifyRevocableDownloadCapability(token, h.key, h.now())
	if err != nil {
		http.NotFound(w, r)
		return
	}
	row, body, err := h.fetcher.FetchRevocableDownload(r.Context(), capability)
	if err != nil {
		code := http.StatusServiceUnavailable
		switch {
		case errors.Is(err, errDownloadEntitlementDenied), status.Code(err) == codes.PermissionDenied:
			code = http.StatusForbidden
		case status.Code(err) == codes.NotFound, status.Code(err) == codes.FailedPrecondition:
			code = http.StatusNotFound
		}
		http.Error(w, "download unavailable", code)
		return
	}
	if row.ID == uuid.Nil || row.Status != "ready" {
		http.Error(w, "download unavailable", http.StatusNotFound)
		return
	}
	contentType := strings.TrimSpace(row.MimeType)
	if contentType == "" || strings.ContainsAny(contentType, "\r\n") {
		contentType = "application/octet-stream"
	}
	filename := path.Base(strings.ReplaceAll(row.OriginalName, "\\", "/"))
	if filename == "." || filename == "/" || strings.ContainsAny(filename, "\r\n") {
		filename = "download"
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
