package reportrecovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func fixtureView() View {
	return View{Protocol: 1, LogicalStore: "moderation", StoreIncarnation: uuid.MustParse("00000000-0000-4000-8000-000000000001"), JournalGeneration: uuid.MustParse("00000000-0000-4000-8000-000000000002"), SnapshotID: uuid.MustParse("00000000-0000-4000-8000-000000000003"), OperationID: uuid.MustParse("00000000-0000-4000-8000-000000000004"), ViewID: "pgsnapshot-sha256:" + strings.Repeat("a", 64), DiagnosticHighWater: 9001, RecordCount: 1, ChunkCount: 1}
}

func fixtureRecord() Record {
	subject := uuid.MustParse("00000000-0000-4000-8000-000000000006")
	return Record{ReportID: uuid.MustParse("00000000-0000-4000-8000-000000000005"), Slot: "REPORTER", Disposition: "BOUND", SubjectID: &subject, EvidenceKind: "AUTH_SUBJECT", EvidenceID: subject, EvidenceVersion: 1}
}

func line(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func digest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

// Constructs trusted expected byte identities independently of Encode. Mutated
// fixtures therefore exercise domain parsing, not merely a checksum mismatch.
func fixture(t *testing.T, v View, rows ...[]byte) ([]byte, map[string][]byte, Cut) {
	t.Helper()
	b := line(t, header{Type: "voice_report_journal_v1", View: v, Ordinal: 1, RecordCount: len(rows)})
	for _, r := range rows {
		b = append(b, r...)
	}
	a := ChunkIdentity{Kind: "report_journal_chunk_000001", Schema: 1, Ordinal: 1, RecordCount: len(rows), ByteLength: len(b), SHA256: digest(b)}
	m := line(t, Manifest{View: v, Artifacts: []ChunkIdentity{a}})
	return m, map[string][]byte{a.Kind: b}, Cut{View: v, ManifestSHA256: digest(m), Artifacts: []ChunkIdentity{a}}
}

func TestCanonicalRoundTripAndImmutableSnapshot(t *testing.T) {
	v, r := fixtureView(), fixtureRecord()
	m, chunks, cut, err := Encode(v, []Record{r})
	if err != nil {
		t.Fatal(err)
	}
	gold, parts, expected := fixture(t, v, line(t, r))
	if string(m) != string(gold) || string(chunks[cut.Artifacts[0].Kind]) != string(parts[expected.Artifacts[0].Kind]) {
		t.Fatal("noncanonical output")
	}
	s, err := Parse(m, chunks, expected)
	if err != nil || len(s.Records()) != 1 {
		t.Fatalf("parse: %v", err)
	}
	s.Records()[0].SubjectID[0] = 255
	s.Cut().Artifacts[0].Kind = "changed"
	chunks[expected.Artifacts[0].Kind][0] = '!'
	if *s.Records()[0].SubjectID != *r.SubjectID || s.Cut().Artifacts[0].Kind != expected.Artifacts[0].Kind {
		t.Fatal("snapshot exposes mutable identity")
	}
}

func TestRejectNoncanonicalAndPrivateFields(t *testing.T) {
	raw := string(line(t, fixtureRecord()))
	for name, row := range map[string]string{
		"raw account":         strings.Replace(raw, "{", `{"account_id":"private",`, 1),
		"raw profile":         strings.Replace(raw, "{", `{"profile_id":"private",`, 1),
		"content":             strings.Replace(raw, "{", `{"content":"private",`, 1),
		"duplicate key":       strings.Replace(raw, "{", `{"slot":"REPORTER",`, 1),
		"whitespace":          " " + raw,
		"truncated":           strings.TrimSuffix(raw, "\n"),
		"unknown disposition": strings.Replace(raw, "BOUND", "READY", 1),
		"zero evidence":       strings.Replace(raw, `"evidence_version":1`, `"evidence_version":0`, 1),
		"noncanonical UUID":   strings.Replace(raw, "00000000-0000-4000-8000-000000000005", "00000000000040008000000000000005", 1),
		"long line":           strings.Repeat(" ", MaxLineBytes) + raw,
	} {
		t.Run(name, func(t *testing.T) {
			m, chunks, cut := fixture(t, fixtureView(), []byte(row))
			if _, err := Parse(m, chunks, cut); err == nil {
				t.Fatal("accepted invalid record with correct byte hashes")
			}
		})
	}
}

func TestRejectIncoherentRecordsAndView(t *testing.T) {
	for _, mutate := range []func(*Record){
		func(r *Record) { r.SubjectID = nil },
		func(r *Record) { r.ReportID = uuid.Nil },
		func(r *Record) { r.Slot = "OTHER" },
		func(r *Record) { r.EvidenceKind = "USER_ERASURE_FACT" },
		func(r *Record) {
			r.Disposition = "REDACTED"
			r.EvidenceKind = "AUTH_TERMINAL"
			r.EvidenceID = uuid.New()
		},
	} {
		r := fixtureRecord()
		mutate(&r)
		m, chunks, cut := fixture(t, fixtureView(), line(t, r))
		if _, err := Parse(m, chunks, cut); err == nil {
			t.Fatal("accepted incoherent record")
		}
	}
	for _, mutate := range []func(*View){
		func(v *View) { v.Protocol = 2 }, func(v *View) { v.LogicalStore = "auth" },
		func(v *View) { v.OperationID = uuid.Nil }, func(v *View) { v.DiagnosticHighWater = -1 },
		func(v *View) { v.RecordCount = 2 }, func(v *View) { v.ChunkCount = 2 },
		func(v *View) { v.ViewID = "arbitrary private value" },
	} {
		v := fixtureView()
		mutate(&v)
		m, chunks, cut := fixture(t, v, line(t, fixtureRecord()))
		if _, err := Parse(m, chunks, cut); err == nil {
			t.Fatal("accepted incoherent view")
		}
	}
}

func TestRejectDuplicateAndConflictingSlot(t *testing.T) {
	for _, differentSubject := range []bool{false, true} {
		r, second := fixtureRecord(), fixtureRecord()
		if differentSubject {
			u := uuid.New()
			second.SubjectID = &u
		}
		v := fixtureView()
		v.RecordCount = 2
		m, chunks, cut := fixture(t, v, line(t, r), line(t, second))
		if _, err := Parse(m, chunks, cut); err == nil {
			t.Fatal("duplicate slot accepted")
		}
		if _, _, _, err := Encode(v, []Record{r, second}); err == nil {
			t.Fatal("duplicate encoded")
		}
	}
}

func TestExactExpectedCutAndArtifactInventory(t *testing.T) {
	for _, mutate := range []func([]byte, map[string][]byte, *Cut){
		func(_ []byte, c map[string][]byte, _ *Cut) { delete(c, "report_journal_chunk_000001") },
		func(_ []byte, c map[string][]byte, _ *Cut) { c["report_journal_chunk_000002"] = []byte("extra") },
		func(_ []byte, c map[string][]byte, _ *Cut) { c["report_journal_chunk_000001"][0] = '!' },
		func(m []byte, _ map[string][]byte, _ *Cut) { m[0] = '!' },
		func(_ []byte, _ map[string][]byte, c *Cut) { c.View.SnapshotID = uuid.New() },
		func(_ []byte, _ map[string][]byte, c *Cut) { c.Artifacts[0].RecordCount++ },
		func(_ []byte, _ map[string][]byte, c *Cut) { c.ManifestSHA256 = strings.Repeat("0", 64) },
	} {
		m, chunks, cut := fixture(t, fixtureView(), line(t, fixtureRecord()))
		mutate(m, chunks, &cut)
		if _, err := Parse(m, chunks, cut); err == nil {
			t.Fatal("accepted changed expected cut or bytes")
		}
	}
}

func TestEmptyAndCompleteTwoChunkSnapshot(t *testing.T) {
	for _, n := range []int{0, RecordsPerChunk + 1} {
		v := fixtureView()
		v.RecordCount = n
		v.ChunkCount = max(1, (n+RecordsPerChunk-1)/RecordsPerChunk)
		records := make([]Record, n)
		for i := range records {
			records[i] = fixtureRecord()
			records[i].ReportID = uuid.MustParse(fmt.Sprintf("00000000-0000-4000-8000-%012x", i+1))
		}
		m, chunks, cut, err := Encode(v, records)
		if err != nil {
			t.Fatal(err)
		}
		s, err := Parse(m, chunks, cut)
		if err != nil || len(s.Records()) != n {
			t.Fatalf("complete snapshot: %v", err)
		}
		if n > RecordsPerChunk {
			delete(chunks, "report_journal_chunk_000002")
			if _, err := Parse(m, chunks, cut); err == nil {
				t.Fatal("partial chunk set accepted")
			}
		}
	}
}

func TestCanonicalManifestHeaderAndGlobalOrder(t *testing.T) {
	for _, mutate := range []func([]byte) []byte{
		func(b []byte) []byte { return append([]byte(" "), b...) },
		func(b []byte) []byte { return []byte(strings.Replace(string(b), "{", `{"unknown":true,`, 1)) },
		func(b []byte) []byte { return []byte(strings.Replace(string(b), "{", `{"view":null,`, 1)) },
	} {
		m, chunks, cut := fixture(t, fixtureView(), line(t, fixtureRecord()))
		m = mutate(m)
		cut.ManifestSHA256 = digest(m)
		if _, err := Parse(m, chunks, cut); err == nil {
			t.Fatal("noncanonical manifest accepted")
		}
	}
	for _, mutate := range []func(*header){
		func(h *header) { h.View.SnapshotID = uuid.New() },
		func(h *header) { h.Ordinal = 2 },
		func(h *header) { h.Type = "other" },
		func(h *header) { h.RecordCount = 0 },
	} {
		v := fixtureView()
		h := header{Type: "voice_report_journal_v1", View: v, Ordinal: 1, RecordCount: 1}
		mutate(&h)
		_, chunks, cut := fixture(t, v, line(t, fixtureRecord()))
		kind := cut.Artifacts[0].Kind
		chunks[kind] = append(line(t, h), line(t, fixtureRecord())...)
		cut.Artifacts[0].ByteLength = len(chunks[kind])
		cut.Artifacts[0].SHA256 = digest(chunks[kind])
		m := line(t, Manifest{View: v, Artifacts: cut.Artifacts})
		cut.ManifestSHA256 = digest(m)
		if _, err := Parse(m, chunks, cut); err == nil {
			t.Fatal("mixed or incorrect header accepted")
		}
	}
	v := fixtureView()
	v.RecordCount = 2
	first, second := fixtureRecord(), fixtureRecord()
	second.Slot = "USER_TARGET"
	// Encoding sorts a private copy; parsing accepts only that single ordering.
	m, chunks, cut, err := Encode(v, []Record{second, first})
	if err != nil {
		t.Fatal(err)
	}
	s, err := Parse(m, chunks, cut)
	if err != nil || s.Records()[0].Slot != "REPORTER" {
		t.Fatalf("sort: %v", err)
	}
	m, chunks, cut = fixture(t, v, line(t, second), line(t, first))
	if _, err := Parse(m, chunks, cut); err == nil {
		t.Fatal("out-of-order rows accepted")
	}
}

func TestTerminalEvidenceAndCapacity(t *testing.T) {
	for _, kind := range []string{"AUTH_TERMINAL", "USER_ERASURE_FACT"} {
		r := fixtureRecord()
		r.Disposition = "REDACTED"
		r.EvidenceKind = kind
		if kind == "USER_ERASURE_FACT" {
			r.SubjectID = nil
			r.EvidenceID = uuid.New()
			r.EvidenceVersion = 7
		}
		m, chunks, cut := fixture(t, fixtureView(), line(t, r))
		if _, err := Parse(m, chunks, cut); err != nil {
			t.Fatalf("valid terminal %s: %v", kind, err)
		}
	}
	v := fixtureView()
	v.RecordCount = RecordsPerChunk*MaxChunks + 1
	v.ChunkCount = MaxChunks + 1
	if _, _, _, err := Encode(v, nil); err == nil {
		t.Fatal("unbounded record count accepted")
	}
	_, chunks, cut := fixture(t, fixtureView(), line(t, fixtureRecord()))
	chunks[cut.Artifacts[0].Kind] = make([]byte, MaxChunkBytes+1)
	cut.Artifacts[0].ByteLength = MaxChunkBytes + 1
	cut.Artifacts[0].SHA256 = digest(chunks[cut.Artifacts[0].Kind])
	m := line(t, Manifest{View: cut.View, Artifacts: cut.Artifacts})
	cut.ManifestSHA256 = digest(m)
	if _, err := Parse(m, chunks, cut); err == nil {
		t.Fatal("oversized artifact accepted")
	}
}

func TestAuthSubjectEvidenceIdentity(t *testing.T) {
	r := fixtureRecord()
	r.EvidenceID = uuid.New()
	t.Run("parse", func(t *testing.T) {
		m, chunks, cut := fixture(t, fixtureView(), line(t, r))
		if _, err := Parse(m, chunks, cut); err == nil {
			t.Fatal("accepted unrelated Auth evidence identity")
		}
	})
	t.Run("encode", func(t *testing.T) {
		if _, _, _, err := Encode(fixtureView(), []Record{r}); err == nil {
			t.Fatal("encoded unrelated Auth evidence identity")
		}
	})
	t.Run("distinct_user_owner_fact", func(t *testing.T) {
		r.EvidenceKind = "USER_OWNER_FACT"
		m, chunks, cut, err := Encode(fixtureView(), []Record{r})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(m, chunks, cut); err != nil {
			t.Fatal(err)
		}
	})
}
