//go:build !linux

package mmadmissionwire

import "net"

func Peer(*net.UnixConn) (PeerIdentity, error)    { return PeerIdentity{}, ErrInvalid }
func CheckPeer(*net.UnixConn, PeerIdentity) error { return ErrInvalid }
