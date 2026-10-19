package mmadmissionctl

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	w "voice/backend/pkg/mmadmissionwire"
)

type crashDriver struct {
	unitDriver
	crash    bool
	launches int
}

func (d *crashDriver) StartPrepared(ctx context.Context, s Snapshot, id w.ID, register func(Evidence) error) (Evidence, error) {
	d.launches++
	if d.crash {
		panic("injected process loss after actual launch")
	}
	return d.unitDriver.StartPrepared(ctx, s, id, register)
}
func TestInterruptedArmRequiresClassifiedReconciliation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "control.sqlite")
	j, e := CreateLocalJournal(path)
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	d := &crashDriver{crash: true}
	c, e := NewLocalController(j, d, w.ID{1}, w.ID{2}, w.ID{3})
	if e != nil {
		t.Fatal(e)
	}
	spec := unitSpec()
	r, e := c.Execute(ctx, Command{Version: 1, Kind: "PREPARE", OperationID: w.ID{11}, LaunchSpec: &spec})
	if e != nil {
		t.Fatal(e)
	}
	arm := Command{Version: 1, Kind: "ARM", OperationID: w.ID{12}, ExpectedRevision: r.Revision, TransitionID: w.ID{11}}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("crash not injected")
			}
		}()
		_, _ = c.Execute(ctx, arm)
	}()
	r, e = c.Read(ctx)
	if e != nil || r.Snapshot.State != w.RecoveryRequired {
		t.Fatal("interrupted effect exposed live admission", r, e)
	}
	if _, e = c.Execute(ctx, arm); !errors.Is(e, ErrPending) {
		t.Fatal("blind arm retry", e)
	}
	if d.launches != 1 {
		t.Fatal("duplicate launch")
	}
	reconcile := Command{Version: 1, Kind: "RECONCILE", OperationID: w.ID{13}, ExpectedRevision: r.Revision, TransitionID: w.ID{11}, InterruptedOperationID: arm.OperationID}
	r, e = c.Execute(ctx, reconcile)
	if e != nil || r.Snapshot.State != w.Closed || d.closed != 1 {
		t.Fatal("reconciliation did not actually close", r, e)
	}
	if d.launches != 1 {
		t.Fatal("reconcile relaunched")
	}
}
func TestOperatorLockCoversEffectsWithoutSQLTransaction(t *testing.T) {
	j, e := CreateLocalJournal(filepath.Join(t.TempDir(), "control.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	unlock, e := j.operationLock()
	if e != nil {
		t.Fatal(e)
	}
	defer unlock()
	other, e := OpenJournal(j.path)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	if release, e := other.operationLock(); e == nil {
		release()
		t.Fatal("second operator crossed live operation")
	}
	if _, e = other.Read(context.Background()); e != nil {
		t.Fatal("readback blocked by operator lock", e)
	}
}
