package effectivemembership

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type sourceFixture struct {
	header      Export
	members     []Member
	pageEdit    func(*Page)
	finalEdit   func(*Export)
	validations int
}

func fixture(t *testing.T, count int) *sourceFixture {
	t.Helper()
	h := Export{ProtocolVersion: 1, ChatID: uuid.New(), Incarnation: uuid.New(), SnapshotID: uuid.New(), Generation: uuid.New(), PinID: uuid.New(), Q: 7, RetainUntil: time.Now().Add(time.Hour).UTC()}
	h.Heads = []Source{{Owner: "chat", Scope: h.ChatID, Incarnation: h.Incarnation, Revision: 7}}
	s := &sourceFixture{header: h}
	for range count {
		id := uuid.New()
		user := Source{Owner: "user", Scope: id, Incarnation: uuid.New(), Revision: 1}
		s.header.Heads = append(s.header.Heads, user)
		s.members = append(s.members, Member{ProfileID: id, Revision: 1, Lineage: uuid.New(), Allowed: true, Sources: []Source{h.Heads[0], user}})
	}
	s.header.Count = uint64(count)
	// The fixture uses the producer's seal algorithm; the acceptance cases below
	// alter individual binding/count/content facts after sealing.
	s.header.Digest = ContentDigest(s.header, s.members)
	return s
}

func (s *sourceFixture) Begin(context.Context, uuid.UUID) (Export, error) {
	return cloneExport(s.header), nil
}
func (s *sourceFixture) Page(_ context.Context, h Export, cursor string, position uint64) (Page, error) {
	end := min(position+2, uint64(len(s.members)))
	p := Page{ChatID: h.ChatID, Incarnation: h.Incarnation, SnapshotID: h.SnapshotID, Generation: h.Generation, PinID: h.PinID, Q: h.Q, Position: position, Cursor: cursor, Members: cloneMembers(s.members[position:end]), Terminal: end == uint64(len(s.members))}
	if !p.Terminal {
		p.NextCursor = uuid.NewString()
	}
	if s.pageEdit != nil {
		s.pageEdit(&p)
	}
	return p, nil
}
func (s *sourceFixture) Validate(_ context.Context, h Export) (Export, error) {
	s.validations++
	if s.finalEdit != nil {
		s.finalEdit(&h)
	}
	return h, nil
}

func TestReceiveCompleteSealedGeneration(t *testing.T) {
	for _, count := range []int{0, 1, 129} {
		s := fixture(t, count)
		got, err := Receive(context.Background(), s, s.header.ChatID)
		require.NoError(t, err)
		require.Len(t, got.Members(), count)
		require.Equal(t, 1, s.validations, "all reads precede final authoritative head validation")
		copy := got.Export()
		copy.Heads[0].Revision++
		require.EqualValues(t, 7, got.Export().Heads[0].Revision, "capability owns its source bytes")
		if count > 0 {
			members := got.Members()
			members[0].Sources[0].Revision++
			require.EqualValues(t, 7, got.Members()[0].Sources[0].Revision)
		}
	}
}

func TestReceiveRejectsFracturedIncompleteAndCrossScopeExport(t *testing.T) {
	for _, kind := range []string{"chat", "incarnation", "snapshot", "generation", "pin", "offset", "position", "cursor", "terminal", "missing_row", "duplicate_row", "changed_row", "expired", "changed_final_head", "changed_final_seal", "missing_dependency"} {
		t.Run(kind, func(t *testing.T) {
			s := fixture(t, 3)
			s.pageEdit = func(p *Page) {
				switch kind {
				case "chat":
					p.ChatID = uuid.New()
				case "incarnation":
					p.Incarnation = uuid.New()
				case "snapshot":
					p.SnapshotID = uuid.New()
				case "generation":
					p.Generation = uuid.New()
				case "pin":
					p.PinID = uuid.New()
				case "offset":
					p.Q++
				case "position":
					p.Position++
				case "cursor":
					p.Cursor = "wrong-position"
				case "terminal":
					p.Terminal = true
					p.NextCursor = ""
				case "missing_row":
					p.Members = nil
				case "duplicate_row":
					if len(p.Members) > 1 {
						p.Members[1] = p.Members[0]
					}
				case "changed_row":
					if len(p.Members) > 0 {
						p.Members[0].Allowed = false
					}
				case "missing_dependency":
					if len(p.Members) > 0 {
						p.Members[0].Sources = p.Members[0].Sources[:1]
					}
				}
			}
			if kind == "expired" {
				s.header.RetainUntil = time.Now().Add(-time.Minute)
			}
			if kind == "changed_final_head" {
				s.finalEdit = func(h *Export) { h.Heads[0].RevocationRevision++ }
			}
			if kind == "changed_final_seal" {
				s.finalEdit = func(h *Export) { h.Q++ }
			}
			got, err := Receive(context.Background(), s, s.header.ChatID)
			require.ErrorIs(t, err, ErrSnapshotUnavailable)
			require.Empty(t, got.Members(), "no partial capability may escape failed intake")
		})
	}
}

func TestSealCanonicalOrderAndEveryCoordinateBinding(t *testing.T) {
	s := fixture(t, 3)
	original := ContentDigest(s.header, s.members)
	flipped := cloneMembers(s.members)
	flipped[0], flipped[2] = flipped[2], flipped[0]
	flipped[0].Sources[0], flipped[0].Sources[1] = flipped[0].Sources[1], flipped[0].Sources[0]
	require.Equal(t, original, ContentDigest(s.header, flipped))
	changed := cloneMembers(s.members)
	changed[0].Sources[0].RevocationRevision++
	require.NotEqual(t, original, ContentDigest(s.header, changed))
}
