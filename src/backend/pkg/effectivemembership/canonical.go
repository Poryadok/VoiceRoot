package effectivemembership

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"sort"

	"github.com/google/uuid"
)

func cloneExport(h Export) Export {
	h.Heads = append([]Source(nil), h.Heads...)
	return h
}

func cloneMembers(members []Member) []Member {
	copy := append([]Member(nil), members...)
	for i := range copy {
		copy[i].Sources = append([]Source(nil), copy[i].Sources...)
	}
	return copy
}

func orderedSources(sources []Source) []Source {
	copy := append([]Source(nil), sources...)
	sort.Slice(copy, func(i, j int) bool {
		if copy[i].Owner != copy[j].Owner {
			return copy[i].Owner < copy[j].Owner
		}
		return bytes.Compare(copy[i].Scope[:], copy[j].Scope[:]) < 0
	})
	return copy
}

// CanonicalSources returns an owned deterministic copy, not an authority proof.
func CanonicalSources(sources []Source) []Source { return orderedSources(sources) }

// ContentDigest is shared with the generation producer. The digest binds the
// complete logical generation; export handles and pin deadlines are separately
// bound by ExportDigest so another export of identical G/Q can reuse its content.
// A digest is integrity evidence, never evidence of authenticated completeness.
func ContentDigest(h Export, members []Member) [32]byte {
	copy := cloneMembers(members)
	for i := range copy {
		copy[i].Sources = orderedSources(copy[i].Sources)
	}
	sort.Slice(copy, func(i, j int) bool { return bytes.Compare(copy[i].ProfileID[:], copy[j].ProfileID[:]) < 0 })
	body := struct {
		ProtocolVersion                 uint32
		ChatID, Incarnation, Generation uuid.UUID
		Q                               int64
		Count                           uint64
		Heads                           []Source
		Members                         []Member
	}{h.ProtocolVersion, h.ChatID, h.Incarnation, h.Generation, h.Q, h.Count, orderedSources(h.Heads), copy}
	raw, err := json.Marshal(body)
	if err != nil {
		return [32]byte{}
	}
	return sha256.Sum256(raw)
}

func ExportDigest(h Export) [32]byte {
	h = cloneExport(h)
	h.Heads = orderedSources(h.Heads)
	h.RetainUntil = h.RetainUntil.UTC().Round(0)
	raw, err := json.Marshal(h)
	if err != nil {
		return [32]byte{}
	}
	return sha256.Sum256(raw)
}
