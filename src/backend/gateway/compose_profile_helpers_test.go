package main

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type composeTrackedReadCloser struct {
	reader *bytes.Reader
	closed bool
}

func (r *composeTrackedReadCloser) Read(p []byte) (int, error) {
	if r.closed {
		return 0, http.ErrBodyReadAfterClose
	}
	return r.reader.Read(p)
}

func (r *composeTrackedReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestComposeReadBodyPreservesResponseForDecode(t *testing.T) {
	originalBody := &composeTrackedReadCloser{
		reader: bytes.NewReader([]byte(`{"profile":{"id":"profile-123"}}`)),
	}
	resp := &http.Response{Body: originalBody}

	diagnostic := composeReadBody(t, resp)
	require.Equal(t, `{"profile":{"id":"profile-123"}}`, diagnostic)
	require.True(t, originalBody.closed, "the original response body must be closed")

	require.Equal(t, "profile-123", composeNestedJSONString(t, resp, "profile", "id"))
}
