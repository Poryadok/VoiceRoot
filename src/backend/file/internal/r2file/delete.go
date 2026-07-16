package r2file

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ObjectDeleter removes objects from R2/S3-compatible storage.
type ObjectDeleter interface {
	DeleteObject(ctx context.Context, key string) error
}

// DeleteObject removes a single object from the configured bucket.
func (p *S3R2Presigner) DeleteObject(ctx context.Context, key string) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("r2file: presigner not configured")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("r2_key required")
	}
	_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	return err
}

// DeleteKeys removes unique non-empty object keys. Returns the first delete error.
func DeleteKeys(ctx context.Context, deleter ObjectDeleter, keys ...string) error {
	if deleter == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(keys))
	for _, raw := range keys {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := deleter.DeleteObject(ctx, key); err != nil {
			return err
		}
	}
	return nil
}
