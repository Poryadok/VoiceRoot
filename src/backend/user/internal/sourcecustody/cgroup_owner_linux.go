//go:build linux

// This file contains an unwired cgroup-v2 lifecycle building block.
// A successful operation is not a physical fence, custody proof, or receipt.
// The trusted host must independently provision an exclusive delegated root;
// metadata checks cannot prove that other privileged actors cannot migrate tasks.
// Source binding, atomic launch, storage isolation, a durable boot gate, and an
// independent watchdog must be established separately before any integration.
// This experiment never removes cgroups: close and failed creation can leave
// empty or populated children. A trusted provisioner must separately reconcile
// and reclaim its isolated root, without inferring a fence from this primitive.
package sourcecustody

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

var errUnavailable = errors.New("owned cgroup unavailable")

type owner struct {
	mu                  sync.Mutex
	root, child         int
	name                string
	rootStat, childStat unix.Stat_t
}

// newOwner takes a pinned, independently qualified root descriptor. The caller
// must not close it concurrently. The supervisor must remain outside child.
// This function never takes a PID or target cgroup name.
func newOwner(root *os.File) (*owner, error) {
	if root == nil {
		return nil, errUnavailable
	}
	fd, err := unix.FcntlInt(root.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	runtime.KeepAlive(root)
	if err != nil {
		return nil, errUnavailable
	}
	o := &owner{root: fd, child: -1}
	fail := func() (*owner, error) { o.close(); return nil, errUnavailable }
	var fs unix.Statfs_t
	if unix.Fstatfs(fd, &fs) != nil || uint64(fs.Type) != uint64(unix.CGROUP2_SUPER_MAGIC) || unix.Fstat(fd, &o.rootStat) != nil || !safeDir(&o.rootStat) {
		return fail()
	}
	// Reject an obviously writable migration endpoint; this is not a proof of
	// ACL, ancestor, namespace, capability, or administrator exclusivity.
	var procs unix.Stat_t
	if unix.Fstatat(fd, "cgroup.procs", &procs, unix.AT_SYMLINK_NOFOLLOW) != nil || !safeControl(&procs) {
		return fail()
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fail()
	}
	o.name = "voice-source-" + hex.EncodeToString(nonce[:])
	if unix.Mkdirat(fd, o.name, 0700) != nil {
		return fail()
	}
	o.child, err = unix.Openat(fd, o.name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil || unix.Fstat(o.child, &o.childStat) != nil || !safeDir(&o.childStat) || o.childStat.Dev != o.rootStat.Dev || !o.identities() {
		return fail()
	}
	return o, nil
}

func safeDir(s *unix.Stat_t) bool {
	return s.Mode&unix.S_IFMT == unix.S_IFDIR && s.Uid == uint32(os.Geteuid()) && s.Mode&0022 == 0
}

func safeControl(s *unix.Stat_t) bool {
	return s.Mode&unix.S_IFMT == unix.S_IFREG && s.Uid == uint32(os.Geteuid()) && s.Mode&0022 == 0
}

func sameInode(a, b *unix.Stat_t) bool { return a.Dev == b.Dev && a.Ino == b.Ino }

func (o *owner) identities() bool {
	var root, child, entry, rootProcs, childProcs unix.Stat_t
	return o.root >= 0 && o.child >= 0 && unix.Fstat(o.root, &root) == nil && safeDir(&root) && sameInode(&root, &o.rootStat) &&
		unix.Fstat(o.child, &child) == nil && safeDir(&child) && sameInode(&child, &o.childStat) &&
		unix.Fstatat(o.root, o.name, &entry, unix.AT_SYMLINK_NOFOLLOW) == nil && sameInode(&entry, &child) &&
		unix.Fstatat(o.root, "cgroup.procs", &rootProcs, unix.AT_SYMLINK_NOFOLLOW) == nil && safeControl(&rootProcs) &&
		unix.Fstatat(o.child, "cgroup.procs", &childProcs, unix.AT_SYMLINK_NOFOLLOW) == nil && safeControl(&childProcs)
}

func (o *owner) control(name string, write bool, value string) (result string, resultErr error) {
	flags := unix.O_RDONLY
	if write {
		flags = unix.O_WRONLY
	}
	fd, err := unix.Openat(o.child, name, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return "", errUnavailable
	}
	f := os.NewFile(uintptr(fd), name)
	defer func() {
		if f.Close() != nil {
			result, resultErr = "", errUnavailable
		}
	}()
	var s unix.Stat_t
	if unix.Fstat(fd, &s) != nil || !safeControl(&s) || s.Dev != o.childStat.Dev {
		return "", errUnavailable
	}
	if write {
		if n, err := io.WriteString(f, value); err != nil || n != len(value) {
			return "", errUnavailable
		}
		return "", nil
	}
	b, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(b) > 4096 {
		return "", errUnavailable
	}
	return string(b), nil
}

// stop restricts forks, requests cgroup.kill and observes populated=0. A caller
// must already have excluded all external migration and new launch paths.
// Cancellation is cooperative; filesystem operations and mutex acquisition
// cannot be forcibly interrupted by the deadline.
func (o *owner) stop(ctx context.Context) error {
	if o == nil || ctx == nil {
		return errUnavailable
	}
	if _, ok := ctx.Deadline(); !ok {
		return errUnavailable
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if ctx.Err() != nil || !o.identities() {
		return errUnavailable
	}
	if _, err := o.control("pids.max", true, "0"); err != nil {
		return errUnavailable
	}
	limit, err := o.control("pids.max", false, "")
	if err != nil || strings.TrimSpace(limit) != "0" || !o.identities() {
		return errUnavailable
	}
	if _, err = o.control("cgroup.kill", true, "1"); err != nil {
		return errUnavailable
	}
	for {
		if ctx.Err() != nil || !o.identities() {
			return errUnavailable
		}
		events, err := o.control("cgroup.events", false, "")
		if err != nil {
			return errUnavailable
		}
		populated, err := parsePopulated(events)
		if err != nil {
			return errUnavailable
		}
		if !populated && o.identities() && ctx.Err() == nil {
			return nil
		}
		if !populated {
			return errUnavailable
		}
		t := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Stop()
			return errUnavailable
		case <-t.C:
		}
	}
}

func parsePopulated(events string) (bool, error) {
	seen := false
	var populated bool
	for _, line := range strings.Split(strings.TrimSpace(events), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return false, errUnavailable
		}
		if fields[0] == "populated" {
			if seen || (fields[1] != "0" && fields[1] != "1") {
				return false, errUnavailable
			}
			seen, populated = true, fields[1] == "1"
		}
	}
	if !seen {
		return false, fmt.Errorf("%w: missing populated", errUnavailable)
	}
	return populated, nil
}

func (o *owner) close() {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.child >= 0 {
		_ = unix.Close(o.child)
		o.child = -1
	}
	if o.root >= 0 {
		_ = unix.Close(o.root)
		o.root = -1
	}
}
