// Package reportrecovery verifies bounded opaque report journal artifacts.
// A valid snapshot proves byte identity, never source currency or boot readiness.
package reportrecovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/google/uuid"
)

const (
	RecordsPerChunk  = 8192
	MaxChunks        = 128
	MaxChunkBytes    = 8 << 20
	MaxTotalBytes    = 64 << 20
	MaxManifestBytes = 64 << 10
	MaxLineBytes     = 1024
)

var ErrInvalid = errors.New("invalid report recovery snapshot")

type Record struct {
	ReportID        uuid.UUID  `json:"report_id"`
	Slot            string     `json:"slot"`
	Disposition     string     `json:"disposition"`
	SubjectID       *uuid.UUID `json:"subject_id,omitempty"`
	EvidenceKind    string     `json:"evidence_kind"`
	EvidenceID      uuid.UUID  `json:"evidence_id"`
	EvidenceVersion int64      `json:"evidence_version"`
}

// View identifies one finite observation. DiagnosticHighWater is not a
// completeness proof, and OperationID must come from the trusted capture caller.
type View struct {
	Protocol            int       `json:"protocol"`
	LogicalStore        string    `json:"logical_store"`
	StoreIncarnation    uuid.UUID `json:"store_incarnation"`
	JournalGeneration   uuid.UUID `json:"journal_generation"`
	SnapshotID          uuid.UUID `json:"snapshot_id"`
	OperationID         uuid.UUID `json:"operation_id"`
	ViewID              string    `json:"view_id"`
	DiagnosticHighWater int64     `json:"diagnostic_high_water"`
	RecordCount         int       `json:"record_count"`
	ChunkCount          int       `json:"chunk_count"`
}

type ChunkIdentity struct {
	Kind        string `json:"kind"`
	Schema      int    `json:"schema"`
	Ordinal     int    `json:"ordinal"`
	RecordCount int    `json:"record_count"`
	ByteLength  int    `json:"byte_length"`
	SHA256      string `json:"sha256"`
}

type Manifest struct {
	View      View            `json:"view"`
	Artifacts []ChunkIdentity `json:"artifacts"`
}

// Cut must be compared against independently selected expected capture inputs.
// It deliberately has no FINAL, verified-current, or ready field.
type Cut struct {
	View           View            `json:"view"`
	ManifestSHA256 string          `json:"manifest_sha256"`
	Artifacts      []ChunkIdentity `json:"artifacts"`
}

type header struct {
	Type        string `json:"type"`
	View        View   `json:"view"`
	Ordinal     int    `json:"ordinal"`
	RecordCount int    `json:"record_count"`
}

type Snapshot struct {
	cut     Cut
	records []Record
}

func Encode(view View, input []Record) ([]byte, map[string][]byte, Cut, error) {
	if !validView(view) || len(input) != view.RecordCount {
		return nil, nil, Cut{}, ErrInvalid
	}
	records := cloneRecords(input)
	slices.SortFunc(records, compareRecords)
	for i, record := range records {
		if !validRecord(record) || (i > 0 && compareRecords(records[i-1], record) >= 0) {
			return nil, nil, Cut{}, ErrInvalid
		}
	}
	manifest := Manifest{View: view, Artifacts: make([]ChunkIdentity, 0, view.ChunkCount)}
	chunks := make(map[string][]byte, view.ChunkCount)
	total := 0
	for ordinal := 1; ordinal <= view.ChunkCount; ordinal++ {
		count := chunkRecords(view, ordinal)
		b, err := canonicalLine(header{Type: "voice_report_journal_v1", View: view, Ordinal: ordinal, RecordCount: count})
		if err != nil {
			return nil, nil, Cut{}, err
		}
		start := (ordinal - 1) * RecordsPerChunk
		for _, record := range records[start : start+count] {
			line, err := canonicalLine(record)
			if err != nil || len(line) > MaxLineBytes {
				return nil, nil, Cut{}, ErrInvalid
			}
			b = append(b, line...)
		}
		total += len(b)
		if len(b) > MaxChunkBytes || total > MaxTotalBytes {
			return nil, nil, Cut{}, ErrInvalid
		}
		kind := chunkKind(ordinal)
		manifest.Artifacts = append(manifest.Artifacts, ChunkIdentity{Kind: kind, Schema: 1, Ordinal: ordinal, RecordCount: count, ByteLength: len(b), SHA256: hashBytes(b)})
		chunks[kind] = b
	}
	b, err := canonicalLine(manifest)
	if err != nil || len(b) > MaxManifestBytes || total+len(b) > MaxTotalBytes {
		return nil, nil, Cut{}, ErrInvalid
	}
	cut := Cut{View: view, ManifestSHA256: hashBytes(b), Artifacts: slices.Clone(manifest.Artifacts)}
	return b, chunks, cut, nil
}

