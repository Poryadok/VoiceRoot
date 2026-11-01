//go:build linux

package sourcecustody

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func testDenyRecord() denyRecord {
	r := denyRecord{generation: 7}
	r.boot[0], r.source[0] = 1, 2
	return r
}

func TestDenyJournalProcessCrash(t *testing.T) {
	for _, stage := range []string{"after-file-sync", "after-rename"} {
		t.Run(stage, func(t *testing.T) {
			j, dir := testDenyJournal(t)
			cmd := exec.Command(os.Args[0], "-test.run=^TestDenyJournalCrashHelper$")
			cmd.Env = append(os.Environ(), "VOICE_DENY_CRASH_DIR="+dir, "VOICE_DENY_CRASH_STAGE="+stage)
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 42 {
				t.Fatalf("helper did not reach crash point: %v", err)
			}
			err = j.matchesDeny(testDenyRecord())
			if stage == "after-file-sync" && err == nil {
				t.Fatal("unrenamed record accepted")
			}
			if stage == "after-rename" && err != nil {
				t.Fatalf("renamed record not observable: %v", err)
			}
			if err = j.persistDeny(testDenyRecord()); err != nil {
				t.Fatalf("retry: %v", err)
			}
		})
	}
}

func TestDenyJournalCrashHelper(t *testing.T) {
	dir := os.Getenv("VOICE_DENY_CRASH_DIR")
	if dir == "" {
		return
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	j, err := openDenyJournal(f)
	if err != nil {
		t.Fatal(err)
	}
	j.fail = func(stage string) error {
		if stage == os.Getenv("VOICE_DENY_CRASH_STAGE") {
			os.Exit(42)
		}
		return nil
	}
	_ = j.persistDeny(testDenyRecord())
	t.Fatal("expected injected process exit")
}

func TestDenyJournalRestrictedUmask(t *testing.T) {
	j, dir := testDenyJournal(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestDenyJournalUmaskHelper$")
	cmd.Env = append(os.Environ(), "VOICE_DENY_UMASK_DIR="+dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("umask helper: %v: %s", err, output)
	}
	if err := j.matchesDeny(testDenyRecord()); err != nil {
		t.Fatal(err)
	}
}

func TestDenyJournalUmaskHelper(t *testing.T) {
	dir := os.Getenv("VOICE_DENY_UMASK_DIR")
	if dir == "" {
		return
	}
	unix.Umask(0400)
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	j, err := openDenyJournal(f)
	if err != nil {
		t.Fatal(err)
	}
	defer j.close()
	if err := j.persistDeny(testDenyRecord()); err != nil {
		t.Fatal(err)
	}
	if err := j.matchesDeny(testDenyRecord()); err != nil {
		t.Fatal(err)
	}
}

func testDenyJournal(t *testing.T) (*denyJournal, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	j, err := openDenyJournal(f)
	if err != nil {
		info, statErr := f.Stat()
		t.Fatalf("open journal: %v; dir stat: %v, %v; euid=%d", err, info, statErr, os.Geteuid())
	}
	t.Cleanup(j.close)
	return j, dir
}

func TestDenyJournalBindingAndPersistence(t *testing.T) {
	j, dir := testDenyJournal(t)
	r := testDenyRecord()
	if j.matchesDeny(r) == nil {
		t.Fatal("missing record accepted")
	}
	if err := j.persistDeny(r); err != nil {
		t.Fatal(err)
	}
	if err := j.matchesDeny(r); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	other, err := openDenyJournal(f)
	if err != nil {
		t.Fatal(err)
	}
	defer other.close()
	if err := other.matchesDeny(r); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	for _, changed := range []denyRecord{func() denyRecord { x := r; x.generation++; return x }(), func() denyRecord { x := r; x.boot[0]++; return x }(), func() denyRecord { x := r; x.source[0]++; return x }()} {
		if other.matchesDeny(changed) == nil || other.persistDeny(changed) == nil {
			t.Fatal("mismatched record accepted")
		}
	}
	if err := os.WriteFile(filepath.Join(dir, denyFile), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if j.matchesDeny(r) == nil || j.persistDeny(r) == nil {
		t.Fatal("corrupt record accepted")
	}
}

func TestDenyJournalFailureStages(t *testing.T) {
	for _, stage := range []string{"write", "file-sync", "file-close", "rename", "directory-sync"} {
		t.Run(stage, func(t *testing.T) {
			j, _ := testDenyJournal(t)
			r := testDenyRecord()
			j.fail = func(at string) error {
				if at == stage {
					return errors.New("injected")
				}
				return nil
			}
			if j.persistDeny(r) == nil {
				t.Fatal("failure acknowledged")
			}
			j.fail = nil
			// A late failure may already have installed DENY. Retrying it is safe.
			if err := j.persistDeny(r); err != nil {
				t.Fatalf("retry: %v", err)
			}
			if err := j.matchesDeny(r); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDenyJournalRejectsSymlinkAndPermissionDrift(t *testing.T) {
	j, dir := testDenyJournal(t)
	r := testDenyRecord()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if j.persistDeny(r) == nil {
		t.Fatal("non-private directory permissions accepted")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(outside, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, denyFile)); err != nil {
		t.Fatal(err)
	}
	if j.persistDeny(r) == nil || j.matchesDeny(r) == nil {
		t.Fatal("symlink accepted")
	}
	got, err := os.ReadFile(outside)
	if err != nil || string(got) != "untouched" {
		t.Fatalf("outside file changed: %q %v", got, err)
	}
}
