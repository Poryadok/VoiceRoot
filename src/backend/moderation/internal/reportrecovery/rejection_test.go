package reportrecovery

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestTaggedRejectionFamilyRoundTrip(t *testing.T) {
	subject := fixtureRecord()
	rejected := Record{ReportID: subject.ReportID, Rejection: &TargetRejection{"MISSING", "OWNER_VALIDATION", 1}}
	v := fixtureView()
	v.RecordCount = 2
	manifest, chunks, cut, err := Encode(v, []Record{rejected, subject})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Parse(manifest, chunks, cut)
	if err != nil {
		t.Fatal(err)
	}
	records := snapshot.Records()
	if len(records) != 2 || records[1].Rejection == nil {
		t.Fatal("lost tagged rejection family")
	}
	records[1].Rejection.Decision = "DELETED"
	if snapshot.Records()[1].Rejection.Decision != "MISSING" {
		t.Fatal("exposed mutable rejection")
	}
	raw := string(line(t, rejected))
	for _, private := range []string{"target_id", "subject_id", "account_id", "evidence_id", "00000000-0000-0000-0000-000000000000"} {
		if strings.Contains(raw, private) {
			t.Fatalf("rejection contains forbidden field %s", private)
		}
	}
}

func TestRejectCorruptOrConflictingRejectionFamily(t *testing.T) {
	r := Record{ReportID: uuid.New(), Rejection: &TargetRejection{"MISSING", "LEGACY_RECORDED_DECISION", 1}}
	raw := string(line(t, r))
	for name, wire := range map[string]string{
		"unknown family":     strings.Replace(raw, "USER_TARGET_REJECTION", "AUTH_TERMINAL", 1),
		"extra subject":      strings.Replace(raw, "{", `{"subject_id":"`+uuid.NewString()+`",`, 1),
		"duplicate decision": strings.Replace(raw, "{", `{"decision":"DELETED",`, 1),
		"owner proof":        strings.Replace(raw, "LEGACY_RECORDED_DECISION", "USER_ERASURE_FACT", 1),
		"bad schema":         strings.Replace(raw, `"schema_version":1`, `"schema_version":2`, 1),
		"unknown reason":     strings.Replace(raw, "MISSING", "OWNER_UNAVAILABLE", 1),
	} {
		t.Run(name, func(t *testing.T) {
			m, c, k := fixture(t, fixtureView(), []byte(wire))
			if _, err := Parse(m, c, k); err == nil {
				t.Fatal("accepted malformed typed rejection")
			}
		})
	}
	for _, decision := range []string{"MISSING", "DELETED"} {
		second := Record{ReportID: r.ReportID, Rejection: &TargetRejection{decision, "OWNER_VALIDATION", 1}}
		v := fixtureView()
		v.RecordCount = 2
		m, c, k := fixture(t, v, line(t, r), line(t, second))
		if _, err := Parse(m, c, k); err == nil {
			t.Fatal("duplicate/conflicting report decision accepted")
		}
		if _, _, _, err := Encode(v, []Record{r, second}); err == nil {
			t.Fatal("duplicate/conflicting decision encoded")
		}
	}
	mixed := r
	mixed.SubjectID = &mixed.ReportID
	if _, _, _, err := Encode(fixtureView(), []Record{mixed}); err == nil {
		t.Fatal("mixed proof kinds accepted")
	}
}
