package accountlifecycle

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type wireCoverage struct {
	SchemaVersion uint32 `json:"schema_version"`
	Schedule      struct {
		Fence       string    `json:"deletion_fence_id"`
		Cycle       string    `json:"deletion_cycle_id"`
		Revision    string    `json:"schedule_revision"`
		ScheduledAt time.Time `json:"scheduled_at"`
		PurgeAt     time.Time `json:"purge_at"`
	} `json:"auth_schedule"`
	Revision string   `json:"coverage_revision"`
	Complete bool     `json:"complete"`
	Count    string   `json:"profile_count"`
	Profiles []string `json:"profile_ids"`
	SHA      string   `json:"manifest_sha256"`
}

func fixture(t *testing.T) ([]Coverage, [][32]byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "testing", "a4-account-lifecycle-producer-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		First      wireCoverage `json:"user_complete_coverage"`
		FirstFrame string       `json:"manifest_preimage_base64"`
		Next       struct {
			Record wireCoverage `json:"user_complete_coverage"`
			Frame  string       `json:"manifest_preimage_base64"`
		} `json:"newer_delete_not_cancelled_by_old_restore"`
		Incomplete struct {
			Record wireCoverage `json:"record"`
			Frame  string       `json:"manifest_preimage_base64"`
		} `json:"incomplete_coverage"`
	}
	if err = json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	var records []Coverage
	var digests [][32]byte
	for i, w := range []wireCoverage{vector.First, vector.Next.Record, vector.Incomplete.Record} {
		parse := func(s string) uint64 {
			n, e := strconv.ParseUint(s, 10, 64)
			if e != nil {
				t.Fatal(e)
			}
			return n
		}
		c := Coverage{w.SchemaVersion, Schedule{w.Schedule.Fence, w.Schedule.Cycle, parse(w.Schedule.Revision), w.Schedule.ScheduledAt, w.Schedule.PurgeAt}, parse(w.Revision), w.Complete, parse(w.Count), append([]string(nil), w.Profiles...)}
		b, e := hex.DecodeString(w.SHA)
		if e != nil || len(b) != 32 {
			t.Fatal("invalid fixture digest", e)
		}
		var digest [32]byte
		copy(digest[:], b)
		frame, e := base64.StdEncoding.DecodeString([]string{vector.FirstFrame, vector.Next.Frame, vector.Incomplete.Frame}[i])
		if e != nil {
			t.Fatal(e)
		}
		if sha256.Sum256(frame) != digest {
			t.Fatal("published preimage/digest mismatch")
		}
		records = append(records, c)
		digests = append(digests, digest)
	}
	return records, digests
}

func TestDigestMatchesAllAgreedProducerVectors(t *testing.T) {
	records, expected := fixture(t)
	for i, record := range records {
		got, err := Digest(record)
		if err != nil || got != expected[i] {
			t.Errorf("vector %d: digest %x, error %v; want %x", i, got, err, expected[i])
		}
	}
}

func TestCompleteCoverageRequiresExactAcceptedScheduleAndDigest(t *testing.T) {
	records, digests := fixture(t)
	for i := range 2 {
		if err := ValidateComplete(records[i], records[i].Schedule, digests[i]); err != nil {
			t.Fatalf("complete vector %d: %v", i, err)
		}
	}
	if err := ValidateComplete(records[2], records[2].Schedule, digests[2]); err == nil {
		t.Fatal("incomplete subset accepted with its valid digest")
	}
	for _, mutate := range []func(*Coverage){
		func(c *Coverage) { c.Schedule.Fence += "foreign" },
		func(c *Coverage) { c.Schedule.Cycle += "foreign" },
		func(c *Coverage) { c.Schedule.Revision-- },
		func(c *Coverage) { c.Schedule.ScheduledAt = c.Schedule.ScheduledAt.Add(-time.Second) },
		func(c *Coverage) { c.Schedule.PurgeAt = c.Schedule.PurgeAt.Add(time.Hour) },
		func(c *Coverage) { c.ProfileCount-- },
		func(c *Coverage) { c.Revision++ },
	} {
		changed := records[0]
		mutate(&changed)
		if err := ValidateComplete(changed, records[0].Schedule, digests[0]); err == nil {
			t.Error("changed record accepted")
		}
	}
	if err := ValidateComplete(records[0], records[0].Schedule, [32]byte{}); err == nil {
		t.Fatal("wrong digest accepted")
	}
	if err := ValidateComplete(records[0], records[1].Schedule, digests[0]); err == nil {
		t.Fatal("old cycle accepted as current schedule")
	}
}

func TestDigestRejectsMalformedOrNoncanonicalSet(t *testing.T) {
	records, _ := fixture(t)
	for _, mutate := range []func(*Coverage){
		func(c *Coverage) { c.SchemaVersion = 0 },
		func(c *Coverage) { c.Schedule.Fence = "" },
		func(c *Coverage) { c.Schedule.Revision = 0 },
		func(c *Coverage) { c.Revision = 0 },
		func(c *Coverage) { c.Schedule.PurgeAt = c.Schedule.ScheduledAt },
		func(c *Coverage) { c.Profiles[0] = c.Profiles[1] },
		func(c *Coverage) { c.Profiles[0], c.Profiles[1] = c.Profiles[1], c.Profiles[0] },
		func(c *Coverage) { c.Profiles[0] = "00000000-0000-0000-0000-000000000000" },
		func(c *Coverage) { c.Profiles[0] = "not-a-uuid" },
	} {
		changed := records[0]
		changed.Profiles = append([]string(nil), changed.Profiles...)
		mutate(&changed)
		if _, err := Digest(changed); err == nil {
			t.Error("invalid record accepted")
		}
	}
}
