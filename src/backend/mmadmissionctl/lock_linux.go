//go:build linux

package mmadmissionctl

import (
	"os"
	"syscall"
)

func (j *Journal) operationLock() (func(), error) {
	f, e := os.OpenFile(j.path+".operator.lock", os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		_ = f.Close()
		return nil, ErrConflict
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
