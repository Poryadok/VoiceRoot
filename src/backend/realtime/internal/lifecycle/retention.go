package lifecycle

import "context"

// CompactReceipts drops full replay material after the original 30-day window.
// Binding hashes, completion times, permanent gates and unresolved admissions
// survive. In particular, GC never grants a new receipt or settles a ticket.
func (s *Store) CompactReceipts(ctx context.Context) (int64, error) {
	result, err := s.Pool.Exec(ctx, `WITH expired AS (
 SELECT space_id,operation_id,generation,kind FROM realtime_lifecycle_operations
 WHERE completed_at < clock_timestamp()-interval '30 days' AND response_bytes IS NOT NULL
 ORDER BY completed_at LIMIT 500 FOR UPDATE SKIP LOCKED
) UPDATE realtime_lifecycle_operations o SET request_bytes=NULL,response_bytes=NULL
 FROM expired e WHERE (o.space_id,o.operation_id,o.generation,o.kind)=(e.space_id,e.operation_id,e.generation,e.kind)`)
	return result.RowsAffected(), err
}
