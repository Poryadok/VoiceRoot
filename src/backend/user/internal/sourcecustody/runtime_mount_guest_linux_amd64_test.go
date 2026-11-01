//go:build linux && amd64

package sourcecustody

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Disposable guest experiment only. The digest pins one self-built inert
// SquashFS fixture; it is neither a PostgreSQL image nor an image trust policy.
const runtimeMountFixtureSHA = "fb1bfe2a8fa015c560975e48b0ce22741e30a66b1e70a018bbab862abb646b27"

func TestRuntimeImageMountDisposableVM(t *testing.T) {
	if os.Getenv("VOICE_DISPOSABLE_VM_RUNTIME_MOUNT") != "1" {
		t.Skip("requires owned disposable Linux VM")
	}
	if os.Geteuid() != 0 {
		t.Fatal("guest root required")
	}
	source, err := os.Open("/tmp/test/image.sqfs")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	var pin unix.Stat_t
	if err = unix.Fstat(int(source.Fd()), &pin); err != nil {
		t.Fatal(err)
	}
	d, err := hex.DecodeString(runtimeMountFixtureSHA)
	if err != nil {
		t.Fatal(err)
	}
	var expected [32]byte
	copy(expected[:], d)
	image, _, err := prepareSealedRuntimeImage(source, expected, pin.Size, pin)
	if err != nil {
		t.Fatal(err)
	}
	defer image.Close()
	var before, after unix.Stat_t
	if unix.Stat("/tmp", &before) != nil {
		t.Fatal("parent /tmp unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/proc/self/exe", "-test.run=^TestRuntimeMountGuestBootstrap$", "-test.v")
	cmd.Env = []string{"VOICE_RUNTIME_MOUNT_GUEST_HELPER=1"}
	cmd.ExtraFiles = []*os.File{image}
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWNS}
	cmd.WaitDelay = 3 * time.Second
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("guest bootstrap: %v: %s", err, output)
	}
	if unix.Stat("/tmp", &after) != nil || before.Dev != after.Dev || before.Ino != after.Ino {
		t.Fatal("parent namespace changed")
	}
	if _, err := os.Stat("/runtime"); !os.IsNotExist(err) {
		t.Fatal("private mount leaked into parent namespace")
	}
	t.Log(string(output))
}

