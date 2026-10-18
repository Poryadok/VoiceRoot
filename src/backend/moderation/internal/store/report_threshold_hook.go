package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

var _ ReportValidatedHook = (*ReportThresholdStore)(nil)

// ReportValidated only enqueues durable work in the activation transaction.
// Subject/audience authority is obtained later, outside this transaction.
func (s *ReportThresholdStore) ReportValidated(ctx context.Context, tx pgx.Tx, fact ValidationFact) error {
	if fact.TargetType != "user" || fact.ReporterAccountID == nil || fact.TargetAccountID == nil || *fact.ReporterAccountID == *fact.TargetAccountID {
		return nil
	}
	return s.EnqueueTx(ctx, tx, fact.ReportID)
}
