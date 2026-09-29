package r2avatar

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
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
		"empty_endpoint":    func(c *S3R2Config) { c.Endpoint = "" },
		"empty_access_key":  func(c *S3R2Config) { c.AccessKeyID = "" },
		"empty_secret":      func(c *S3R2Config) { c.SecretAccessKey = "" },
		"empty_bucket":      func(c *S3R2Config) { c.Bucket = "" },
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
	_, _, _, err = p.PresignPut(ctx, "   ", "image/png", 100)
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

func TestAvatarPresignUsesBrowserEndpointAndKeepsPublicObjectURL(t *testing.T) {
	t.Setenv("USER_R2_ENDPOINT", "http://voice-minio:9000")
	t.Setenv("USER_R2_SIGNING_ENDPOINT", "https://storage.example.test")
	t.Setenv("USER_R2_REGION", "us-east-1")
	t.Setenv("USER_R2_ACCESS_KEY_ID", "test-access")
	t.Setenv("USER_R2_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("USER_R2_BUCKET", "voice-avatars")
	t.Setenv("USER_R2_PUBLIC_BASE_URL", "https://cdn.example.test/avatars")
	p, err := NewS3R2PutPresigner(EnvConfigFromOSEnv())
	require.NoError(t, err)

	key := "avatars/11111111-1111-1111-1111-111111111111/photo.png"
	raw, _, _, err := p.PresignPut(context.Background(), key, "image/png", 2048)
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "https", u.Scheme)
	require.Equal(t, "storage.example.test", u.Host)
	require.Equal(t, "/voice-avatars/"+key, u.Path)
	require.Contains(t, u.Query().Get("X-Amz-SignedHeaders"), "host")
	require.NotEmpty(t, u.Query().Get("X-Amz-Signature"))
	require.Equal(t, u.Query().Get("X-Amz-Signature"), testAvatarSigV4(u, "test-secret", "image/png", "2048"), "signature must bind the public host and path")
	require.Equal(t, "https://cdn.example.test/avatars/"+key, p.PublicObjectURL(key))
}

func TestAvatarPresignDefaultsToStorageEndpointWhenNoSigningEndpointConfigured(t *testing.T) {
	t.Setenv("USER_R2_ENDPOINT", "http://minio:9000")
	t.Setenv("USER_R2_SIGNING_ENDPOINT", "")
	t.Setenv("USER_R2_REGION", "us-east-1")
	t.Setenv("USER_R2_ACCESS_KEY_ID", "test-access")
	t.Setenv("USER_R2_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("USER_R2_BUCKET", "voice-avatars")
	t.Setenv("USER_R2_PUBLIC_BASE_URL", "http://localhost:9000/voice-avatars")
	p, err := NewS3R2PutPresigner(EnvConfigFromOSEnv())
	require.NoError(t, err)
	raw, _, _, err := p.PresignPut(context.Background(), "avatars/photo.png", "image/png", 2048)
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "http", u.Scheme)
	require.Equal(t, "minio:9000", u.Host)
	require.Equal(t, u.Query().Get("X-Amz-Signature"), testAvatarSigV4(u, "test-secret", "image/png", "2048"))
}

func testAvatarSigV4(u *url.URL, secret, contentType, contentLength string) string {
	q := u.Query()
	q.Del("X-Amz-Signature")
	signed := q.Get("X-Amz-SignedHeaders")
	var canonicalHeaders strings.Builder
	for _, name := range strings.Split(signed, ";") {
		value := map[string]string{"host": u.Host, "content-type": contentType, "content-length": contentLength}[name]
		canonicalHeaders.WriteString(name + ":" + strings.TrimSpace(value) + "\n")
	}
	canonicalRequest := http.MethodPut + "\n" + u.EscapedPath() + "\n" + q.Encode() + "\n" + canonicalHeaders.String() + "\n" + signed + "\nUNSIGNED-PAYLOAD"
	credential := strings.Split(q.Get("X-Amz-Credential"), "/")
	requestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" + q.Get("X-Amz-Date") + "\n" + strings.Join(credential[1:], "/") + "\n" + hex.EncodeToString(requestHash[:])
	key := []byte("AWS4" + secret)
	for _, part := range credential[1:] {
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(part))
		key = mac.Sum(nil)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestAvatarRejectsNonHTTPSOrPathfulSigningEndpoint(t *testing.T) {
	for _, endpoint := range []string{"http://storage.example.test", "https://storage.example.test/prefix", "https://storage.example.test?token=oops"} {
		t.Run(endpoint, func(t *testing.T) {
			_, err := NewS3R2PutPresigner(S3R2Config{Endpoint: "http://minio:9000", SigningEndpoint: endpoint, Region: "us-east-1", AccessKeyID: "ak", SecretAccessKey: "sk", Bucket: "avatars", PublicBaseURL: "https://cdn.example.test"})
			require.ErrorContains(t, err, "signing endpoint")
		})
	}
}
