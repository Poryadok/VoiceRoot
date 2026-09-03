package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/file/internal/store"

	filev1 "voice.app/voice/file/v1"
)

func TestLifecycleStatus_MapsExpiredSeparately(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   filev1.FileLifecycleStatus
	}{
		{
			name:   "expired",
			status: "expired",
			want:   filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_EXPIRED,
		},
		{
			name:   "deleted remains deleted",
			status: "deleted",
			want:   filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_DELETED,
		},
		{
			name:   "unknown remains unspecified",
			status: "unknown",
			want:   filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, lifecycleStatus(tt.status))
		})
	}
}

func TestFileRowToProto_ExpiredPreservesStringStatusAndReturnsExpiredEnum(t *testing.T) {
	meta := fileRowToProto(store.FileRow{
		ID:                uuid.MustParse("01020304-0506-0708-090a-0b0c0d0e0f10"),
		UploaderProfileID: uuid.MustParse("11111111-2222-3333-4444-555555555555"),
		OriginalName:      "retained-file.png",
		MimeType:          "image/png",
		SizeBytes:         1024,
		R2Key:             "attachments/retained-file.png",
		Status:            "expired",
		FileType:          "image",
		ScanResult:        "clean",
		CreatedAt:         time.Date(2026, time.September, 3, 0, 0, 0, 0, time.UTC),
	})

	require.Equal(t, "expired", meta.GetStatus())
	require.Equal(t, filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_EXPIRED, meta.GetStatusEnum())
}
