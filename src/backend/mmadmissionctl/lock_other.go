//go:build !linux

package mmadmissionctl

func (j *Journal) operationLock() (func(), error) { return nil, ErrPending }
