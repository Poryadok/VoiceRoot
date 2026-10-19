//go:build linux

package nativeapp

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	w "voice/backend/pkg/mmadmissionwire"
)

type workloadServer struct {
	listener *net.UnixListener
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	once     sync.Once
}

func startWorkloadServer(parent context.Context, control *workloadControl) (*workloadServer, error) {
	if control == nil {
		return nil, errUnavailable
	}
	info, err := os.Lstat(w.WorkloadSocketDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0002 != 0 {
		return nil, errUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return nil, errUnavailable
	}
	// The installer owns the parent directory. Never remove an existing socket
	// or create a replacement authority/control directory on startup.
	return listenWorkload(parent, filepath.Join(w.WorkloadSocketDir, control.descriptor.ReplicaID.String()+".sock"), control)
}

func listenWorkload(parent context.Context, path string, control *workloadControl) (*workloadServer, error) {
	if parent == nil || parent.Err() != nil || control == nil {
		return nil, errUnavailable
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return nil, errUnavailable
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, errUnavailable
	}
	listener.SetUnlinkOnClose(true)
	if os.Chmod(path, 0600) != nil {
		_ = listener.Close()
		return nil, errUnavailable
	}
	ctx, cancel := context.WithCancel(parent)
	server := &workloadServer{listener: listener, cancel: cancel}
	slots := make(chan struct{}, 16)
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		for {
			conn, err := listener.AcceptUnix()
			if err != nil {
				return
			}
			select {
			case slots <- struct{}{}:
			default:
				_ = conn.Close()
				continue
			}
			server.wg.Add(1)
			go func() {
				defer server.wg.Done()
				defer func() { <-slots }()
				defer conn.Close()
				done := context.AfterFunc(ctx, func() { _ = conn.Close() })
				defer done()
				_ = conn.SetDeadline(time.Now().Add(12 * time.Second))
				peer, err := w.PeerCredentials(conn)
				if err != nil {
					return
				}
				var request w.WorkloadRequestV1
				if w.ReadFrame(conn, &request) != nil {
					return
				}
				var extra [1]byte
				if n, e := conn.Read(extra[:]); n != 0 || e != io.EOF {
					return
				}
				requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				response, err := control.handle(requestCtx, peer, request)
				if err != nil {
					return
				}
				_ = w.WriteFrame(conn, response)
			}()
		}
	}()
	return server, nil
}

func (s *workloadServer) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() { s.cancel(); _ = s.listener.Close(); s.wg.Wait() })
}
