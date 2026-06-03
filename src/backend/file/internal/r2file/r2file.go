package r2file

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const (
	MaxFreeFileBytes = 50 * 1024 * 1024
	DefaultURLTTL    = time.Hour
)

type PutPresignInput struct {
	Key           string
	ContentType   string
	ContentLength int64
	TTL           time.Duration
}

type GetPresignInput struct {
	Key string
	TTL time.Duration
}

type Presigner interface {
	PresignPut(context.Context, PutPresignInput) (string, error)
	PresignGet(context.Context, GetPresignInput) (string, error)
}

type ObjectReader interface {
	ReadObject(ctx context.Context, key string, maxBytes int64) ([]byte, error)
}

type S3R2Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

type S3R2Presigner struct {
	bucket        string
	client        *s3.Client
	presignClient *s3.PresignClient
}

func EnvConfigFromOSEnv() S3R2Config {
	return S3R2Config{
		Endpoint:        strings.TrimSpace(os.Getenv("FILE_R2_ENDPOINT")),
		Region:          strings.TrimSpace(os.Getenv("FILE_R2_REGION")),
		AccessKeyID:     strings.TrimSpace(os.Getenv("FILE_R2_ACCESS_KEY_ID")),
		SecretAccessKey: strings.TrimSpace(os.Getenv("FILE_R2_SECRET_ACCESS_KEY")),
		Bucket:          strings.TrimSpace(os.Getenv("FILE_R2_BUCKET")),
	}
}

func NewS3R2Presigner(cfg S3R2Config) (*S3R2Presigner, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	ak := strings.TrimSpace(cfg.AccessKeyID)
	sk := strings.TrimSpace(cfg.SecretAccessKey)
	bucket := strings.TrimSpace(cfg.Bucket)
	if endpoint == "" || ak == "" || sk == "" || bucket == "" {
		return nil, fmt.Errorf("r2file: incomplete R2 configuration (endpoint, access key, secret, bucket required)")
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
		o.BaseEndpoint = aws.String(strings.TrimRight(endpoint, "/"))
		o.UsePathStyle = true
	})
	return &S3R2Presigner{
		bucket:        bucket,
		client:        client,
		presignClient: s3.NewPresignClient(client),
	}, nil
}

func (p *S3R2Presigner) PresignPut(ctx context.Context, in PutPresignInput) (string, error) {
	if err := ValidateUpload(in.OriginalNameForValidation(), in.ContentType, in.ContentLength); err != nil {
		return "", err
	}
	key := strings.TrimSpace(in.Key)
	if key == "" {
		return "", fmt.Errorf("r2_key required")
	}
	ttl := in.TTL
	if ttl <= 0 {
		ttl = DefaultURLTTL
	}
	out, err := p.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(p.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(strings.TrimSpace(strings.ToLower(in.ContentType))),
		ContentLength: aws.Int64(in.ContentLength),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (p *S3R2Presigner) ReadObject(ctx context.Context, key string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = MaxFreeFileBytes
	}
	out, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(strings.TrimSpace(key)),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = out.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(out.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("object exceeds max of %d bytes", maxBytes)
	}
	return data, nil
}

func (p *S3R2Presigner) PresignGet(ctx context.Context, in GetPresignInput) (string, error) {
	key := strings.TrimSpace(in.Key)
	if key == "" {
		return "", fmt.Errorf("r2_key required")
	}
	ttl := in.TTL
	if ttl <= 0 {
		ttl = DefaultURLTTL
	}
	out, err := p.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func ValidateUpload(originalName, contentType string, sizeBytes int64) error {
	if strings.TrimSpace(originalName) == "" {
		return fmt.Errorf("original_name is required")
	}
	if safeOriginalName(originalName) == "" {
		return fmt.Errorf("original_name is invalid")
	}
	ct := strings.TrimSpace(strings.ToLower(contentType))
	if ct == "" {
		return fmt.Errorf("mime_type is required")
	}
	if _, _, err := mime.ParseMediaType(ct); err != nil {
		return fmt.Errorf("mime_type is invalid")
	}
	if sizeBytes <= 0 {
		return fmt.Errorf("size_bytes must be positive")
	}
	if sizeBytes > MaxFreeFileBytes {
		return fmt.Errorf("size_bytes exceeds max of %d bytes", MaxFreeFileBytes)
	}
	return nil
}

func ObjectKey(fileID uuid.UUID, originalName string) string {
	leaf := safeOriginalName(originalName)
	if leaf == "" {
		leaf = "file"
	}
	return fmt.Sprintf("attachments/%s/%s", fileID.String(), leaf)
}

func MediaCategory(mimeType string) string {
	ct := strings.TrimSpace(strings.ToLower(mimeType))
	switch {
	case strings.HasPrefix(ct, "image/"):
		return "image"
	case strings.HasPrefix(ct, "video/"):
		return "video"
	case strings.HasPrefix(ct, "audio/"):
		return "audio"
	case strings.HasPrefix(ct, "text/"), ct == "application/pdf":
		return "document"
	default:
		return "other"
	}
}

func safeOriginalName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.Trim(base, ". ")
	if base == "" || base == "." || base == string(filepath.Separator) {
		return ""
	}
	var b strings.Builder
	for _, r := range base {
		switch {
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "._ ")
}

func (in PutPresignInput) OriginalNameForValidation() string {
	key := strings.TrimSpace(in.Key)
	if key == "" {
		return "file"
	}
	return filepath.Base(key)
}
