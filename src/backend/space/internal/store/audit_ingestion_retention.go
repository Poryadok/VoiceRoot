package store

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// cleanupAuditIngressEvidence removes only full replay fingerprints at the fixed
// aggregate PURGED+30d boundary. The permanent Space fence denies late ingress
// before receipt lookup; global audit IDs and trusted origin remain reserved.
// No row can be minimized before its database-verified terminal deadline.
func cleanupAuditIngressEvidence(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return ErrLifecycleEvidenceInvalid
	}
	_, err := tx.Exec(ctx, `WITH due AS (
 SELECT r.audit_event_id FROM audit_event_receipts r JOIN space_permanent_purge_fences p ON p.space_id=r.space_id
 WHERE r.origin IN ('role','chat') AND NOT r.fingerprint_minimized
 AND clock_timestamp()>=p.purged_at+interval '30 days'
 ORDER BY r.space_id,r.audit_event_id LIMIT 1000 FOR UPDATE OF r SKIP LOCKED)
 UPDATE audit_event_receipts r SET fingerprint_version=NULL,fingerprint=NULL,fingerprint_minimized=true
 FROM due WHERE r.audit_event_id=due.audit_event_id`)
	return err
}
