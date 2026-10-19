//go:build linux

package mmadmissionwire

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"syscall"
)

func Peer(conn *net.UnixConn) (PeerIdentity, error) {
	var out PeerIdentity
	raw, e := conn.SyscallConn()
	if e != nil {
		return out, ErrInvalid
	}
	var cred *syscall.Ucred
	var inner error
	if e = raw.Control(func(fd uintptr) {
		cred, inner = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); e != nil || inner != nil || cred == nil {
		return out, ErrInvalid
	}
	out.PID = cred.Pid
	out.UID = cred.Uid
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/exe", cred.Pid))
	if e != nil {
		return out, ErrInvalid
	}
	out.ExecutableHash = sha256.Sum256(b)
	return out, nil
}
func CheckPeer(conn *net.UnixConn, want PeerIdentity) error {
	got, e := Peer(conn)
	if e != nil || got != want {
		return ErrInvalid
	}
	return nil
}
