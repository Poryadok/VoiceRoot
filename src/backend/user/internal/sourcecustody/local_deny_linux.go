//go:build linux

package sourcecustody

// This private, unwired journal records DENY only. It never permits launch,
// proves a fence, or issues a receipt. The trusted host must independently
// provide an exclusive pinned directory on a separate trusted volume, protect
// it from PostgreSQL and other writers, and enforce deny by default at boot.
// Every generation writer must use this same pinned directory and honor its
// lifetime-held flock; the host must exclude bypass paths and protect the lock
// inode from replacement. The filesystem must support the required lock semantics.
// Successful fsync calls and local tests do not prove power-loss durability or
// rollback resistance. A separate reviewed protocol must qualify storage,
// authenticate and advance generations, and enforce the boot/watchdog gate.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"runtime"
	"sync"

	"golang.org/x/sys/unix"
)

const denyFile = "source-deny-v1"
const denyLockFile = ".source-deny.lock"

var errDenyUnavailable = errors.New("deny journal unavailable")

type denyRecord struct {
	generation uint64
	boot       [16]byte
	source     [32]byte
}

func (r denyRecord) valid() bool {
	return r.generation != 0 && r.boot != [16]byte{} && r.source != [32]byte{}
}

func (r denyRecord) encode() []byte {
	b := make([]byte, 96)
	copy(b, "VDENY001")
	binary.BigEndian.PutUint64(b[8:16], r.generation)
	copy(b[16:32], r.boot[:])
	copy(b[32:64], r.source[:])
	sum := sha256.Sum256(b[:64])
	copy(b[64:], sum[:])
	return b
}

type denyJournal struct {
	mu           sync.Mutex
	dir          int
	lock         int
	identity     unix.Stat_t
	lockIdentity unix.Stat_t
	// Fault injection is private and cannot establish journal authority.
	fail func(string) error
}

