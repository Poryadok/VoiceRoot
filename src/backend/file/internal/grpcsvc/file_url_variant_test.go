package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/file/internal/store"

	filev1 "voice.app/voice/file/v1"
)

func TestFileURLKey_SelectsRequestedVariant(t *testing.T) {
	converted := "processed/file/full.webp"
	thumbnail := "processed/file/thumb.webp"
	blank := "   "
	row := store.FileRow{
		R2Key:          "attachments/file/original.png",
		ConvertedR2Key: &converted,
		ThumbnailR2Key: &thumbnail,
	}

	tests := []struct {
		name    string
		variant filev1.FileURLVariant
		row     store.FileRow
		wantKey string
		wantErr codes.Code
	}{
		{
			name:    "unspecified preserves converted legacy URL",
			variant: filev1.FileURLVariant_FILE_URL_VARIANT_UNSPECIFIED,
			row:     row,
			wantKey: converted,
		},
		{
			name:    "unspecified falls back to original when converted is absent",
			variant: filev1.FileURLVariant_FILE_URL_VARIANT_UNSPECIFIED,
			row:     store.FileRow{R2Key: "attachments/file/original.png"},
			wantKey: "attachments/file/original.png",
		},
		{
			name:    "unspecified falls back to original when converted is blank",
			variant: filev1.FileURLVariant_FILE_URL_VARIANT_UNSPECIFIED,
			row:     store.FileRow{R2Key: "attachments/file/original.png", ConvertedR2Key: &blank},
			wantKey: "attachments/file/original.png",
		},
		{
			name:    "thumbnail uses thumbnail key only",
			variant: filev1.FileURLVariant_FILE_URL_VARIANT_THUMBNAIL,
			row:     row,
			wantKey: thumbnail,
		},
		{
			name:    "missing thumbnail has no legacy fallback",
			variant: filev1.FileURLVariant_FILE_URL_VARIANT_THUMBNAIL,
			row:     store.FileRow{R2Key: "attachments/file/original.png", ConvertedR2Key: &converted},
			wantErr: codes.FailedPrecondition,
		},
		{
			name:    "unknown variant is invalid",
			variant: filev1.FileURLVariant(99),
			row:     row,
			wantErr: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := fileURLKey(tt.row, tt.variant)
			if tt.wantErr != codes.OK {
				require.Equal(t, tt.wantErr, status.Code(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantKey, key)
		})
	}
}

func TestGetFileURL_RejectsInvalidVariantBeforeAccessChecks(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	svc := New(Deps{Files: store.NewFilesStore(pool)})

	_, err := svc.GetFileURL(fileGateCtx(ctx, uuid.New(), uuid.New()), &filev1.GetFileURLRequest{
		FileId:  uuid.NewString(),
		Variant: filev1.FileURLVariant(99),
	})

	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
