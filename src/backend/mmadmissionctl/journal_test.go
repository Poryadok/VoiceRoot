package mmadmissionctl

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	w "voice/backend/pkg/mmadmissionwire"
)

func TestJournalPersistsIntentAndRejectsChangedReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.sqlite")
	j, e := CreateLocalJournal(path)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	in := Intent{OperationID: w.ID{1}, RequestHash: w.Hash{2}, Kind: "PREPARE"}
	a, e := j.Begin(ctx, in)
	if e != nil || a.Replay || a.Revision != 1 {
		t.Fatal(a, e)
	}
	_ = j.Close()
	j, e = OpenJournal(path)
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	a, e = j.Begin(ctx, in)
	if e != nil || !a.Replay || !a.Pending {
		t.Errorf("lostACK did not preserve pending intent: %+v %v", a, e)
	}
	bad := in
	bad.RequestHash[0]++
	if _, e = j.Begin(ctx, bad); !errors.Is(e, ErrConflict) {
		t.Error("changed operation replay accepted", e)
	}
	bad = in
	bad.OperationID[0]++
	bad.ExpectedRevision = 1
	if _, e = j.Begin(ctx, bad); !errors.Is(e, ErrPending) {
		t.Error("new effect crossed interrupted effect", e)
	}
}
func TestMissingJournalDoesNotCreateAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sqlite")
	if j, e := OpenJournal(path); e == nil {
		_ = j.Close()
		t.Fatal("missing journal created")
	}
}
