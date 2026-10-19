//go:build linux

package mmadmissionctl

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"syscall"
	w "voice/backend/pkg/mmadmissionwire"
)

// NativeBootstrapChannel owns a private driver/child socketpair. Only Child is
// passed as exec.Cmd.ExtraFiles[0] (fixed FD3); parent closes its Child handle
// immediately after Start. Deliver happens after actual launch observations and
// protected public pin installation, while native startup is still closed.
type NativeBootstrapChannel struct {
	Driver *net.UnixConn
	Child  *os.File
}

func NewNativeBootstrapChannel() (*NativeBootstrapChannel, error) {
	if os.Geteuid() != 0 {
		return nil, w.ErrInvalid
	}
	pair, e := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	driver := os.NewFile(uintptr(pair[0]), "mm-bootstrap-driver")
	child := os.NewFile(uintptr(pair[1]), "mm-bootstrap-child")
	raw, e := net.FileConn(driver)
	_ = driver.Close()
	if e != nil {
		_ = child.Close()
		return nil, e
	}
	conn, ok := raw.(*net.UnixConn)
	if !ok {
		_ = raw.Close()
		_ = child.Close()
		return nil, w.ErrInvalid
	}
	return &NativeBootstrapChannel{Driver: conn, Child: child}, nil
}
func (c *NativeBootstrapChannel) Deliver(ctx context.Context, descriptor []byte, recipient w.KernelPeer, expected w.LaunchExpectations, own tls.Certificate) error {
	if c == nil || c.Driver == nil {
		return w.ErrInvalid
	}
	defer c.Driver.Close()
	return w.WriteNativeLaunchBootstrap(ctx, c.Driver, descriptor, recipient, expected, own)
}
func (c *NativeBootstrapChannel) Close() error {
	if c == nil {
		return nil
	}
	if c.Child != nil {
		_ = c.Child.Close()
	}
	if c.Driver != nil {
		return c.Driver.Close()
	}
	return nil
}
