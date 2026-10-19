//go:build linux

package mmadmissionwire

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"sync/atomic"
	"syscall"
	"time"
)

var nativeBootstrapConsumed atomic.Bool

// LoadNativeReadbackClient consumes only inherited FD3, once per process.
// A missing/invalid channel never falls back to environment or public pin data.
// The driver and serving process must share a compatible PID namespace: a zero
// SO_PEERCRED PID is rejected. No capability is added to the serving process.
func LoadNativeReadbackClient(ctx context.Context, descriptor []byte, timeout time.Duration) (Client, error) {
	if nativeBootstrapConsumed.Swap(true) {
		return Client{}, ErrInvalid
	}
	syscall.CloseOnExec(NativeBootstrapFD)
	file := os.NewFile(NativeBootstrapFD, "mm-native-private-bootstrap")
	if file == nil {
		return Client{}, ErrInvalid
	}
	raw, e := net.FileConn(file)
	_ = file.Close()
	if e != nil {
		return Client{}, ErrInvalid
	}
	defer raw.Close()
	conn, ok := raw.(*net.UnixConn)
	if !ok || os.Geteuid() == 0 {
		return Client{}, ErrInvalid
	}
	// Keys must never become available to an executable that can subsequently
	// inherit them into another image. This mandatory profile only narrows self.
	if privateBootstrapDriver(conn) != nil || DenyFurtherExec() != nil {
		return Client{}, ErrInvalid
	}
	return consumeNativeBootstrap(ctx, conn, descriptor, KernelPeer{int32(os.Getpid()), uint32(os.Geteuid()), uint32(os.Getegid())}, timeout)
}
func consumeNativeBootstrap(ctx context.Context, conn *net.UnixConn, descriptor []byte, recipient KernelPeer, timeout time.Duration) (Client, error) {
	if ctx == nil || ctx.Err() != nil || conn == nil {
		return Client{}, ErrInvalid
	}
	if privateBootstrapDriver(conn) != nil {
		return Client{}, ErrInvalid
	}
	rawConn, e := conn.SyscallConn()
	if e != nil {
		return Client{}, ErrInvalid
	}
	var kind int
	var inner error
	if e = rawConn.Control(func(fd uintptr) { kind, inner = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_TYPE) }); e != nil || inner != nil || kind != syscall.SOCK_STREAM {
		return Client{}, ErrInvalid
	}
	deadline := time.Now().Add(NativeBootstrapTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if conn.SetReadDeadline(deadline) != nil {
		return Client{}, ErrInvalid
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetReadDeadline(time.Now()) })
	defer stop()
	var payload json.RawMessage
	if ReadFrame(conn, &payload) != nil {
		return Client{}, ErrInvalid
	}
	defer clear(payload)
	var tail [1]byte
	if n, e := conn.Read(tail[:]); n != 0 || e != io.EOF {
		return Client{}, ErrInvalid
	}
	expected, own, e := decodeNativeBootstrap(payload, descriptor, recipient)
	if e != nil {
		return Client{}, ErrInvalid
	}
	var d DescriptorV1
	if Decode(descriptor, &d) != nil {
		return Client{}, ErrInvalid
	}
	protected, e := LoadValidatedPeerBinding(d, expected)
	if e != nil {
		return Client{}, ErrInvalid
	}
	if protected.record.ConsumerExpectedKernelPeer.Kernel() != recipient {
		return Client{}, ErrInvalid
	}
	return NewReadbackClient(d, protected, own, timeout)
}
func privateBootstrapDriver(conn *net.UnixConn) error {
	peer, e := PeerCredentials(conn)
	if e != nil || peer.PID <= 0 || peer.UID != 0 {
		return ErrInvalid
	}
	return nil
}
