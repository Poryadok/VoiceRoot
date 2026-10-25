//go:build !linux

package mmadmissionwire

import "net"

func PeerCredentials(*net.UnixConn) (KernelPeer, error) { return KernelPeer{}, ErrInvalid }

func Peer(*net.UnixConn) (PeerIdentity, error)    { return PeerIdentity{}, ErrInvalid }
func CheckPeer(*net.UnixConn, PeerIdentity) error { return ErrInvalid }