// Parse checks a complete inventory against its independently selected cut.
// Supplying a cut copied from an untrusted manifest is not source verification.
func Parse(manifestBytes []byte, chunks map[string][]byte, expected Cut) (*Snapshot, error) {
	if len(manifestBytes) == 0 || len(manifestBytes) > MaxManifestBytes || !validView(expected.View) || !validHash(expected.ManifestSHA256) || hashBytes(manifestBytes) != expected.ManifestSHA256 {
		return nil, ErrInvalid
	}
	var manifest Manifest
	if err := decodeCanonical(manifestBytes, &manifest); err != nil {
		return nil, err
	}
	if manifest.View != expected.View || !slices.Equal(manifest.Artifacts, expected.Artifacts) || len(manifest.Artifacts) != manifest.View.ChunkCount || len(chunks) != manifest.View.ChunkCount {
		return nil, ErrInvalid
	}
	total := len(manifestBytes)
	// Validate the complete bounded byte inventory before allocating record state.
	for i, artifact := range manifest.Artifacts {
		ordinal := i + 1
		b, found := chunks[artifact.Kind]
		if !found || artifact.Kind != chunkKind(ordinal) || artifact.Schema != 1 || artifact.Ordinal != ordinal || artifact.RecordCount != chunkRecords(manifest.View, ordinal) || artifact.ByteLength <= 0 || artifact.ByteLength > MaxChunkBytes || len(b) != artifact.ByteLength || !validHash(artifact.SHA256) || hashBytes(b) != artifact.SHA256 {
			return nil, ErrInvalid
		}
		total += len(b)
		if total > MaxTotalBytes {
			return nil, ErrInvalid
		}
	}
	records := make([]Record, 0, manifest.View.RecordCount)
	for _, artifact := range manifest.Artifacts {
		remaining := chunks[artifact.Kind]
		line, rest, err := takeLine(remaining)
		if err != nil {
			return nil, err
		}
		var h header
		if err := decodeCanonical(line, &h); err != nil {
			return nil, err
		}
		if h.Type != "voice_report_journal_v1" || h.View != manifest.View || h.Ordinal != artifact.Ordinal || h.RecordCount != artifact.RecordCount {
			return nil, ErrInvalid
		}
		remaining = rest
		for n := 0; n < artifact.RecordCount; n++ {
			line, rest, err = takeLine(remaining)
			if err != nil {
				return nil, err
			}
			var record Record
			if err := decodeCanonical(line, &record); err != nil {
				return nil, err
			}
			if !validRecord(record) || (len(records) > 0 && compareRecords(records[len(records)-1], record) >= 0) {
				return nil, ErrInvalid
			}
			records = append(records, record)
			remaining = rest
		}
		if len(remaining) != 0 {
			return nil, ErrInvalid
		}
	}
	if len(records) != manifest.View.RecordCount {
		return nil, ErrInvalid
	}
	return &Snapshot{cut: cloneCut(expected), records: records}, nil
}

func (s *Snapshot) Cut() Cut {
	if s == nil {
		return Cut{}
	}
	return cloneCut(s.cut)
}

func (s *Snapshot) Records() []Record {
	if s == nil {
		return nil
	}
	return cloneRecords(s.records)
}

func validView(v View) bool {
	return v.Protocol == 1 && v.LogicalStore == "moderation" && v.StoreIncarnation != uuid.Nil && v.JournalGeneration != uuid.Nil && v.SnapshotID != uuid.Nil && v.OperationID != uuid.Nil && strings.HasPrefix(v.ViewID, "pgsnapshot-sha256:") && validHash(strings.TrimPrefix(v.ViewID, "pgsnapshot-sha256:")) && v.DiagnosticHighWater >= 0 && v.RecordCount >= 0 && v.RecordCount <= RecordsPerChunk*MaxChunks && v.ChunkCount == max(1, (v.RecordCount+RecordsPerChunk-1)/RecordsPerChunk)
}

func validRecord(r Record) bool {
	if r.ReportID == uuid.Nil || r.EvidenceID == uuid.Nil || r.EvidenceVersion <= 0 || (r.SubjectID != nil && *r.SubjectID == uuid.Nil) || (r.Slot != "REPORTER" && r.Slot != "USER_TARGET") {
		return false
	}
	if r.Disposition == "BOUND" {
		return r.SubjectID != nil && (r.EvidenceKind == "USER_OWNER_FACT" || (r.EvidenceKind == "AUTH_SUBJECT" && *r.SubjectID == r.EvidenceID))
	}
	if r.Disposition != "REDACTED" {
		return false
	}
	return (r.EvidenceKind == "AUTH_TERMINAL" && r.SubjectID != nil && *r.SubjectID == r.EvidenceID) || r.EvidenceKind == "USER_ERASURE_FACT"
}

func compareRecords(a, b Record) int {
	if c := bytes.Compare(a.ReportID[:], b.ReportID[:]); c != 0 {
		return c
	}
	return strings.Compare(a.Slot, b.Slot)
}

func chunkRecords(v View, ordinal int) int {
	return min(RecordsPerChunk, v.RecordCount-(ordinal-1)*RecordsPerChunk)
}
func chunkKind(ordinal int) string { return fmt.Sprintf("report_journal_chunk_%06d", ordinal) }
func hashBytes(b []byte) string    { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func validHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func canonicalLine(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: JSON encoding", ErrInvalid)
	}
	return append(b, '\n'), nil
}

func decodeCanonical(b []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("%w: JSON structure", ErrInvalid)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing JSON", ErrInvalid)
	}
	canonical, err := canonicalLine(out)
	if err != nil || !bytes.Equal(canonical, b) {
		return fmt.Errorf("%w: noncanonical JSON", ErrInvalid)
	}
	return nil
}

func takeLine(b []byte) ([]byte, []byte, error) {
	n := bytes.IndexByte(b, '\n')
	if n < 0 || n+1 > MaxLineBytes {
		return nil, nil, ErrInvalid
	}
	return b[:n+1], b[n+1:], nil
}

func cloneCut(c Cut) Cut { c.Artifacts = slices.Clone(c.Artifacts); return c }
func cloneRecords(records []Record) []Record {
	cloned := slices.Clone(records)
	for i := range cloned {
		if cloned[i].SubjectID != nil {
			id := *cloned[i].SubjectID
			cloned[i].SubjectID = &id
		}
	}
	return cloned
}
