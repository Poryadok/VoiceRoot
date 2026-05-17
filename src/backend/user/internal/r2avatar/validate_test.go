package r2avatar

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateUploadParams_contentTypeWhitelist(t *testing.T) {
	for _, ct := range []string{
		"image/jpeg",
		"image/png",
		"image/webp",
		"image/gif",
	} {
		t.Run(ct, func(t *testing.T) {
			err := ValidateUploadParams(ct, 1024)
			require.NoError(t, err)
		})
	}
}

func TestValidateUploadParams_rejectsNonImage(t *testing.T) {
	err := ValidateUploadParams("application/pdf", 100)
	require.Error(t, err)
	require.Contains(t, err.Error(), "content_type")
}

func TestValidateUploadParams_rejectsOversize(t *testing.T) {
	err := ValidateUploadParams("image/png", MaxAvatarBytes+1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "content_length")
}

func TestValidateUploadParams_rejectsZeroLength(t *testing.T) {
	err := ValidateUploadParams("image/png", 0)
	require.Error(t, err)
}

func TestFileExtForContentType(t *testing.T) {
	require.Equal(t, ".jpg", FileExtForContentType("image/jpeg"))
	require.Equal(t, ".png", FileExtForContentType("image/png"))
	require.Equal(t, ".webp", FileExtForContentType("image/webp"))
	require.Equal(t, ".gif", FileExtForContentType("image/gif"))
	require.Equal(t, "", FileExtForContentType("application/octet-stream"))
}
