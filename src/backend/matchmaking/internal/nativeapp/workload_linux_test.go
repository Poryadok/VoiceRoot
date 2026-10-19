//go:build linux

package nativeapp

import (
	"context"
	"github.com/stretchr/testify/require"
	"net"
	"os"
	"path/filepath"
	"testing"
	w "voice/backend/pkg/mmadmissionwire"
)

func TestActualUnixWorkloadControlUsesKernelPeerAndClosesOwnedSocket(t *testing.T) {
	c, _, r := workloadFixture(t)
	c.peer = workloadPeerFixture{w.KernelPeer{PID: int32(os.Getpid()), UID: uint32(os.Getuid()), GID: uint32(os.Getgid())}}
	path := filepath.Join(t.TempDir(), "control.sock")
	server, err := listenWorkload(context.Background(), path, c)
	require.NoError(t, err)
	t.Cleanup(server.Close)
	exchange := func(request w.WorkloadRequestV1) (w.WorkloadResponseV1, error) {
		conn, e := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
		if e != nil {
			return w.WorkloadResponseV1{}, e
		}
		defer conn.Close()
		if e = w.WriteFrame(conn, request); e != nil {
			return w.WorkloadResponseV1{}, e
		}
		if e = conn.CloseWrite(); e != nil {
			return w.WorkloadResponseV1{}, e
		}
		var response w.WorkloadResponseV1
		e = w.ReadFrame(conn, &response)
		return response, e
	}
	response, err := exchange(r)
	require.NoError(t, err)
	require.Equal(t, w.Armed, response.State)
	c.peer = workloadPeerFixture{w.KernelPeer{PID: int32(os.Getpid() + 1), UID: uint32(os.Getuid()), GID: uint32(os.Getgid())}}
	_, err = exchange(r)
	require.Error(t, err)
	// Do not replace an existing control endpoint, even if its pathname is known.
	_, err = listenWorkload(context.Background(), path, c)
	require.Error(t, err)
	server.Close()
	_, err = os.Lstat(path)
	require.True(t, os.IsNotExist(err))
}
