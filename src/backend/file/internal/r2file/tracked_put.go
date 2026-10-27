package r2file

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// TrackedPut is the immutable measured declaration returned by File's tracked
// write start. Media validity and owner publication authority are separate.
type TrackedPut struct {
	Key, Kind, ContentType string
	ExpectedSHA256         []byte
	ExpectedSize           int64
}

var ErrTrackedPut = errors.New("tracked object write unavailable")

func validTrackedPut(w TrackedPut, body []byte) bool {
	if w.Key == "" || w.Key != strings.TrimSpace(w.Key) || path.IsAbs(w.Key) || path.Clean(w.Key) != w.Key || strings.ContainsAny(w.Key, "\\\r\n\x00") || strings.HasPrefix(w.Key, "../") || w.Key == ".." || w.ExpectedSize <= 0 || int64(len(body)) != w.ExpectedSize || len(w.ExpectedSHA256) != 32 {
		return false
	}
	limit := int64(5 << 20)
	switch w.Kind {
	case "catalog_source":
		limit = 50 << 20
		if w.ContentType != "image/gif" {
			return false
		}
	case "pdf_thumbnail":
		if w.ContentType != "image/png" && w.ContentType != "image/webp" {
			return false
		}
	case "gif_full":
		if w.ContentType != "video/mp4" {
			return false
		}
	case "gif_thumbnail":
		if w.ContentType != "image/webp" {
			return false
		}
	default:
		return false
	}
	sum := sha256.Sum256(body)
	return w.ExpectedSize <= limit && bytes.Equal(sum[:], w.ExpectedSHA256)
}

// PutTrackedObject owns one synchronous storage attempt. No hidden SDK retry or
// HTTP redirect can leave an earlier attempt behind a successful ACK. Any error
// is unresolved physical I/O, not proof that submitted bytes cannot latecommit.
func (p *S3R2Presigner) PutTrackedObject(ctx context.Context, w TrackedPut, body []byte) error {
	if p == nil || p.client == nil || !validTrackedPut(w, body) {
		return ErrTrackedPut
	}
	transport := awshttp.NewBuildableClient().GetTransport()
	transport.DisableKeepAlives = true
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(p.bucket), Key: aws.String(w.Key), Body: bytes.NewReader(body), ContentType: aws.String(w.ContentType)}, func(options *s3.Options) { options.RetryMaxAttempts = 1; options.HTTPClient = client })
	if err != nil {
		return errors.Join(ErrTrackedPut, ctx.Err())
	}
	return nil
}
