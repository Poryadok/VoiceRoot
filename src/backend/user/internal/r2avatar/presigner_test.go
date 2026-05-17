package r2avatar

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewS3R2PutPresigner_rejectsIncompleteConfig(t *testing.T) {
	full := S3R2Config{
		Endpoint:        "https://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.r2.cloudflarestorage.com",
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		Bucket:          "voice-avatars",
		PublicBaseURL:   "https://cdn.example.test",
	}
	for name, patch := range map[string]func(*S3R2Config){
		"empty_endpoint": func(c *S3R2Config) { c.Endpoint = "" },
		"empty_access_key": func(c *S3R2Config) { c.AccessKeyID = "" },
		"empty_secret": func(c *S3R2Config) { c.SecretAccessKey = "" },
		"empty_bucket": func(c *S3R2Config) { c.Bucket = "" },
		"empty_public_base": func(c *S3R2Config) { c.PublicBaseURL = "" },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := full
			patch(&cfg)
			_, err := NewS3R2PutPresigner(cfg)
			require.Error(t, err)
			require.Contains(t, err.Error(), "incomplete R2 configuration")
		})
	}
}

func TestNewS3R2PutPresigner_defaultRegionAuto(t *testing.T) {
	cfg := S3R2Config{
		Endpoint:        "https://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.r2.cloudflarestorage.com",
		Region:          "",
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		Bucket:          "voice-avatars",
		PublicBaseURL:   "https://cdn.example.test",
	}
	p, err := NewS3R2PutPresigner(cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
}

// TestS3R2PutPresigner_PresignPut_sigV4URLContract asserts the AWS SDK v2 presigned PUT shape
// (query SigV4 params, path-style bucket/key) without calling R2 — signing is local.
func TestS3R2PutPresigner_PresignPut_sigV4URLContract(t *testing.T) {
	ctx := context.Background()
	pid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	objectKey := ObjectKey(pid, ".png")

	p, err := NewS3R2PutPresigner(S3R2Config{
		Endpoint:        "https://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.r2.cloudflarestorage.com",
		Region:          "auto",
		AccessKeyID:     "testaccesskeyid",
		SecretAccessKey: "testsecretaccesskeyvalue",
		Bucket:          "voice-avatars",
		PublicBaseURL:   "https://pub-xxxx.r2.dev",
	})
	require.NoError(t, err)

	uploadURL, hdrs, exp, err := p.PresignPut(ctx, objectKey, "image/png", 2048)
	require.NoError(t, err)
	require.NotEmpty(t, uploadURL)

	parsed, err := url.Parse(uploadURL)
	require.NoError(t, err)
	require.Equal(t, "https", parsed.Scheme)
	require.Contains(t, parsed.Host, "r2.cloudflarestorage.com")

	q := parsed.Query()
	require.Equal(t, "AWS4-HMAC-SHA256", q.Get("X-Amz-Algorithm"))
	require.NotEmpty(t, q.Get("X-Amz-Credential"))
	require.NotEmpty(t, q.Get("X-Amz-Date"))
	require.NotEmpty(t, q.Get("X-Amz-Expires"))
	require.NotEmpty(t, q.Get("X-Amz-SignedHeaders"))
	require.NotEmpty(t, q.Get("X-Amz-Signature"))

	expiresParam := q.Get("X-Amz-Expires")
	require.Equal(t, "900", expiresParam, "expected 15m presign TTL (900s) bound in URL")

	path := parsed.Path
	require.True(t, strings.HasPrefix(path, "/"), "path-style URL must start with /")
	require.Contains(t, path, "/voice-avatars/")
	require.Contains(t, path, "/"+objectKey)

	require.Contains(t, hdrs, "Content-Type")
	require.Equal(t, "image/png", hdrs["Content-Type"])
	require.Contains(t, hdrs, "Content-Length")
	require.Equal(t, "2048", hdrs["Content-Length"])

	now := time.Now()
	require.True(t, exp.After(now.Add(14*time.Minute)), "expiresAt should be ~15m ahead")
	require.True(t, exp.Before(now.Add(16*time.Minute)))

	pub := p.PublicObjectURL(objectKey)
	require.Equal(t, "https://pub-xxxx.r2.dev/"+objectKey, pub)
}

func TestS3R2PutPresigner_PresignPut_rejectsEmptyObjectKey(t *testing.T) {
	ctx := context.Background()
	p, err := NewS3R2PutPresigner(S3R2Config{
		Endpoint:        "https://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.r2.cloudflarestorage.com",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
		Bucket:          "b",
		PublicBaseURL:   "https://pub.example",
	})
	require.NoError(t, err)
	_, _, _, err = p.PresignPut(ctx, "   ", "image/gif", 100)
	require.Error(t, err)
	require.Contains(t, err.Error(), "object key required")
}

func TestS3R2PutPresigner_PresignPut_enforcesUploadLimits(t *testing.T) {
	ctx := context.Background()
	p, err := NewS3R2PutPresigner(S3R2Config{
		Endpoint:        "https://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.r2.cloudflarestorage.com",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
		Bucket:          "b",
		PublicBaseURL:   "https://pub.example",
	})
	require.NoError(t, err)
	key := "avatars/" + uuid.New().String() + "/" + uuid.New().String() + ".jpg"

	_, _, _, err = p.PresignPut(ctx, key, "application/pdf", 100)
	require.Error(t, err)

	_, _, _, err = p.PresignPut(ctx, key, "image/jpeg", MaxAvatarBytes+1)
	require.Error(t, err)
}
