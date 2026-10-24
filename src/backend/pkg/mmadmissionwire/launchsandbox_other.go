//go:build !linux || !amd64

package mmadmissionwire

func DenyFurtherExec() error { return ErrInvalid }
