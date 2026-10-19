//go:build linux

package fencecustody

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"voice/backend/voice/internal/retirementproof"
)

const maxInstalledDocument = 1 << 20

// Every path component is opened relative to an already checked descriptor.
// Installer ownership is UID0; no group/other writer or symbolic link is
// accepted, including in ancestors. This is local provenance, not evidence of
// deployment completeness or immunity to an installation administrator.
func protectedDirectory(path string) (int, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return -1, retirementproof.ErrUnavailable
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	check := func(fd int) bool {
		var st unix.Stat_t
		return unix.Fstat(fd, &st) == nil && st.Uid == 0 && st.Mode&0022 == 0 && st.Mode&unix.S_IFMT == unix.S_IFDIR
	}
	if !check(fd) {
		unix.Close(fd)
		return -1, retirementproof.ErrUnavailable
	}
	for _, component := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if component == "" {
			continue
		}
		next, e := unix.Openat(fd, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if e != nil {
			return -1, retirementproof.ErrUnavailable
		}
		fd = next
		if !check(fd) {
			unix.Close(fd)
			return -1, retirementproof.ErrUnavailable
		}
	}
	return fd, nil
}

func documentName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name && !strings.ContainsAny(name, "/\\\x00")
}

func readInstalledFile(root, name string, secret bool) ([]byte, error) {
	if !documentName(name) {
		return nil, retirementproof.ErrUnavailable
	}
	dir, err := protectedDirectory(root)
	if err != nil {
		return nil, err
	}
	defer unix.Close(dir)
	fd, err := unix.Openat(dir, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, retirementproof.ErrUnavailable
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil || st.Uid != 0 || st.Mode&0022 != 0 || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 || st.Size <= 0 || st.Size > maxInstalledDocument || (secret && st.Mode&0077 != 0) {
		return nil, retirementproof.ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxInstalledDocument+1))
	if err != nil || len(raw) == 0 || len(raw) > maxInstalledDocument {
		return nil, retirementproof.ErrUnavailable
	}
	return raw, nil
}

// Atomic no-replace publication survives a crash before rename without exposing
// partial receipts. Exact duplicate acknowledgements read the retained bytes.
func writeInstalledFile(root, name string, raw []byte) error {
	if os.Geteuid() != 0 || !documentName(name) || len(raw) == 0 || len(raw) > maxInstalledDocument {
		return retirementproof.ErrUnavailable
	}
	dir, err := protectedDirectory(root)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	temporary := ".pending-" + uuid.NewString()
	fd, err := unix.Openat(dir, temporary, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	defer unix.Unlinkat(dir, temporary, 0)
	f := os.NewFile(uintptr(fd), temporary)
	if _, err = f.Write(raw); err == nil {
		err = f.Chmod(0444)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = unix.Renameat2(dir, temporary, dir, name, unix.RENAME_NOREPLACE); err != nil {
		if !errors.Is(err, unix.EEXIST) {
			return err
		}
		existing, readErr := readInstalledFile(root, name, false)
		if readErr != nil || !bytes.Equal(existing, raw) {
			return retirementproof.ErrConflict
		}
	}
	return unix.Fsync(dir)
}
