package profileprojection

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

const readinessDomain = "voice.search.profile-generation-readiness.v1"

// ReadinessEvidence is the local, domain-separated proof of an exact ordered
// v1 authority stream. It deliberately frames offset and deterministic bytes,
// so mutation, omission, reordering and cross-protocol reuse change the hash.
type ReadinessEvidence struct {
	Generation, HighWatermark, Cutoff, Count, First, Last uint64
	Digest                                                [32]byte
}

// ReplayEvidenceCollector accepts only the exact H-bounded snapshot followed
// by the contiguous journal suffix. It is reusable across pages/restarts: the
// persisted evidence is its resume state, not a best-effort in-memory digest.
type ReplayEvidenceCollector struct{ Evidence ReadinessEvidence }

func NewReplayEvidenceCollector(generation, highWatermark uint64) (*ReplayEvidenceCollector, error) {
	evidence, err := NewReadinessEvidence(generation, highWatermark)
	if err != nil {
		return nil, err
	}
	return &ReplayEvidenceCollector{Evidence: evidence}, nil
}

func (c *ReplayEvidenceCollector) AddSnapshot(offset uint64, deterministicEvent []byte) error {
	if c == nil || offset == 0 || offset > c.Evidence.HighWatermark {
		return fmt.Errorf("snapshot event exceeds high watermark")
	}
	return c.Evidence.Add(offset, deterministicEvent)
}

func (c *ReplayEvidenceCollector) AddJournal(offset uint64, deterministicEvent []byte) error {
	if c == nil || offset <= c.Evidence.HighWatermark {
		return fmt.Errorf("journal event is outside (H,C]")
	}
	return c.Evidence.Add(offset, deterministicEvent)
}

func NewReadinessEvidence(generation, highWatermark uint64) (ReadinessEvidence, error) {
	if generation == 0 {
		return ReadinessEvidence{}, fmt.Errorf("generation is required")
	}
	b := make([]byte, 0, len(readinessDomain)+24)
	b = append(b, readinessDomain...)
	for _, value := range []uint64{generation, highWatermark} {
		var frame [10]byte
		n := binary.PutUvarint(frame[:], value)
		b = append(b, frame[:n]...)
	}
	return ReadinessEvidence{Generation: generation, HighWatermark: highWatermark, Digest: sha256.Sum256(b)}, nil
}

func (e *ReadinessEvidence) Add(offset uint64, deterministicEvent []byte) error {
	if e == nil || offset == 0 || len(deterministicEvent) == 0 {
		return fmt.Errorf("invalid readiness event")
	}
	if e.Count != 0 && offset != e.Last+1 {
		return fmt.Errorf("non-contiguous journal offset")
	}
	if e.Count == 0 {
		e.First = offset
	}
	b := make([]byte, 0, 32+20+len(deterministicEvent))
	b = append(b, e.Digest[:]...)
	var frame [10]byte
	n := binary.PutUvarint(frame[:], offset)
	b = append(b, frame[:n]...)
	n = binary.PutUvarint(frame[:], uint64(len(deterministicEvent)))
	b = append(b, frame[:n]...)
	b = append(b, deterministicEvent...)
	e.Digest = sha256.Sum256(b)
	e.Count++
	e.Last = offset
	e.Cutoff = offset
	return nil
}
