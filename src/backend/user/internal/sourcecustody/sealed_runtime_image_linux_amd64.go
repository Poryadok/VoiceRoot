//go:build linux && amd64

package sourcecustody

import (
	"crypto/sha256"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// This unwired byte-preparation primitive neither authenticates a manifest nor
// qualifies image format, mounting, execution, server shutdown or custody. The
// caller must independently authenticate expected length/digest and source pin.
// A future consumer must mount this exact retained descriptor read-only, within
// independently qualified isolation. All I/O may block; an independent host
// supervisor is required. The 1 GiB bound limits memory-backed storage use.
const sealedRuntimeMaxBytes int64 = 1 << 30
const sealedRuntimeRequiredSeals = unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_EXEC | unix.F_SEAL_SEAL

// The caller retains source ownership and must not close it concurrently. The
// returned CLOEXEC file is independently owned and must be closed by its caller.
func prepareSealedRuntimeImage(source *os.File, expected [32]byte, length int64, pin unix.Stat_t) (result *os.File, observed [32]byte, resultErr error) {
	if source == nil || length <= 0 || length > sealedRuntimeMaxBytes {
		return nil, observed, errUnavailable
	}
	fd, err := unix.FcntlInt(source.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	runtime.KeepAlive(source)
	if err != nil {
		return nil, observed, errUnavailable
	}
	defer func() {
		if unix.Close(fd) != nil {
			if result != nil {
				_ = result.Close()
			}
			result, observed, resultErr = nil, [32]byte{}, errUnavailable
		}
	}()
	var before, after unix.Stat_t
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil || flags&unix.O_ACCMODE != unix.O_RDONLY || unix.Fstat(fd, &before) != nil || before.Mode&unix.S_IFMT != unix.S_IFREG || before.Size != length || !sameELFSource(&before, &pin) {
		return nil, observed, errUnavailable
	}
	// No legacy executable fallback: a kernel without MFD_NOEXEC_SEAL cannot
	// provide the non-executable data descriptor promised by this primitive.
	memfd, err := unix.MemfdCreate("voice-runtime-image", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING|unix.MFD_NOEXEC_SEAL)
	if err != nil {
		return nil, observed, errUnavailable
	}
	transferred := false
	defer func() {
		if !transferred {
			_ = unix.Close(memfd)
		}
	}()
	buf := make([]byte, 64<<10)
	hash := sha256.New()
	for offset := int64(0); offset < length; {
		size := int64(len(buf))
		if length-offset < size {
			size = length - offset
		}
		chunk := buf[:int(size)]
		if n, e := unix.Pread(fd, chunk, offset); e != nil || n != len(chunk) {
			return nil, observed, errUnavailable
		}
		_, _ = hash.Write(chunk)
		if n, e := unix.Pwrite(memfd, chunk, offset); e != nil || n != len(chunk) {
			return nil, observed, errUnavailable
		}
		offset += size
	}
	if unix.Fstat(fd, &after) != nil || !sameELFSource(&before, &after) {
		return nil, observed, errUnavailable
	}
	var copied [32]byte
	copy(copied[:], hash.Sum(nil))
	if copied != expected {
		return nil, observed, errUnavailable
	}
	if _, err = unix.FcntlInt(uintptr(memfd), unix.F_ADD_SEALS, sealedRuntimeRequiredSeals); err != nil {
		return nil, observed, errUnavailable
	}
	seals, err := unix.FcntlInt(uintptr(memfd), unix.F_GET_SEALS, 0)
	fdFlags, flagErr := unix.FcntlInt(uintptr(memfd), unix.F_GETFD, 0)
	var sealedStat unix.Stat_t
	if err != nil || seals&sealedRuntimeRequiredSeals != sealedRuntimeRequiredSeals || flagErr != nil || fdFlags&unix.FD_CLOEXEC == 0 || unix.Fstat(memfd, &sealedStat) != nil || sealedStat.Mode&unix.S_IFMT != unix.S_IFREG || sealedStat.Mode&0111 != 0 || sealedStat.Size != length {
		return nil, observed, errUnavailable
	}
	hash.Reset()
	for offset := int64(0); offset < length; {
		size := int64(len(buf))
		if length-offset < size {
			size = length - offset
		}
		chunk := buf[:int(size)]
		if n, e := unix.Pread(memfd, chunk, offset); e != nil || n != len(chunk) {
			return nil, observed, errUnavailable
		}
		_, _ = hash.Write(chunk)
		offset += size
	}
	copy(observed[:], hash.Sum(nil))
	if observed != expected {
		return nil, [32]byte{}, errUnavailable
	}
	result = os.NewFile(uintptr(memfd), "voice-runtime-image")
	if result == nil {
		return nil, [32]byte{}, errUnavailable
	}
	transferred = true
	return result, observed, nil
}
