package r2avatar

import (
	"testing"

	"github.com/google/uuid"
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
	require.Contains(t, err.Error(), "content_length")
}

func TestValidateUploadParams_rejectsNegativeLength(t *testing.T) {
	err := ValidateUploadParams("image/png", -1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "content_length")
}

func TestValidateUploadParams_acceptsExactMax(t *testing.T) {
	err := ValidateUploadParams("image/webp", MaxAvatarBytes)
	require.NoError(t, err)
}

func TestValidateUploadParams_trimsAndLowercasesMIME(t *testing.T) {
	err := ValidateUploadParams("  Image/JPEG  ", 1)
	require.NoError(t, err)
}

func TestValidateUploadParams_rejectsMIMEWithCharset(t *testing.T) {
	err := ValidateUploadParams("image/png; charset=utf-8", 100)
	require.Error(t, err)
	require.Contains(t, err.Error(), "content_type")
}

func TestFileExtForContentType(t *testing.T) {
	require.Equal(t, ".jpg", FileExtForContentType("image/jpeg"))
	require.Equal(t, ".png", FileExtForContentType("image/png"))
	require.Equal(t, ".webp", FileExtForContentType("image/webp"))
	require.Equal(t, ".gif", FileExtForContentType("image/gif"))
	require.Equal(t, "", FileExtForContentType("application/octet-stream"))
}

func TestValidateAvatarObjectKeyForProfile_ok(t *testing.T) {
	pid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	file := "33333333-3333-3333-3333-333333333333.png"
	key := "avatars/" + pid.String() + "/" + file
	require.NoError(t, ValidateAvatarObjectKeyForProfile(pid, key))
	require.NoError(t, ValidateAvatarObjectKeyForProfile(pid, "/"+key))
}

func TestValidateAvatarObjectKeyForProfile_wrongProfile(t *testing.T) {
	pid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	other := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	key := "avatars/" + other.String() + "/33333333-3333-3333-3333-333333333333.png"
	require.Error(t, ValidateAvatarObjectKeyForProfile(pid, key))
}

func TestValidateAvatarObjectKeyForProfile_badShape(t *testing.T) {
	pid := uuid.New()
	require.Error(t, ValidateAvatarObjectKeyForProfile(pid, "not-a-key"))
	require.Error(t, ValidateAvatarObjectKeyForProfile(pid, "avatars/"+pid.String()+"/bad.png"))
}

func TestValidateAvatarPublicURLForProfile_ok(t *testing.T) {
	base := "https://cdn-test.example"
	pid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	u := JoinPublicURL(base, "avatars/"+pid.String()+"/33333333-3333-3333-3333-333333333333.png")
	require.NoError(t, ValidateAvatarPublicURLForProfile(base, pid, u))
}

func TestValidateAvatarPublicURLForProfile_wrongProfileFolder(t *testing.T) {
	base := "https://cdn-test.example"
	pid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	other := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	u := JoinPublicURL(base, "avatars/"+other.String()+"/33333333-3333-3333-3333-333333333333.png")
	err := ValidateAvatarPublicURLForProfile(base, pid, u)
	require.Error(t, err)
}
