package clamav

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScanner_ScanBytes_cleanAndInfected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4096)
		for {
			n, readErr := conn.Read(buf)
			if readErr != nil || n == 0 {
				break
			}
			if n >= 4 && buf[n-4] == 0 && buf[n-3] == 0 && buf[n-2] == 0 && buf[n-1] == 0 {
				break
			}
		}
		_, _ = conn.Write([]byte("stream: OK\n"))
	}()

	scanner := Scanner{Addr: ln.Addr().String(), Timeout: time.Second}
	outcome, err := scanner.ScanBytes(context.Background(), []byte("clean-payload"))
	require.NoError(t, err)
	require.Equal(t, "clean", outcome)
}

func TestScanner_ScanBytes_skipsWhenAddrEmpty(t *testing.T) {
	scanner := Scanner{}
	outcome, err := scanner.ScanBytes(context.Background(), []byte("x"))
	require.NoError(t, err)
	require.Equal(t, "skipped", outcome)
}

func TestScanner_ScanBytes_reportsInfected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4096)
		for {
			n, readErr := conn.Read(buf)
			if readErr != nil || n == 0 {
				break
			}
			if n >= 4 && buf[n-4] == 0 && buf[n-3] == 0 && buf[n-2] == 0 && buf[n-1] == 0 {
				break
			}
		}
		_, _ = conn.Write([]byte("stream: Eicar-Test-Signature FOUND\n"))
	}()

	scanner := Scanner{Addr: ln.Addr().String(), Timeout: time.Second}
	outcome, err := scanner.ScanBytes(context.Background(), []byte("bad"))
	require.NoError(t, err)
	require.Equal(t, "infected", outcome)
}

func TestScanner_ScanBytes_unexpectedResponse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4096)
		for {
			n, readErr := conn.Read(buf)
			if readErr != nil || n == 0 {
				break
			}
			if n >= 4 && buf[n-4] == 0 && buf[n-3] == 0 && buf[n-2] == 0 && buf[n-1] == 0 {
				break
			}
		}
		_, _ = conn.Write([]byte("stream: weird\n"))
	}()

	scanner := Scanner{Addr: ln.Addr().String()}
	_, err = scanner.ScanBytes(context.Background(), []byte("x"))
	require.Error(t, err)
}

func TestScanner_ScanBytes_dialFailure(t *testing.T) {
	scanner := Scanner{Addr: "127.0.0.1:1", Timeout: time.Millisecond}
	_, err := scanner.ScanBytes(context.Background(), []byte("x"))
	require.Error(t, err)
}
