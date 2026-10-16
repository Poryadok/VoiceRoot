package main

import (
	jwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"io"
	"os"
	"time"
)

// Scratch broker CONNECT and PING/PONG only: no PUB, SUB, ACK, or JS API.
func authenticateExistingActor() (result error) {
	const path = "/inputs/actor.creds"
	before, e := os.Lstat(path)
	if e != nil || !before.Mode().IsRegular() || before.Mode().Perm()&0022 != 0 || before.Size() <= 0 || before.Size() > 262144 {
		return renewalFailure()
	}
	file, e := os.Open(path)
	if e != nil {
		return renewalFailure()
	}
	defer func() {
		if err := file.Close(); err != nil {
			result = renewalFailure()
		}
	}()
	after, e := file.Stat()
	if e != nil || !os.SameFile(before, after) {
		return renewalFailure()
	}
	raw, e := io.ReadAll(io.LimitReader(file, 262145))
	if e != nil || len(raw) > 262144 {
		return renewalFailure()
	}
	defer func() {
		for i := range raw {
			raw[i] = 0
		}
	}()
	token, e := jwt.ParseDecoratedJWT(raw)
	if e != nil {
		return renewalFailure()
	}
	key, e := jwt.ParseDecoratedUserNKey(raw)
	if e != nil {
		return renewalFailure()
	}
	defer key.Wipe()
	conn, e := nats.Connect("nats://127.0.0.1:4222", nats.UserJWT(func() (string, error) { return token, nil }, key.Sign),
		nats.Timeout(5*time.Second), nats.NoReconnect())
	if e != nil {
		return renewalFailure()
	}
	defer conn.Close()
	if conn.ConnectedServerVersion() != "2.12.12" || conn.FlushTimeout(5*time.Second) != nil {
		return renewalFailure()
	}
	return nil
}
