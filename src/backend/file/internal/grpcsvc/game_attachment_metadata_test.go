package grpcsvc

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/file/internal/store"
)

func TestFileMetadataPublishesImmutableSourceObjectRevision(t *testing.T) {
	meta := fileRowToProto(store.FileRow{ID: uuid.New()})
	require.EqualValues(t, 1, meta.GetObjectRevision())
}