func openDenyJournal(dir *os.File) (*denyJournal, error) {
	if dir == nil {
		return nil, errDenyUnavailable
	}
	fd, err := unix.FcntlInt(dir.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	runtime.KeepAlive(dir) // Caller must not close dir concurrently.
	if err != nil {
		return nil, errDenyUnavailable
	}
	j := &denyJournal{dir: fd, lock: -1}
	if unix.Fstat(fd, &j.identity) != nil || !safeDenyDir(&j.identity) {
		_ = unix.Close(fd)
		return nil, errDenyUnavailable
	}
	lock, err := unix.Openat(fd, denyLockFile, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	created := err == nil
	if errors.Is(err, unix.EEXIST) {
		lock, err = unix.Openat(fd, denyLockFile, unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	}
	if err != nil {
		_ = unix.Close(fd)
		return nil, errDenyUnavailable
	}
	j.lock = lock
	if (created && unix.Fchmod(lock, 0600) != nil) || unix.Fstat(lock, &j.lockIdentity) != nil ||
		!safeDenyLock(&j.lockIdentity, &j.identity) || unix.Flock(lock, unix.LOCK_EX|unix.LOCK_NB) != nil || !j.rootOK() {
		_ = unix.Close(lock)
		_ = unix.Close(fd)
		return nil, errDenyUnavailable
	}
	return j, nil
}

func safeDenyLock(s, dir *unix.Stat_t) bool {
	return s.Mode&unix.S_IFMT == unix.S_IFREG && s.Mode&0777 == 0600 && s.Uid == uint32(os.Geteuid()) && s.Nlink == 1 && s.Dev == dir.Dev
}

func safeDenyDir(s *unix.Stat_t) bool {
	return s.Mode&unix.S_IFMT == unix.S_IFDIR && s.Uid == uint32(os.Geteuid()) && s.Mode&0077 == 0
}

func (j *denyJournal) rootOK() bool {
	var s, lock, entry unix.Stat_t
	return j.dir >= 0 && j.lock >= 0 && unix.Fstat(j.dir, &s) == nil && safeDenyDir(&s) && s.Dev == j.identity.Dev && s.Ino == j.identity.Ino &&
		unix.Fstat(j.lock, &lock) == nil && safeDenyLock(&lock, &s) && lock.Dev == j.lockIdentity.Dev && lock.Ino == j.lockIdentity.Ino &&
		unix.Fstatat(j.dir, denyLockFile, &entry, unix.AT_SYMLINK_NOFOLLOW) == nil && entry.Dev == lock.Dev && entry.Ino == lock.Ino
}

func (j *denyJournal) fault(stage string) error {
	if j.fail != nil {
		return j.fail(stage)
	}
	return nil
}

// read observes the record; it does not prove a prior fsync succeeded.
func (j *denyJournal) read() (denyRecord, bool, error) {
	var zero denyRecord
	if !j.rootOK() {
		return zero, false, errDenyUnavailable
	}
	fd, err := unix.Openat(j.dir, denyFile, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(err, unix.ENOENT) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, errDenyUnavailable
	}
	f := os.NewFile(uintptr(fd), denyFile)
	var before, after, entry unix.Stat_t
	statErr := unix.Fstat(fd, &before)
	b, readErr := io.ReadAll(io.LimitReader(f, 97))
	statAfterErr := unix.Fstat(fd, &after)
	closeErr := f.Close()
	if statErr != nil || statAfterErr != nil || readErr != nil || closeErr != nil || len(b) != 96 ||
		before.Mode&unix.S_IFMT != unix.S_IFREG || before.Mode&0777 != 0600 || before.Uid != uint32(os.Geteuid()) || before.Nlink != 1 || before.Dev != j.identity.Dev ||
		before.Dev != after.Dev || before.Ino != after.Ino || unix.Fstatat(j.dir, denyFile, &entry, unix.AT_SYMLINK_NOFOLLOW) != nil || entry.Dev != before.Dev || entry.Ino != before.Ino || !j.rootOK() {
		return zero, false, errDenyUnavailable
	}
	if string(b[:8]) != "VDENY001" {
		return zero, false, errDenyUnavailable
	}
	sum := sha256.Sum256(b[:64])
	if string(b[64:]) != string(sum[:]) {
		return zero, false, errDenyUnavailable
	}
	r := denyRecord{generation: binary.BigEndian.Uint64(b[8:16])}
	copy(r.boot[:], b[16:32])
	copy(r.source[:], b[32:64])
	if !r.valid() {
		return zero, false, errDenyUnavailable
	}
	return r, true, nil
}

// matchesDeny is an observation only; it cannot authorize startup or a fence.
func (j *denyJournal) matchesDeny(want denyRecord) error {
	if j == nil || !want.valid() {
		return errDenyUnavailable
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	got, present, err := j.read()
	if err != nil || !present || got != want {
		return errDenyUnavailable
	}
	return nil
}

func (j *denyJournal) persistDeny(want denyRecord) error {
	if j == nil || !want.valid() {
		return errDenyUnavailable
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	got, present, err := j.read()
	if err != nil || (present && got != want) {
		return errDenyUnavailable
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return errDenyUnavailable
	}
	name := ".source-deny-" + hex.EncodeToString(nonce[:])
	fd, err := unix.Openat(j.dir, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return errDenyUnavailable
	}
	f := os.NewFile(uintptr(fd), name)
	// On uncertainty leave the uniquely created temp file for trusted recovery.
	var created unix.Stat_t
	if unix.Fchmod(fd, 0600) != nil || unix.Fstat(fd, &created) != nil ||
		created.Mode&unix.S_IFMT != unix.S_IFREG || created.Mode&0777 != 0600 ||
		created.Uid != uint32(os.Geteuid()) || created.Nlink != 1 || created.Dev != j.identity.Dev {
		_ = f.Close()
		return errDenyUnavailable
	}
	data := want.encode()
	if j.fault("write") != nil {
		_ = f.Close()
		return errDenyUnavailable
	}
	n, writeErr := f.Write(data)
	if writeErr != nil || n != len(data) || j.fault("file-sync") != nil {
		_ = f.Close()
		return errDenyUnavailable
	}
	if f.Sync() != nil {
		_ = f.Close()
		return errDenyUnavailable
	}
	if j.fault("after-file-sync") != nil {
		_ = f.Close()
		return errDenyUnavailable
	}
	closeErr := f.Close()
	if closeErr != nil || j.fault("file-close") != nil || !j.rootOK() {
		return errDenyUnavailable
	}
	if j.fault("rename") != nil || unix.Renameat(j.dir, name, j.dir, denyFile) != nil {
		return errDenyUnavailable
	}
	if j.fault("after-rename") != nil {
		return errDenyUnavailable
	}
	if j.fault("directory-sync") != nil || unix.Fsync(j.dir) != nil || !j.rootOK() {
		return errDenyUnavailable
	}
	return nil
}

func (j *denyJournal) close() {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.dir >= 0 {
		_ = unix.Close(j.dir)
		j.dir = -1
	}
	if j.lock >= 0 {
		_ = unix.Close(j.lock)
		j.lock = -1
	}
}
