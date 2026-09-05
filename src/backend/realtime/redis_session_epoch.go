package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const sessionEpochFloorPrefix = "auth:session:min_epoch:"

type sessionEpochFloor interface {
	Minimum(context.Context, string) (int64, error)
}

type redisSessionEpochFloor struct {
	addr     string
	password string
	prefix   string
	timeout  time.Duration
}

func newRedisSessionEpochFloor(addr, password string) *redisSessionEpochFloor {
	return &redisSessionEpochFloor{
		addr:     addr,
		password: password,
		prefix:   sessionEpochFloorPrefix,
		timeout:  2 * time.Second,
	}
}

func (f *redisSessionEpochFloor) Minimum(ctx context.Context, accountID string) (int64, error) {
	if accountID == "" || strings.TrimSpace(accountID) != accountID {
		return 0, errors.New("missing session epoch account id")
	}
	raw, err := realtimeRedisGet(ctx, f.addr, f.password, f.timeout, f.prefix+accountID)
	if err != nil {
		return 0, err
	}
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || epoch <= 0 {
		if err == nil {
			err = errors.New("session epoch floor must be positive")
		}
		return 0, err
	}
	return epoch, nil
}

func realtimeRedisGet(ctx context.Context, addr, password string, timeout time.Duration, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	deadline := time.Now().Add(timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return "", err
	}

	stopCancellation := make(chan struct{})
	cancellationStopped := make(chan struct{})
	go func() {
		defer close(cancellationStopped)
		select {
		case <-ctx.Done():
			_ = conn.SetDeadline(time.Now())
		case <-stopCancellation:
		}
	}()
	defer func() {
		close(stopCancellation)
		<-cancellationStopped
	}()

	reader := bufio.NewReader(conn)
	if password != "" {
		if _, err := realtimeRedisCommand(conn, reader, "AUTH", password); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", ctxErr
			}
			return "", err
		}
	}
	value, err := realtimeRedisBulkCommand(conn, reader, "GET", key)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	return value, err
}

func realtimeRedisCommand(conn net.Conn, reader *bufio.Reader, args ...string) (string, error) {
	if err := realtimeRedisWriteCommand(conn, args...); err != nil {
		return "", err
	}
	prefix, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	line, err := realtimeRedisReadLine(reader)
	if err != nil {
		return "", err
	}
	if prefix == '-' {
		return "", errors.New(line)
	}
	if prefix != '+' {
		return "", errors.New("unexpected redis response")
	}
	return line, nil
}

func realtimeRedisBulkCommand(conn net.Conn, reader *bufio.Reader, args ...string) (string, error) {
	if err := realtimeRedisWriteCommand(conn, args...); err != nil {
		return "", err
	}
	prefix, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	line, err := realtimeRedisReadLine(reader)
	if err != nil {
		return "", err
	}
	switch prefix {
	case '-':
		return "", errors.New(line)
	case '$':
		length, err := strconv.Atoi(line)
		if err != nil || length < 0 {
			if err == nil {
				err = errors.New("redis nil value")
			}
			return "", err
		}
		if length > int(^uint(0)>>1)-2 {
			return "", errors.New("redis bulk response too large")
		}
		value := make([]byte, length+2)
		if _, err := io.ReadFull(reader, value); err != nil {
			return "", err
		}
		if value[length] != '\r' || value[length+1] != '\n' {
			return "", errors.New("invalid redis bulk response")
		}
		return string(value[:length]), nil
	default:
		return "", errors.New("unexpected redis response")
	}
}

func realtimeRedisWriteCommand(conn net.Conn, args ...string) error {
	var request strings.Builder
	request.WriteByte('*')
	request.WriteString(strconv.Itoa(len(args)))
	request.WriteString("\r\n")
	for _, arg := range args {
		request.WriteByte('$')
		request.WriteString(strconv.Itoa(len(arg)))
		request.WriteString("\r\n")
		request.WriteString(arg)
		request.WriteString("\r\n")
	}
	if _, err := conn.Write([]byte(request.String())); err != nil {
		return err
	}
	return nil
}

func realtimeRedisReadLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}
