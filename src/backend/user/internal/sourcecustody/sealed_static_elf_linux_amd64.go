//go:build linux && amd64

package sourcecustody

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// This private, unwired byte-preparation primitive is not authority to launch,
// a custody proof, or a production fence. The caller must independently qualify
// the source descriptor, identity and expected digest against an authenticated
// manifest. It supports only static x86-64 ET_EXEC and never executes bytes.
const sealedELFMaxBytes = 128 << 20
const sealedELFRequiredSeals = unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_EXEC | unix.F_SEAL_SEAL

type elfMemfdOps interface {
	create() (int, error)
	write(int, []byte, int64) (int, error)
	read(int, []byte, int64) (int, error)
	addSeals(int, int) error
	getSeals(int) (int, error)
	closeSource(int) error
}

type realELFMemfdOps struct{}

func (realELFMemfdOps) create() (int, error) {
	// Explicit MFD_EXEC is required. No legacy fallback is safe under different
	// kernel vm.memfd_noexec policies; unsupported kernels fail closed.
	return unix.MemfdCreate("voice-sealed-static", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING|unix.MFD_EXEC)
}
func (realELFMemfdOps) write(fd int, b []byte, off int64) (int, error) {
	return unix.Pwrite(fd, b, off)
}
func (realELFMemfdOps) read(fd int, b []byte, off int64) (int, error) { return unix.Pread(fd, b, off) }
func (realELFMemfdOps) addSeals(fd, seals int) error {
	_, err := unix.FcntlInt(uintptr(fd), unix.F_ADD_SEALS, seals)
	return err
}
func (realELFMemfdOps) getSeals(fd int) (int, error) {
	return unix.FcntlInt(uintptr(fd), unix.F_GET_SEALS, 0)
}
func (realELFMemfdOps) closeSource(fd int) error { return unix.Close(fd) }

func sameELFSource(a, b *unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Size == b.Size && a.Mode == b.Mode &&
		a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == b.Nlink && a.Mtim == b.Mtim && a.Ctim == b.Ctim
}

// staticELFShape only enforces this primitive's limited binary shape; it does
// not prove executable safety or bind an ELF to an authenticated authority.
func staticELFShape(data []byte) bool {
	if len(data) == 0 || len(data) > sealedELFMaxBytes {
		return false
	}
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return false
	}
	if f.Class != elf.ELFCLASS64 || f.Data != elf.ELFDATA2LSB || f.Machine != elf.EM_X86_64 || f.Type != elf.ET_EXEC {
		return false
	}
	entryFound := false
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP || p.Type == elf.PT_DYNAMIC {
			return false
		}
		if p.Type != elf.PT_LOAD {
			continue
		}
		if p.Filesz > p.Memsz || p.Off > uint64(len(data)) || p.Filesz > uint64(len(data))-p.Off || p.Vaddr+p.Memsz < p.Vaddr {
			return false
		}
		if p.Flags&elf.PF_X != 0 && p.Flags&elf.PF_W != 0 {
			return false
		}
		if p.Flags&elf.PF_X != 0 && p.Filesz > 0 && f.Entry >= p.Vaddr && f.Entry-p.Vaddr < p.Filesz {
			entryFound = true
		}
	}
	return entryFound
}

func prepareSealedStaticELF(source *os.File, expected [32]byte, pin unix.Stat_t) (*os.File, error) {
	return prepareSealedStaticELFWithOps(source, expected, pin, realELFMemfdOps{})
}

// The caller retains source ownership and must not close it concurrently. The
// returned CLOEXEC descriptor is independently owned and must be closed by the
// caller. The supplied pin/digest do not authenticate themselves.
func prepareSealedStaticELFWithOps(source *os.File, expected [32]byte, pin unix.Stat_t, ops elfMemfdOps) (result *os.File, resultErr error) {
	if source == nil || ops == nil {
		return nil, errUnavailable
	}
	fd, err := unix.FcntlInt(source.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	runtime.KeepAlive(source)
	if err != nil {
		return nil, errUnavailable
	}
	defer func() {
		if ops.closeSource(fd) != nil {
			if result != nil {
				_ = result.Close()
			}
			result, resultErr = nil, errUnavailable
		}
	}()
	var before, after unix.Stat_t
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil || flags&unix.O_ACCMODE != unix.O_RDONLY || unix.Fstat(fd, &before) != nil ||
		before.Mode&unix.S_IFMT != unix.S_IFREG || before.Size <= 0 || before.Size > sealedELFMaxBytes || !sameELFSource(&before, &pin) {
		return nil, errUnavailable
	}
	data := make([]byte, int(before.Size))
	if n, err := unix.Pread(fd, data, 0); err != nil || n != len(data) || unix.Fstat(fd, &after) != nil || !sameELFSource(&before, &after) || sha256.Sum256(data) != expected || !staticELFShape(data) {
		return nil, errUnavailable
	}
	memfd, err := ops.create()
	if err != nil || memfd < 0 {
		return nil, errUnavailable
	}
	transferred := false
	defer func() {
		if !transferred {
			_ = unix.Close(memfd)
		}
	}()
	if n, err := ops.write(memfd, data, 0); err != nil || n != len(data) || ops.addSeals(memfd, sealedELFRequiredSeals) != nil {
		return nil, errUnavailable
	}
	seals, err := ops.getSeals(memfd)
	var sealedStat unix.Stat_t
	fdFlags, fdErr := unix.FcntlInt(uintptr(memfd), unix.F_GETFD, 0)
	if err != nil || seals&sealedELFRequiredSeals != sealedELFRequiredSeals || unix.Fstat(memfd, &sealedStat) != nil ||
		sealedStat.Mode&unix.S_IFMT != unix.S_IFREG || sealedStat.Mode&0111 == 0 || sealedStat.Size != before.Size || fdErr != nil || fdFlags&unix.FD_CLOEXEC == 0 {
		return nil, errUnavailable
	}
	verified := make([]byte, len(data))
	if n, err := ops.read(memfd, verified, 0); err != nil || n != len(verified) || sha256.Sum256(verified) != expected || !staticELFShape(verified) {
		return nil, errUnavailable
	}
	result = os.NewFile(uintptr(memfd), "voice-sealed-static")
	if result == nil {
		return nil, errUnavailable
	}
	transferred = true
	return result, nil
}
