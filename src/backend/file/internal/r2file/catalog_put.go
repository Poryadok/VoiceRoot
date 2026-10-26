package r2file

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"strings"
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
	sum := sha256.Sum256(body)
	e := p.PutTrackedObject(ctx, TrackedPut{Key: key, Kind: "catalog_source", ContentType: "image/gif", ExpectedSHA256: sum[:], ExpectedSize: int64(len(body))}, body)
	if e != nil {
		return errors.Join(ErrCatalogPut, ctx.Err())
	}
	return nil
}
