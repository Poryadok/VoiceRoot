//go:build linux && amd64

package sourcecustody

// Private and unwired device readback for a future dedicated Linux/KVM host
// session. Merely opening /dev/watchdog0 may start a hardware countdown. This
// file must not be activated or tested against a real host watchdog until the
// exact deployment and failure behavior have been independently qualified.
//
// A successful observation is not proof of physical arming, reset timing,
// nowayout, host/boot identity, exclusive storage, boot DENY, source fencing,
// or permission to launch. Independently trusted provisioning must bind the
// expected device number and driver identity to actual physical hardware and
// exclude alternate openers. The watchdog driver/kernel may disable or
// continue pinging after process death/FD close. An independent watchdog and
// boot/storage qualification remain mandatory. There is intentionally no feed,
// disable, timeout-setting, magic-close, descriptor-close, or launch API here.

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

const fixedPhysicalWatchdogPath = "/dev/watchdog0"
const watchdogObservationLimit = 2 * time.Second
const watchdogHardMaxSeconds = 30

var errPhysicalWatchdogUnavailable = errors.New("watchdog observation unavailable")
var physicalWatchdogClaim atomic.Bool

type watchdogPolicy struct {
	rdev       uint64
	identity   [32]byte
	maxSeconds int
}

type watchdogStat struct {
	mode uint32
	rdev uint64
}
type watchdogObservation struct {
	rdev           uint64
	identity       [32]byte
	timeoutSeconds int
}

// No method here can write, feed, disable or close a device. Tests inject this
// interface; the real constructor below always selects fixed kernel operations.
type watchdogOps interface {
	open(string, int) (int, error)
	statFD(int) (watchdogStat, error)
	support(int) (unix.WatchdogInfo, error)
	getTimeout(int) (int, error)
}

type kernelWatchdogOps struct{}

func (kernelWatchdogOps) open(path string, flags int) (int, error) { return unix.Open(path, flags, 0) }
func (kernelWatchdogOps) statFD(fd int) (watchdogStat, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return watchdogStat{}, err
	}
	return watchdogStat{mode: stat.Mode, rdev: stat.Rdev}, nil
}
func (kernelWatchdogOps) support(fd int) (unix.WatchdogInfo, error) {
	info, err := unix.IoctlGetWatchdogInfo(fd)
	if err != nil || info == nil {
		return unix.WatchdogInfo{}, errPhysicalWatchdogUnavailable
	}
	return *info, nil
}
func (kernelWatchdogOps) getTimeout(fd int) (int, error) {
	return unix.IoctlGetInt(fd, unix.WDIOC_GETTIMEOUT)
}

type physicalWatchdog struct {
	policy     watchdogPolicy
	ops        watchdogOps
	wait       time.Duration
	now        func() time.Time
	state      atomic.Uint32 // 0 fresh, 1 observing, 2 observed, 3 permanently revoked
	retainedFD atomic.Int64  // raw FD plus one; never a finalizable *os.File
}

// newPhysicalWatchdog is the only non-test constructor. Policy values are
// comparisons against separately provisioned identity, not qualification.
// A claim is never recycled in this process, including after cancellation.
func newPhysicalWatchdog(p watchdogPolicy) (*physicalWatchdog, error) {
	if !validWatchdogPolicy(p) || !physicalWatchdogClaim.CompareAndSwap(false, true) {
		return nil, errPhysicalWatchdogUnavailable
	}
	return newWatchdogWithOps(p, kernelWatchdogOps{}, watchdogObservationLimit), nil
}

func newWatchdogWithOps(p watchdogPolicy, ops watchdogOps, wait time.Duration) *physicalWatchdog {
	return &physicalWatchdog{policy: p, ops: ops, wait: wait, now: time.Now}
}

func canonicalWatchdogIdentity(identity [32]byte) bool {
	if identity[0] == 0 {
		return false
	}
	terminated := false
	for _, c := range identity {
		if c == 0 {
			terminated = true
			continue
		}
		if terminated || c < 0x20 || c > 0x7e {
			return false
		}
	}
	return terminated
}

func validWatchdogPolicy(p watchdogPolicy) bool {
	return p.rdev != 0 && canonicalWatchdogIdentity(p.identity) && p.maxSeconds > 0 && p.maxSeconds <= watchdogHardMaxSeconds
}

// revoke is nonblocking and irreversible; it does not wait for a syscall and
// never closes a descriptor that might have armed the device.
func (w *physicalWatchdog) revoke() {
	if w != nil {
		w.state.Store(3)
	}
}

// observe burns its attempt before any syscall, even on prior cancellation.
// Deadline checks are cooperative: blocked kernel calls cannot be interrupted.
// The result is readback only and must not be used as a host/boot gate.
func (w *physicalWatchdog) observe(ctx context.Context) (watchdogObservation, error) {
	var zero watchdogObservation
	if w == nil || !w.state.CompareAndSwap(0, 1) {
		return zero, errPhysicalWatchdogUnavailable
	}
	if w.ops == nil || w.now == nil || !validWatchdogPolicy(w.policy) || w.wait <= 0 {
		w.revoke()
		return zero, errPhysicalWatchdogUnavailable
	}
	deadline := w.now().Add(w.wait)
	active := func() bool {
		return ctx != nil && ctx.Err() == nil && w.state.Load() == 1 && w.now().Before(deadline)
	}
	if !active() {
		w.revoke()
		return zero, errPhysicalWatchdogUnavailable
	}
	fd, err := w.ops.open(fixedPhysicalWatchdogPath, unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK)
	// A descriptor is retained even if the open reports an uncertain error.
	if fd >= 0 {
		w.retainedFD.Store(int64(fd) + 1)
	}
	if err != nil || fd < 0 || !active() {
		w.revoke()
		return zero, errPhysicalWatchdogUnavailable
	}
	stat, err := w.ops.statFD(fd)
	if err != nil || stat.mode&unix.S_IFMT != unix.S_IFCHR || stat.rdev != w.policy.rdev || !active() {
		w.revoke()
		return zero, errPhysicalWatchdogUnavailable
	}
	info, err := w.ops.support(fd)
	if err != nil || !canonicalWatchdogIdentity(info.Identity) || info.Identity != w.policy.identity || !active() {
		w.revoke()
		return zero, errPhysicalWatchdogUnavailable
	}
	seconds, err := w.ops.getTimeout(fd)
	if err != nil || seconds <= 0 || seconds > w.policy.maxSeconds || !active() {
		w.revoke()
		return zero, errPhysicalWatchdogUnavailable
	}
	if !w.state.CompareAndSwap(1, 2) {
		return zero, errPhysicalWatchdogUnavailable
	}
	if ctx.Err() != nil || !w.now().Before(deadline) || w.state.Load() != 2 {
		w.revoke()
		return zero, errPhysicalWatchdogUnavailable
	}
	return watchdogObservation{rdev: stat.rdev, identity: info.Identity, timeoutSeconds: seconds}, nil
}
