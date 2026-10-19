package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/moderation/internal/reportrecovery"
)

// ReportJournalSnapshot contains one finite byte observation, not a current-tail
// or readiness certificate. The offline caller supplies trusted capture IDs.
type ReportJournalSnapshot struct {
	Manifest []byte
	Chunks   map[string][]byte
	Cut      reportrecovery.Cut
}

// ExportReportRecoverySnapshot observes one supported current database. It does
// not fence writers: a FINAL capture additionally needs the trusted offline
// drain/continuity proof. Ordinary live boot does not need this export.
func (s *ReportStore) ExportReportRecoverySnapshot(ctx context.Context, incarnation, operation uuid.UUID) (*ReportJournalSnapshot, error) {
	if s == nil || s.Pool == nil || incarnation == uuid.Nil || operation == uuid.Nil {
		return nil, ErrReportPrivacy
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	view := reportrecovery.View{Protocol: 1, LogicalStore: "moderation", StoreIncarnation: incarnation, OperationID: operation, SnapshotID: uuid.New()}
	var snapshot string
	if err := tx.QueryRow(ctx, `SELECT journal_generation,pg_current_snapshot()::text FROM report_privacy_journal_metadata WHERE singleton`).Scan(&view.JournalGeneration, &snapshot); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(snapshot))
	view.ViewID = "pgsnapshot-sha256:" + hex.EncodeToString(sum[:])
	if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(max(sequence),0) FROM report_privacy_journal`).Scan(&view.RecordCount, &view.DiagnosticHighWater); err != nil {
		return nil, err
	}
	if view.RecordCount > reportrecovery.RecordsPerChunk*reportrecovery.MaxChunks {
		return nil, reportrecovery.ErrInvalid
	}
	view.ChunkCount = max(1, (view.RecordCount+reportrecovery.RecordsPerChunk-1)/reportrecovery.RecordsPerChunk)
	rows, err := tx.Query(ctx, `SELECT report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version
 FROM report_privacy_journal ORDER BY report_id,slot COLLATE "C"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]reportrecovery.Record, 0, min(view.RecordCount, reportrecovery.RecordsPerChunk))
	byteCount := 0
	for rows.Next() {
		var record reportrecovery.Record
		if err := rows.Scan(&record.ReportID, &record.Slot, &record.Disposition, &record.SubjectID, &record.EvidenceKind, &record.EvidenceID, &record.EvidenceVersion); err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		byteCount += len(encoded) + 1
		if byteCount > reportrecovery.MaxTotalBytes || len(records) >= view.RecordCount {
			return nil, reportrecovery.ErrInvalid
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	manifest, chunks, cut, err := reportrecovery.Encode(view, records)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &ReportJournalSnapshot{Manifest: manifest, Chunks: chunks, Cut: cut}, nil
}
