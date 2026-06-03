package clamav

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

type Scanner struct {
	Addr    string
	Timeout time.Duration
}

func (s Scanner) ScanBytes(ctx context.Context, data []byte) (string, error) {
	addr := strings.TrimSpace(s.Addr)
	if addr == "" {
		return "skipped", nil
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "error", err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return "error", err
	}
	for len(data) > 0 {
		n := len(data)
		if n > 1024*1024 {
			n = 1024 * 1024
		}
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(n))
		if _, err := conn.Write(lenBuf[:]); err != nil {
			return "error", err
		}
		if _, err := conn.Write(data[:n]); err != nil {
			return "error", err
		}
		data = data[n:]
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return "error", err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "error", err
	}
	resp := string(bytes.TrimSpace(buf[:n]))
	switch {
	case strings.Contains(resp, "FOUND"):
		return "infected", nil
	case strings.Contains(resp, "OK"):
		return "clean", nil
	default:
		return "error", fmt.Errorf("clamav unexpected response: %s", resp)
	}
}
