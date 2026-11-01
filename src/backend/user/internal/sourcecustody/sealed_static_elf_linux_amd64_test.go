//go:build linux && amd64

package sourcecustody

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// This opt-in test only prepares descriptors in an isolated guest. It never
// executes prepared bytes and is not authentication or launch authorization.
func TestPrepareSealedStaticELFInDisposableVM(t *testing.T) {
	if os.Getenv("VOICE_DISPOSABLE_VM_SEALED_ELF_TEST") != "1" {
		t.Skip("requires isolated Linux VM fixture")
	}
	self, err := os.ReadFile("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(self)
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture")
	if err := os.WriteFile(path, self, 0600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	var pin unix.Stat_t
	if err := unix.Fstat(int(source.Fd()), &pin); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Seek(17, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	sealed, err := prepareSealedStaticELF(source, want, pin)
	if err != nil {
		t.Fatalf("prepare self-built fixture: %v", err)
	}
	defer sealed.Close()
	if pos, err := source.Seek(0, io.SeekCurrent); err != nil || pos != 17 {
		t.Fatalf("source offset changed: %d %v", pos, err)
	}
	got := make([]byte, len(self))
	if _, err := sealed.ReadAt(got, 0); err != nil || !bytes.Equal(got, self) {
		t.Fatalf("sealed bytes changed: %v", err)
	}
	seals, err := unix.FcntlInt(sealed.Fd(), unix.F_GET_SEALS, 0)
	if err != nil || seals&sealedELFRequiredSeals != sealedELFRequiredSeals {
		t.Fatalf("required seals absent: %x %v", seals, err)
	}
	flags, err := unix.FcntlInt(sealed.Fd(), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatalf("CLOEXEC missing: %v", err)
	}
	if _, err := unix.Pwrite(int(sealed.Fd()), []byte{0}, 0); err != unix.EPERM {
		t.Fatalf("write not denied: %v", err)
	}
	if err := unix.Ftruncate(int(sealed.Fd()), 0); err != unix.EPERM {
		t.Fatalf("truncate not denied: %v", err)
	}
	if err := unix.Fchmod(int(sealed.Fd()), 0600); err != unix.EPERM {
		t.Fatalf("exec mode change not denied: %v", err)
	}

	wrong := want
	wrong[0] ^= 1
	if got, err := prepareSealedStaticELF(source, wrong, pin); err == nil || got != nil {
		t.Fatal("wrong digest accepted")
	}
	badPin := pin
	badPin.Ino++
	if got, err := prepareSealedStaticELF(source, want, badPin); err == nil || got != nil {
		t.Fatal("wrong inode accepted")
	}
	rw, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	if got, err := prepareSealedStaticELF(rw, want, pin); err == nil || got != nil {
		t.Fatal("writable source accepted")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if got, err := prepareSealedStaticELF(r, want, pin); err == nil || got != nil {
		t.Fatal("pipe accepted")
	}

	dynamic := bytes.Clone(self)
	programOffset := binary.LittleEndian.Uint64(dynamic[32:40])
	if programOffset > uint64(len(dynamic)-4) {
		t.Fatal("invalid fixture program header")
	}
	binary.LittleEndian.PutUint32(dynamic[programOffset:programOffset+4], uint32(elf.PT_INTERP))
	dynamicSection := bytes.Clone(self)
	binary.LittleEndian.PutUint32(dynamicSection[programOffset:programOffset+4], uint32(elf.PT_DYNAMIC))
	for name, input := range map[string][]byte{"empty": {}, "script": []byte("#!/bin/sh\nexit 0\n"), "truncated": self[:16], "interpreter": dynamic, "dynamic-section": dynamicSection} {
		fpath := filepath.Join(dir, name)
		if err := os.WriteFile(fpath, input, 0600); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(fpath)
		if err != nil {
			t.Fatal(err)
		}
		var current unix.Stat_t
		if err := unix.Fstat(int(f.Fd()), &current); err != nil {
			t.Fatal(err)
		}
		got, err := prepareSealedStaticELF(f, sha256.Sum256(input), current)
		f.Close()
		if err == nil || got != nil {
			t.Fatalf("accepted %s", name)
		}
	}
	oversize := filepath.Join(dir, "oversize")
	f, err := os.Create(oversize)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(sealedELFMaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	f, err = os.Open(oversize)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var large unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &large); err != nil {
		t.Fatal(err)
	}
	if got, err := prepareSealedStaticELF(f, want, large); err == nil || got != nil {
		t.Fatal("oversized source accepted")
	}

	for _, stage := range []string{"create", "write", "seal", "get", "missing-seal", "read", "wrong-read", "source-close"} {
		t.Run(stage, func(t *testing.T) {
			ops := &faultELFMemfdOps{stage: stage, created: -1}
			got, err := prepareSealedStaticELFWithOps(source, want, pin, ops)
			if err == nil || got != nil {
				t.Fatalf("accepted failure at %s", stage)
			}
			if ops.created >= 0 {
				if _, err := unix.FcntlInt(uintptr(ops.created), unix.F_GETFD, 0); err != unix.EBADF {
					t.Fatalf("memfd leaked at %s: %v", stage, err)
				}
			}
		})
	}
	if _, err := rw.WriteAt([]byte("changed"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := sealed.ReadAt(got, 0); err != nil || !bytes.Equal(got, self) {
		t.Fatalf("source mutation or replacement changed sealed bytes: %v", err)
	}
}

type faultELFMemfdOps struct {
	realELFMemfdOps
	stage   string
	created int
}

func (o *faultELFMemfdOps) create() (int, error) {
	if o.stage == "create" {
		return -1, unix.ENOSYS
	}
	fd, err := o.realELFMemfdOps.create()
	o.created = fd
	return fd, err
}
func (o *faultELFMemfdOps) write(fd int, p []byte, offset int64) (int, error) {
	if o.stage == "write" {
		return 0, io.ErrShortWrite
	}
	return o.realELFMemfdOps.write(fd, p, offset)
}
func (o *faultELFMemfdOps) addSeals(fd, seals int) error {
	if o.stage == "seal" {
		return unix.EPERM
	}
	return o.realELFMemfdOps.addSeals(fd, seals)
}
func (o *faultELFMemfdOps) getSeals(fd int) (int, error) {
	if o.stage == "get" {
		return 0, unix.EINVAL
	}
	if o.stage == "missing-seal" {
		return sealedELFRequiredSeals &^ unix.F_SEAL_EXEC, nil
	}
	return o.realELFMemfdOps.getSeals(fd)
}
func (o *faultELFMemfdOps) read(fd int, p []byte, offset int64) (int, error) {
	if o.stage == "read" {
		return 0, io.ErrUnexpectedEOF
	}
	n, err := o.realELFMemfdOps.read(fd, p, offset)
	if o.stage == "wrong-read" && n > 0 {
		p[n-1] ^= 1
	}
	return n, err
}
func (o *faultELFMemfdOps) closeSource(fd int) error {
	err := o.realELFMemfdOps.closeSource(fd)
	if o.stage == "source-close" {
		return errors.New("injected close failure")
	}
	return err
}
