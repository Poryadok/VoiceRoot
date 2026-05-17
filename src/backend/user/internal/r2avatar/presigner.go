package r2avatar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// PutPresigner issues a time-limited S3-compatible PUT URL (Cloudflare R2).
type PutPresigner interface {
	PresignPut(ctx context.Context, objectKey, contentType string, contentLength int64) (uploadURL string, signedHeaders map[string]string, expiresAt time.Time, err error)
}

// S3R2Config holds Cloudflare R2 (S3 API) credentials and public URL base for avatars.
type S3R2Config struct {
	Endpoint        string // e.g. https://<accountid>.r2.cloudflarestorage.com
	Region          string // often "auto"
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PublicBaseURL   string // e.g. https://pub-xxxxx.r2.dev or custom CDN origin (no trailing slash)
}

// S3R2PutPresigner builds presigned PUT requests against R2.
type S3R2PutPresigner struct {
	bucket         string
	publicBaseURL  string
	presignClient  *s3.PresignClient
	presignExpires time.Duration
}

// NewS3R2PutPresigner validates config and constructs an R2 S3 presigning client.
func NewS3R2PutPresigner(cfg S3R2Config) (*S3R2PutPresigner, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	ak := strings.TrimSpace(cfg.AccessKeyID)
	sk := strings.TrimSpace(cfg.SecretAccessKey)
	bucket := strings.TrimSpace(cfg.Bucket)
	pub := strings.TrimSpace(cfg.PublicBaseURL)
	if endpoint == "" || ak == "" || sk == "" || bucket == "" || pub == "" {
		return nil, fmt.Errorf("r2avatar: incomplete R2 configuration (endpoint, access key, secret, bucket, public base URL required)")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "auto"
	}
	awsCfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(ak, sk, ""),
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		ep := strings.TrimRight(endpoint, "/")
		o.BaseEndpoint = aws.String(ep)
		o.UsePathStyle = true
	})
	return &S3R2PutPresigner{
		bucket:         bucket,
		publicBaseURL:  pub,
		presignClient:  s3.NewPresignClient(client),
		presignExpires: 15 * time.Minute,
	}, nil
}

// PresignPut creates a presigned PUT URL; content type and length are bound into the signature.
func (p *S3R2PutPresigner) PresignPut(ctx context.Context, objectKey, contentType string, contentLength int64) (string, map[string]string, time.Time, error) {
	if err := ValidateUploadParams(contentType, contentLength); err != nil {
		return "", nil, time.Time{}, err
	}
	key := strings.TrimSpace(objectKey)
	if key == "" {
		return "", nil, time.Time{}, fmt.Errorf("object key required")
	}
	ct := strings.TrimSpace(contentType)
	out, err := p.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(p.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(ct),
		ContentLength: aws.Int64(contentLength),
	}, s3.WithPresignExpires(p.presignExpires))
	if err != nil {
		return "", nil, time.Time{}, err
	}
	exp := time.Now().Add(p.presignExpires)
	headers := map[string]string{}
	for k, vals := range out.SignedHeader {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	return out.URL, headers, exp, nil
}

// PublicObjectURL returns the URL clients should persist as avatar_url after upload.
func (p *S3R2PutPresigner) PublicObjectURL(objectKey string) string {
	return JoinPublicURL(p.publicBaseURL, objectKey)
}
