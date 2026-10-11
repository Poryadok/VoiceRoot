package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestFileColdStartSubprocess(t *testing.T) {
	if os.Getenv("VOICE_FILE_COLD_START_CHILD") != "1" {
		return
	}
	main()
}

type startupOutput struct {
	sync.Mutex
	bytes.Buffer
}

func (b *startupOutput) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(p)
}

func (b *startupOutput) text() string {
	b.Lock()
	defer b.Unlock()
	return b.String()
}

type observedListener struct {
	net.Listener
	accepted chan struct{}
	once     sync.Once
}

func (l *observedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.once.Do(func() { close(l.accepted) })
	}
	return conn, err
}

func TestFileColdStartBeforeMessagingAndReconnect(t *testing.T) {
	// Run the actual startup wiring, rather than a replacement dial helper.
	address := func() string {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := listener.Addr().String()
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		return addr
	}
	httpAddr, fileAddr, messagingAddr := address(), address(), address()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestFileColdStartSubprocess$")
	cmd.Env = []string{
		"VOICE_FILE_COLD_START_CHILD=1", "LISTEN_ADDR=" + httpAddr,
		"FILE_GRPC_LISTEN=" + fileAddr, "MESSAGING_GRPC_ADDR=" + messagingAddr,
		"GRPC_DIAL_TIMEOUT=200ms", "SHUTDOWN_TIMEOUT=1s",
		"DATABASE_URL=postgres://fixture:fixture@127.0.0.1:1/fixture?sslmode=disable",
	}
	output := &startupOutput{}
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})
	client := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, err := client.Get("http://" + httpAddr + "/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("File health status %d", response.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("File listener stayed blocked by absent Messaging: %s", output.text())
		}
		time.Sleep(20 * time.Millisecond)
	}
	conn, err := net.DialTimeout("tcp", fileAddr, time.Second)
	if err != nil {
		t.Fatalf("File gRPC listener unavailable: %v", err)
	}
	_ = conn.Close()
	listener, err := net.Listen("tcp", messagingAddr)
	if err != nil {
		t.Fatal(err)
	}
	peer := &observedListener{Listener: listener, accepted: make(chan struct{})}
	server := grpc.NewServer()
	t.Cleanup(server.Stop)
	go func() { _ = server.Serve(peer) }()
	select {
	case <-peer.accepted:
	case <-time.After(4 * time.Second):
		t.Fatalf("File did not reconnect to later Messaging peer: %s", output.text())
	}
}
