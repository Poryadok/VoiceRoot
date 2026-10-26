package r2file

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

var ErrCatalogPut = errors.New("catalog source write unavailable")

// PutCatalogSource owns one synchronous storage attempt. No SDK retry or HTTP
// redirect can leave an earlier, hidden PUT behind an acknowledged attempt.
// Errors remain uncertain storage writes; callers must never infer quiescence.
func (p *S3R2Presigner) PutCatalogSource(ctx context.Context, key string, body []byte) error {
	parts := strings.Split(key, "/")
	if p == nil || p.client == nil || len(parts) != 4 || parts[0] != "catalog" || parts[3] != "source.gif" || len(body) < 6 || len(body) > 50<<20 || (!bytes.Equal(body[:6], []byte("GIF89a")) && !bytes.Equal(body[:6], []byte("GIF87a"))) {
		return ErrCatalogPut
	}
	for _, value := range parts[1:3] {
		id, e := uuid.Parse(value)
		if e != nil || id == uuid.Nil || id.String() != value {
			return ErrCatalogPut
		}
	}
	transport := awshttp.NewBuildableClient().GetTransport()
	transport.DisableKeepAlives = true
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	_, e := p.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(p.bucket), Key: aws.String(key), Body: bytes.NewReader(body), ContentType: aws.String("image/gif")}, func(options *s3.Options) { options.RetryMaxAttempts = 1; options.HTTPClient = client })
	if e != nil {
		return errors.Join(ErrCatalogPut, ctx.Err())
	}
	return nil
}
