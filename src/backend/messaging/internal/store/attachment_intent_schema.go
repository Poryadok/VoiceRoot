package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAttachmentIntentSchema = errors.New("messaging lifecycle requires clean migration 000026 attachment send intents")

// RequireAttachmentIntentSchema runs before the enabled lifecycle runtime can
// import manifests, acquire references, recover intents or issue receipts.
// Return a fixed diagnostic; database errors can contain connection details.
func RequireAttachmentIntentSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return ErrAttachmentIntentSchema
	}
	var ready bool
	if err := pool.QueryRow(ctx, `SELECT count(*)=1 AND coalesce(bool_and(version>=26 AND NOT dirty),false) FROM public.schema_migrations`).Scan(&ready); err != nil || !ready {
		return ErrAttachmentIntentSchema
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)=9 FROM information_schema.columns
		WHERE table_schema='public' AND table_name='messaging_attachment_send_intents'
		AND (column_name,udt_name,is_nullable) IN (
		('operation_id','uuid','NO'),('message_id','uuid','NO'),('chat_id','uuid','NO'),
		('scope_space_id','uuid','YES'),('request_sha256','bytea','NO'),
		('references_bytes','bytea','NO'),('state','text','NO'),
		('expires_at','timestamptz','NO'),('completed_at','timestamptz','YES'))`).Scan(&ready); err != nil || !ready {
		return ErrAttachmentIntentSchema
	}
	return nil
}
