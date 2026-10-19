package reportrecovery

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/google/uuid"
)

// TargetRejection describes one intake decision, not positive owner evidence.
type TargetRejection struct {
	Decision      string `json:"decision"`
	Provenance    string `json:"provenance"`
	SchemaVersion int    `json:"schema_version"`
}

type subjectRecordWire struct {
	Type            string     `json:"type"`
	ReportID        uuid.UUID  `json:"report_id"`
	Slot            string     `json:"slot"`
	Disposition     string     `json:"disposition"`
	SubjectID       *uuid.UUID `json:"subject_id,omitempty"`
	EvidenceKind    string     `json:"evidence_kind"`
	EvidenceID      uuid.UUID  `json:"evidence_id"`
	EvidenceVersion int64      `json:"evidence_version"`
}
type rejectionRecordWire struct {
	Type          string    `json:"type"`
	ReportID      uuid.UUID `json:"report_id"`
	Decision      string    `json:"decision"`
	Provenance    string    `json:"provenance"`
	SchemaVersion int       `json:"schema_version"`
}

func validTargetRejection(r TargetRejection) bool {
	return (r.Decision == "MISSING" || r.Decision == "DELETED") && (r.Provenance == "OWNER_VALIDATION" || r.Provenance == "LEGACY_RECORDED_DECISION") && r.SchemaVersion == 1
}

func recordKey(r Record) string {
	if r.Rejection != nil {
		return "USER_TARGET_REJECTION"
	}
	return "SUBJECT_PRIVACY:" + r.Slot
}

func (r Record) MarshalJSON() ([]byte, error) {
	if r.Rejection != nil {
		return json.Marshal(rejectionRecordWire{"USER_TARGET_REJECTION", r.ReportID, r.Rejection.Decision, r.Rejection.Provenance, r.Rejection.SchemaVersion})
	}
	return json.Marshal(subjectRecordWire{"SUBJECT_PRIVACY", r.ReportID, r.Slot, r.Disposition, r.SubjectID, r.EvidenceKind, r.EvidenceID, r.EvidenceVersion})
}

func (r *Record) UnmarshalJSON(b []byte) error {
	var kind struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &kind); err != nil {
		return ErrInvalid
	}
	switch kind.Type {
	case "SUBJECT_PRIVACY":
		var wire subjectRecordWire
		if err := decodeRecordWire(b, &wire); err != nil {
			return err
		}
		*r = Record{ReportID: wire.ReportID, Slot: wire.Slot, Disposition: wire.Disposition, SubjectID: wire.SubjectID, EvidenceKind: wire.EvidenceKind, EvidenceID: wire.EvidenceID, EvidenceVersion: wire.EvidenceVersion}
	case "USER_TARGET_REJECTION":
		var wire rejectionRecordWire
		if err := decodeRecordWire(b, &wire); err != nil {
			return err
		}
		*r = Record{ReportID: wire.ReportID, Rejection: &TargetRejection{wire.Decision, wire.Provenance, wire.SchemaVersion}}
	default:
		return ErrInvalid
	}
	if !validRecord(*r) {
		return ErrInvalid
	}
	return nil
}

func decodeRecordWire(b []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrInvalid
	}
	return nil
}
