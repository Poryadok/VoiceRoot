//go:build linux && amd64

package sourcecustody

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPrepareSealedRuntimeImage(t *testing.T) {
	data := bytes.Repeat([]byte("untrusted fixture bytes, not a runtime image"), 8192)
	path := filepath.Join(t.TempDir(), "image")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var pin unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &pin); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if _, err := f.Seek(17, 0); err != nil {
		t.Fatal(err)
	}
	sealed, observed, err := prepareSealedRuntimeImage(f, digest, int64(len(data)), pin)
	if err != nil {
		t.Fatal(err)
	}
	defer sealed.Close()
	if observed != digest {
		t.Fatal("wrong observed digest")
	}
	if pos, err := f.Seek(0, 1); err != nil || pos != 17 {
		t.Fatalf("source offset changed: %d %v", pos, err)
	}
	actual := make([]byte, len(data))
	if _, err := sealed.ReadAt(actual, 0); err != nil || !bytes.Equal(actual, data) {
		t.Fatalf("sealed content mismatch: %v", err)
	}
	seals, err := unix.FcntlInt(sealed.Fd(), unix.F_GET_SEALS, 0)
	if err != nil || seals&sealedRuntimeRequiredSeals != sealedRuntimeRequiredSeals {
		t.Fatal("missing required seals")
	}
	flags, err := unix.FcntlInt(sealed.Fd(), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatal("missing CLOEXEC")
	}
	if _, err := unix.Pwrite(int(sealed.Fd()), []byte{0}, 0); err != unix.EPERM {
		t.Fatalf("write allowed: %v", err)
	}
	if err := unix.Ftruncate(int(sealed.Fd()), 0); err != unix.EPERM {
		t.Fatalf("truncate allowed: %v", err)
	}
	var sealedStat unix.Stat_t
	if unix.Fstat(int(sealed.Fd()), &sealedStat) != nil || sealedStat.Mode&0111 != 0 {
		t.Fatal("image fd is executable")
	}
	if err := unix.Fchmod(int(sealed.Fd()), 0700); err != unix.EPERM {
		t.Fatalf("adding execute bits allowed: %v", err)
	}
	wrong := digest
	wrong[0] ^= 1
	badPin := pin
	badPin.Ino++
	for _, tc := range []struct {
		hash   [32]byte
		length int64
		pin    unix.Stat_t
	}{{wrong, int64(len(data)), pin}, {digest, int64(len(data)) + 1, pin}, {digest, 0, pin}, {digest, sealedRuntimeMaxBytes + 1, pin}, {digest, int64(len(data)), badPin}} {
		if got, _, err := prepareSealedRuntimeImage(f, tc.hash, tc.length, tc.pin); err == nil || got != nil {
			t.Fatal("invalid input accepted")
		}
	}
	rw, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	if got, _, err := prepareSealedRuntimeImage(rw, digest, int64(len(data)), pin); err == nil || got != nil {
		t.Fatal("writable source accepted")
	}
	pipe, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Close()
	defer writer.Close()
	if got, _, err := prepareSealedRuntimeImage(pipe, digest, int64(len(data)), pin); err == nil || got != nil {
		t.Fatal("pipe source accepted")
	}
	if _, err := rw.WriteAt([]byte("changed"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := sealed.ReadAt(actual, 0); err != nil || !bytes.Equal(actual, data) {
		t.Fatal("sealed bytes changed with source")
	}
	if err := os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := sealed.ReadAt(actual, 0); err != nil || !bytes.Equal(actual, data) {
		t.Fatal("pathname replacement changed sealed bytes")
	}
	if got, _, err := prepareSealedRuntimeImage(f, digest, int64(len(data)), pin); err == nil || got != nil {
		t.Fatal("changed source accepted")
	}
}