// This test-only bootstrap never executes image contents. It receives exactly
// one inherited sealed fixture descriptor and is run only in the offline VM.
func TestRuntimeMountGuestBootstrap(t *testing.T) {
	if os.Getenv("VOICE_RUNTIME_MOUNT_GUEST_HELPER") != "1" {
		return
	}
	var backing unix.Stat_t
	if unix.Fstat(3, &backing) != nil || backing.Mode&0111 != 0 || backing.Size != 4096 {
		t.Fatal("bad image fd")
	}
	seals, err := unix.FcntlInt(3, unix.F_GET_SEALS, 0)
	if err != nil || seals&sealedRuntimeRequiredSeals != sealedRuntimeRequiredSeals {
		t.Fatal("bad seals")
	}
	image := os.NewFile(3, "fixture")
	bytes, err := os.ReadFile("/proc/self/fd/3")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(bytes)) != runtimeMountFixtureSHA {
		t.Fatal("wrong fixture digest")
	}
	if unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, "") != nil {
		t.Fatal("private propagation unavailable")
	}
	if unix.Mount("tmpfs", "/tmp", "tmpfs", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, "mode=0700,size=16m") != nil {
		t.Fatal("private tmpfs unavailable")
	}
	if os.Mkdir("/tmp/root", 0700) != nil || unix.Mount("tmpfs", "/tmp/root", "tmpfs", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, "mode=0700,size=8m") != nil {
		t.Fatal("private root unavailable")
	}
	if os.Mkdir("/tmp/root/old", 0700) != nil || os.Mkdir("/tmp/root/runtime", 0700) != nil {
		t.Fatal("private targets unavailable")
	}
	ctl, err := unix.Open("/dev/loop-control", unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	index, err := unix.IoctlRetInt(ctl, unix.LOOP_CTL_GET_FREE)
	_ = unix.Close(ctl)
	if err != nil || index < 0 {
		t.Fatal("loop unavailable")
	}
	loop, err := unix.Open("/dev/loop"+strconv.Itoa(index), unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(loop)
	var device unix.Stat_t
	if unix.Fstat(loop, &device) != nil || device.Mode&unix.S_IFMT != unix.S_IFBLK || unix.Major(uint64(device.Rdev)) != 7 || unix.Minor(uint64(device.Rdev)) != uint32(index) {
		t.Fatal("wrong loop device")
	}
	config := unix.LoopConfig{Fd: 3, Info: unix.LoopInfo64{Sizelimit: uint64(backing.Size), Flags: unix.LO_FLAGS_READ_ONLY | unix.LO_FLAGS_AUTOCLEAR}}
	if unix.IoctlLoopConfigure(loop, &config) != nil {
		t.Fatal("atomic loop configuration failed")
	}
	info, err := unix.IoctlLoopGetStatus64(loop)
	if err != nil || info.Device != uint64(backing.Dev) || info.Inode != backing.Ino || info.Offset != 0 || info.Sizelimit != uint64(backing.Size) || info.Number != uint32(index) || info.Flags != unix.LO_FLAGS_READ_ONLY|unix.LO_FLAGS_AUTOCLEAR || info.Encrypt_type != 0 || info.Encrypt_key_size != 0 {
		t.Fatal("loop identity mismatch")
	}
	if unix.Mount("/proc/self/fd/"+strconv.Itoa(loop), "/tmp/root/runtime", "squashfs", unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, "") != nil {
		t.Fatal("sealed fixture mount failed")
	}
	if unix.Chdir("/tmp/root") != nil || unix.PivotRoot(".", "old") != nil || unix.Chdir("/") != nil || unix.Unmount("/old", unix.MNT_DETACH) != nil || os.Remove("/old") != nil {
		t.Fatal("private root transition failed")
	}
	var fs unix.Statfs_t
	if unix.Statfs("/runtime", &fs) != nil || uint64(fs.Type) != uint64(unix.SQUASHFS_MAGIC) || fs.Flags&(unix.ST_RDONLY|unix.ST_NOSUID|unix.ST_NODEV|unix.ST_NOEXEC) != unix.ST_RDONLY|unix.ST_NOSUID|unix.ST_NODEV|unix.ST_NOEXEC {
		t.Fatal("wrong filesystem or mount flags")
	}
	if _, err := os.Stat("/proc"); !os.IsNotExist(err) {
		t.Fatal("old proc reachable")
	}
	if _, err := os.Stat("/dev"); !os.IsNotExist(err) {
		t.Fatal("old dev reachable")
	}
	// Capabilities and no-new-privileges are per-thread. Other Go runtime
	// threads may remain privileged: this is not a production demotion proof.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0) != nil {
		t.Fatal("no-new-privileges unavailable")
	}
	var header unix.CapUserHeader
	header.Version = unix.LINUX_CAPABILITY_VERSION_3
	var caps [2]unix.CapUserData
	if unix.Capset(&header, &caps[0]) != nil {
		t.Fatal("capability drop failed")
	}
	var actual [2]unix.CapUserData
	if unix.Capget(&header, &actual[0]) != nil || actual != [2]unix.CapUserData{} {
		t.Fatal("capability readback failed")
	}
	if n, err := unix.PrctlRetInt(unix.PR_GET_NO_NEW_PRIVS, 0, 0, 0, 0); err != nil || n != 1 {
		t.Fatal("no-new-privileges readback failed")
	}
	got, err := os.ReadFile("/runtime/sentinel")
	if err != nil || string(got) != "voice-fixed-fixture" {
		t.Fatal("fixture content mismatch")
	}
	_ = image.Close()
}
