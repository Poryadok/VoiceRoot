//go:build role_bot_terminal

package main

import (
	"bytes"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

type terminalComplete struct {
	SchemaVersion uint32 `json:"schema_version"`
	BotID         string `json:"bot_id"`
	DecisionID    string `json:"decision_id"`
	ReceiptID     string `json:"receipt_id"`
}

func decodeTerminalComplete(raw []byte) (terminalComplete, error) {
	var c terminalComplete
	if len(raw) > 4096 {
		return c, errors.New("terminal completion exceeds bound")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return c, errors.New("terminal completion has trailing input")
	}
	if c.SchemaVersion != 1 {
		return c, errors.New("unknown completion version")
	}
	for _, raw := range []string{c.BotID, c.DecisionID, c.ReceiptID} {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || id.String() != raw {
			return c, errors.New("noncanonical completion identity")
		}
	}
	return c, nil
}

func TestTerminalCompleteStrictBoundary(t *testing.T) {
	c := terminalComplete{1, uuid.NewString(), uuid.NewString(), uuid.NewString()}
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	got, err := decodeTerminalComplete(raw)
	require.NoError(t, err)
	require.Equal(t, c, got)
	for _, raw := range [][]byte{nil, []byte("null"), []byte(`{}`), append(append([]byte{}, raw...), []byte(` {}`)...), []byte(strings.Replace(string(raw), `"schema_version":1`, `"schema_version":2`, 1)), []byte(strings.Replace(string(raw), `"schema_version":1`, `"extra":true,"schema_version":1`, 1)), []byte(strings.Replace(string(raw), c.BotID, uuid.Nil.String(), 1)), []byte(strings.Replace(string(raw), c.ReceiptID, strings.ToUpper(c.ReceiptID), 1))} {
		_, err := decodeTerminalComplete(raw)
		require.Error(t, err)
	}
}

func terminalAtomicJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(path+".tmp", raw, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func (p *terminalProvider) hold(selfReq *rolev1.RegisterBotGlobalTerminalRequest, selfResult *rolev1.RegisterBotGlobalTerminalResponse) {
	dir := os.Getenv("VOICE_BOT_ROLE26_HANDOFF_DIR")
	if dir == "" {
		return
	}
	t := p.t
	require.True(t, filepath.IsAbs(dir))
	pin := os.Getenv("VOICE_BOT_ROLE26_SOURCE_PIN")
	_, err := hex.DecodeString(pin)
	require.NoError(t, err)
	require.Len(t, pin, 40)
	require.Equal(t, strings.ToLower(pin), pin)
	require.NoError(t, os.MkdirAll(filepath.Dir(dir), 0700))
	require.NoError(t, os.Mkdir(dir, 0700), "fresh consumer-owned handoff directory required")
	ca, cert, key, signer := filepath.Join(dir, "role-ca.pem"), filepath.Join(dir, "role-client.pem"), filepath.Join(dir, "role-client-key.pem"), filepath.Join(dir, "bot-test-key.pem")
	der, err := x509.MarshalPKCS8PrivateKey(p.key)
	require.NoError(t, err)
	files := map[string][]byte{ca: p.ca, cert: p.clientCert, key: p.clientKey, signer: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})}
	ready, stop, complete, verified := filepath.Join(dir, "ready.json"), filepath.Join(dir, "stop"), filepath.Join(dir, "complete.json"), filepath.Join(dir, "verified.json")
	defer func() {
		for _, path := range []string{ready, ready + ".tmp", ca, cert, key, signer, complete, complete + ".tmp", verified, verified + ".tmp", stop} {
			_ = os.Remove(path)
		}
	}()
	for path, raw := range files {
		require.NoError(t, os.WriteFile(path, raw, 0600))
	}
	expires := time.Now().Add(5 * time.Minute).UTC()
	if deadline, ok := p.ctx.Deadline(); ok && deadline.Before(expires) {
		expires = deadline
	}
	require.NoError(t, terminalAtomicJSON(ready, map[string]any{"schema_version": 1, "catalog_version": 26, "fixture_only": true, "address": p.address, "server_name": "127.0.0.1", "ca_file": ca, "client_cert_file": cert, "client_key_file": key, "signer_key_file": signer, "key_id": "current", "source_pin": pin, "expires_at": expires, "stop_file": stop}))
	selftest := os.Getenv("VOICE_BOT_ROLE26_SELFTEST") == "1"
	if selftest {
		require.NoError(t, terminalAtomicJSON(complete, terminalComplete{1, selfReq.BotId, selfReq.DecisionId, selfResult.Receipt.ReceiptId}))
	}
	t.Logf("private Role26 handoff ready: %s", ready)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	validated := false
	for {
		select {
		case <-p.ctx.Done():
			t.Fatal("Role26 handoff context expired")
		case <-ticker.C:
			if time.Now().After(expires) {
				t.Fatal("Role26 handoff five-minute lifetime expired")
			}
			if !validated {
				raw, err := os.ReadFile(complete)
				if err == nil {
					c, err := decodeTerminalComplete(raw)
					require.NoError(t, err)
					req := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: c.BotID, DecisionId: c.DecisionID}
					// complete.json synchronizes observation; it never authorizes an
					// INSERT. Require the production receiver's preexisting exact row.
					var saved string
					require.NoError(t, p.admin.QueryRow(p.ctx, "SELECT receipt_id::text FROM role_bot_global_terminals WHERE bot_id=$1 AND decision_id=$2", c.BotID, c.DecisionID).Scan(&saved))
					require.Equal(t, c.ReceiptID, saved)
					first := p.register(req)
					require.Equal(t, c.ReceiptID, first.Receipt.ReceiptId)
					require.True(t, proto.Equal(first, p.register(req)), "consumer's committed root replays own stable typed receipt")
					p.assertFence(c.BotID)
					p.assertClosedWriters(c.BotID)
					require.NoError(t, terminalAtomicJSON(verified, map[string]any{"schema_version": 1, "bot_id": c.BotID, "decision_id": c.DecisionID, "receipt_id": c.ReceiptID, "own_row_count": 1, "stable_typed_replay": true, "local_bot_fence_denied": true}))
					validated = true
					if selftest {
						require.NoError(t, os.WriteFile(stop, []byte("stop\n"), 0600))
					}
				} else {
					require.True(t, os.IsNotExist(err), fmt.Sprintf("cannot read owned completion: %T", err))
				}
			}
			_, err := os.Stat(stop)
			if err == nil {
				require.True(t, validated, "stop requires own database postcondition")
				return
			}
			require.True(t, os.IsNotExist(err))
		}
	}
}
