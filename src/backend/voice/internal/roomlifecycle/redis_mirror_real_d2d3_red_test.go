//go:build r223_d2d3_red

package roomlifecycle

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func r223Operation(t *testing.T, method LifecycleMethod, vectorIndex int, completed bool) LifecycleOperation {
	t.Helper()
	vector := r223ReceiptVectors(t)[vectorIndex]
	receiptBytes, receiptHash, err := EncodeLifecycleReceipt(vector.receipt)
	require.NoError(t, err)
	replayUntil := r223FixedTime.Add(24 * time.Hour)
	completedAt := replayUntil.Add(-24 * time.Hour)
	operation := LifecycleOperation{
		ActorProfileID: vector.receipt.ActorProfileID, OperationID: vector.receipt.OperationID,
		SubjectProfileID: vector.receipt.SubjectProfileID, SpaceID: vector.receipt.SpaceID,
		Method: method, Fingerprint: r223Digest(0x20), BindingBytes: []byte("r22.2-immutable-binding"),
		RedisOwnerToken: r223Digest(0x40), State: LifecycleOperationDecided,
		DecidedAt: completedAt.Add(-time.Minute), CreatedAt: completedAt.Add(-time.Minute), UpdatedAt: completedAt.Add(-time.Minute),
	}
	if completed {
		operation.State = LifecycleOperationCompleted
		operation.Receipt = &vector.receipt
		operation.ReceiptBytes = append([]byte(nil), receiptBytes...)
		operation.ReceiptHash = &receiptHash
		operation.CompletedAt = &completedAt
		operation.ReplayUntil = &replayUntil
	}
	return operation
}

func r223NewReservation(t *testing.T, origin string, operation LifecycleOperation) RedisMirrorReservation {
	t.Helper()
	reservation, err := NewRedisMirrorReservation(origin, operation)
	require.NoError(t, err)
	return reservation
}

func r223NewReceipt(t *testing.T, operation LifecycleOperation) RedisMirrorReceipt {
	t.Helper()
	receipt, err := NewRedisMirrorReceipt(operation)
	require.NoError(t, err)
	return receipt
}

func r223KeyText(key RedisMirrorKey) string {
	return fmt.Sprintf("voice:lifecycle:v2:op:%s:%s:%s", key.Origin, key.ActorProfileID, key.OperationID)
}

func r223ReservationFields(reservation RedisMirrorReservation) map[string]string {
	return map[string]string{
		"schema_version": "2", "state": "pending", "method": r223MethodName(reservation.Binding.Method),
		"fingerprint": hex.EncodeToString(reservation.Binding.Fingerprint[:]),
		"owner_token": hex.EncodeToString(reservation.OwnerToken[:]),
	}
}

func r223CompletedFields(reservation RedisMirrorReservation, receipt RedisMirrorReceipt) map[string]string {
	fields := r223ReservationFields(reservation)
	fields["state"] = "completed"
	fields["receipt_bytes"] = string(receipt.Bytes)
	fields["receipt_hash"] = hex.EncodeToString(receipt.Hash[:])
	fields["replay_until"] = fmt.Sprint(receipt.ReplayUntil.UnixMilli())
	return fields
}

func r223RequirePendingObservation(t *testing.T, observation RedisMirrorObservation, reservation RedisMirrorReservation) {
	t.Helper()
	require.Equal(t, RedisMirrorPending, observation.State)
	require.Equal(t, reservation.Binding, observation.Binding, "pending observation must return the full key/method/fingerprint binding")
	require.Equal(t, reservation.OwnerToken, observation.OwnerToken)
	require.Nil(t, observation.Receipt)
}

func r223RequireCompletedObservation(t *testing.T, observation RedisMirrorObservation, reservation RedisMirrorReservation, receipt RedisMirrorReceipt) {
	t.Helper()
	require.Equal(t, RedisMirrorCompleted, observation.State)
	require.Equal(t, reservation.Binding, observation.Binding, "completed observation must return the full key/method/fingerprint binding")
	require.Equal(t, reservation.OwnerToken, observation.OwnerToken)
	require.NotNil(t, observation.Receipt)
	require.Equal(t, receipt.Bytes, observation.Receipt.Bytes)
	require.Equal(t, receipt.Hash, observation.Receipt.Hash)
	require.Equal(t, receipt.ReplayUntil.UTC(), observation.Receipt.ReplayUntil.UTC())
}

func r223RequireDiverged(t *testing.T, err error, class string) {
	t.Helper()
	require.True(t, errors.Is(err, ErrRedisMirrorDiverged), "must wrap ErrRedisMirrorDiverged: %v", err)
	var divergence *RedisMirrorDivergenceError
	require.True(t, errors.As(err, &divergence), "must expose RedisMirrorDivergenceError: %v", err)
	require.Equal(t, class, string(divergence.Class))
}

func TestRedisMirrorV2_ProductionConstructorsValidateRehashAndCopy(t *testing.T) {
	operation := r223Operation(t, LifecycleMethodJoin, 0, true)
	originalBytes := append([]byte(nil), operation.ReceiptBytes...)
	reservation := r223NewReservation(t, "delegated_user", operation)
	receipt := r223NewReceipt(t, operation)
	require.Equal(t, operation.ActorProfileID, reservation.Binding.Key.ActorProfileID)
	require.Equal(t, operation.OperationID, reservation.Binding.Key.OperationID)
	require.Equal(t, "delegated_user", reservation.Binding.Key.Origin)
	recomputed := sha256.Sum256(originalBytes)
	require.Equal(t, LifecycleDigest(recomputed), receipt.Hash)
	operation.ReceiptBytes[0] ^= 0xff
	require.Equal(t, originalBytes, receipt.Bytes, "receipt constructor must copy PostgreSQL bytes")

	invalid := operation
	invalid.ActorProfileID = uuid.Nil
	_, err := NewRedisMirrorReservation("delegated_user", invalid)
	require.Error(t, err)
	invalid = operation
	invalid.OperationID = uuid.Nil
	_, err = NewRedisMirrorReservation("delegated_user", invalid)
	require.Error(t, err)
	invalid = operation
	invalid.Method = 0
	_, err = NewRedisMirrorReservation("delegated_user", invalid)
	require.Error(t, err)
	_, err = NewRedisMirrorReservation("client", operation)
	require.Error(t, err)
	invalid = operation
	wrongHash := r223Digest(0x77)
	invalid.ReceiptHash = &wrongHash
	_, err = NewRedisMirrorReceipt(invalid)
	require.Error(t, err, "constructor must recompute and reject a stored hash mismatch")
	invalid = operation
	invalid.ReplayUntil = nil
	_, err = NewRedisMirrorReceipt(invalid)
	require.Error(t, err)
	invalid = operation
	nonMillisecond := invalid.ReplayUntil.Add(time.Nanosecond)
	invalid.ReplayUntil = &nonMillisecond
	_, err = NewRedisMirrorReceipt(invalid)
	require.Error(t, err)

	client, ledger := r223MiniredisLedger(t)
	observation, err := ledger.CompleteMirror(context.Background(), reservation, receipt)
	require.NoError(t, err)
	receipt.Bytes[0] ^= 0xff
	require.Equal(t, string(originalBytes), client.HGet(context.Background(), r223KeyText(reservation.Binding.Key), "receipt_bytes").Val())
	observation.Receipt.Bytes[0] ^= 0xff
	again, err := ledger.InspectMirror(context.Background(), reservation.Binding.Key)
	require.NoError(t, err)
	require.Equal(t, originalBytes, again.Receipt.Bytes, "observations must return defensive copies")
}

func TestRedisMirrorV2_LiteralKeysHashesAndReceiptShapesThroughDirectAPI(t *testing.T) {
	methods := []struct {
		method LifecycleMethod
		vector int
	}{{LifecycleMethodJoin, 0}, {LifecycleMethodLeave, 2}, {LifecycleMethodSelfMove, 4}, {LifecycleMethodModeratorMove, 5}}
	for _, item := range methods {
		t.Run(r223MethodName(item.method), func(t *testing.T) {
			client, ledger := r223MiniredisLedger(t)
			operation := r223Operation(t, item.method, item.vector, false)
			reservation := r223NewReservation(t, "delegated_user", operation)
			observation, err := ledger.AttachPending(context.Background(), reservation)
			require.NoError(t, err)
			r223RequirePendingObservation(t, observation, reservation)
			inspected, err := ledger.InspectMirror(context.Background(), reservation.Binding.Key)
			require.NoError(t, err)
			r223RequirePendingObservation(t, inspected, reservation)
			key := r223KeyText(reservation.Binding.Key)
			require.Equal(t, r223ReservationFields(reservation), client.HGetAll(context.Background(), key).Val())
			require.EqualValues(t, 5, client.HLen(context.Background(), key).Val())
			require.Equal(t, time.Duration(-1), client.PTTL(context.Background(), key).Val())
		})
	}
	for index, vector := range r223ReceiptVectors(t) {
		t.Run(vector.name, func(t *testing.T) {
			client, ledger := r223MiniredisLedger(t)
			operation := r223Operation(t, vector.receipt.Method, index, true)
			reservation := r223NewReservation(t, "delegated_user", operation)
			receipt := r223NewReceipt(t, operation)
			observation, err := ledger.CompleteMirror(context.Background(), reservation, receipt)
			require.NoError(t, err)
			r223RequireCompletedObservation(t, observation, reservation, receipt)
			inspected, err := ledger.InspectMirror(context.Background(), reservation.Binding.Key)
			require.NoError(t, err)
			r223RequireCompletedObservation(t, inspected, reservation, receipt)
			key := r223KeyText(reservation.Binding.Key)
			require.Equal(t, r223CompletedFields(reservation, receipt), client.HGetAll(context.Background(), key).Val())
			require.EqualValues(t, 8, client.HLen(context.Background(), key).Val())
		})
	}
	client, ledger := r223MiniredisLedger(t)
	operation := r223Operation(t, LifecycleMethodLeave, 2, false)
	delegated := r223NewReservation(t, "delegated_user", operation)
	system := r223NewReservation(t, "system_disconnect", operation)
	_, err := ledger.AttachPending(context.Background(), delegated)
	require.NoError(t, err)
	_, err = ledger.AttachPending(context.Background(), system)
	require.NoError(t, err)
	require.EqualValues(t, 2, client.DBSize(context.Background()).Val(), "equal UUIDs in closed origins must create two keys")
}

type r223MalformedSeed struct {
	fields map[string]string
	expiry *time.Time
	class  string
}

func TestRedisMirrorV2_StrictMalformedMatrixDirectErrorsAndImmutability(t *testing.T) {
	operation := r223Operation(t, LifecycleMethodJoin, 0, true)
	reservation := r223NewReservation(t, "delegated_user", operation)
	receipt := r223NewReceipt(t, operation)
	valid := r223CompletedFields(reservation, receipt)
	expiry := receipt.ReplayUntil
	wrongExpiry := expiry.Add(time.Second)
	malformedWire := cloneStringMap(valid)
	malformedWire["receipt_bytes"] = "\xff"
	malformedHash := sha256.Sum256([]byte(malformedWire["receipt_bytes"]))
	malformedWire["receipt_hash"] = hex.EncodeToString(malformedHash[:])
	cases := map[string]r223MalformedSeed{
		"legacy_v1":    {fields: map[string]string{"state": "pending", "method": "JOIN", "fingerprint": "sha256:" + strings.Repeat("1", 64), "owner": strings.Repeat("2", 64)}, class: "legacy_schema"},
		"hlen_missing": {fields: func() map[string]string { v := cloneStringMap(valid); delete(v, "owner_token"); return v }(), expiry: &expiry, class: "malformed"},
		"hlen_extra":   {fields: func() map[string]string { v := cloneStringMap(valid); v["extra"] = "x"; return v }(), expiry: &expiry, class: "malformed"},
		"schema":       {fields: func() map[string]string { v := cloneStringMap(valid); v["schema_version"] = "02"; return v }(), expiry: &expiry, class: "malformed"},
		"state":        {fields: func() map[string]string { v := cloneStringMap(valid); v["state"] = "ready"; return v }(), expiry: &expiry, class: "malformed"},
		"method":       {fields: func() map[string]string { v := cloneStringMap(valid); v["method"] = "JOIN"; return v }(), expiry: &expiry, class: "malformed"},
		"fingerprint": {fields: func() map[string]string {
			v := cloneStringMap(valid)
			v["fingerprint"] = strings.Repeat("g", 64)
			return v
		}(), expiry: &expiry, class: "malformed"},
		"owner": {fields: func() map[string]string {
			v := cloneStringMap(valid)
			v["owner_token"] = strings.Repeat("A", 64)
			return v
		}(), expiry: &expiry, class: "malformed"},
		"receipt_decode": {fields: malformedWire, expiry: &expiry, class: "malformed"},
		"receipt_hash_encoding": {fields: func() map[string]string {
			v := cloneStringMap(valid)
			v["receipt_hash"] = strings.Repeat("g", 64)
			return v
		}(), expiry: &expiry, class: "malformed"},
		"receipt_hash_mismatch": {fields: func() map[string]string {
			v := cloneStringMap(valid)
			v["receipt_hash"] = strings.Repeat("0", 64)
			return v
		}(), expiry: &expiry, class: "malformed"},
		"deadline_zero":       {fields: func() map[string]string { v := cloneStringMap(valid); v["replay_until"] = "0"; return v }(), expiry: &expiry, class: "malformed"},
		"deadline_nondecimal": {fields: func() map[string]string { v := cloneStringMap(valid); v["replay_until"] = "1e3"; return v }(), expiry: &expiry, class: "malformed"},
		"deadline_overflow": {fields: func() map[string]string {
			v := cloneStringMap(valid)
			v["replay_until"] = "9223372036854775808"
			return v
		}(), expiry: &expiry, class: "malformed"},
		"deadline_leading_zero": {fields: func() map[string]string {
			v := cloneStringMap(valid)
			v["replay_until"] = "0" + v["replay_until"]
			return v
		}(), expiry: &expiry, class: "malformed"},
		"completed_no_expiry":    {fields: valid, class: "malformed"},
		"completed_wrong_expiry": {fields: valid, expiry: &wrongExpiry, class: "deadline_mismatch"},
		"pending_with_expiry":    {fields: r223ReservationFields(reservation), expiry: &wrongExpiry, class: "malformed"},
	}
	for name, seed := range cases {
		for _, call := range []string{"inspect", "attach", "complete"} {
			t.Run(name+"/"+call, func(t *testing.T) {
				client, ledger := r223MiniredisLedger(t)
				key := r223KeyText(reservation.Binding.Key)
				require.NoError(t, client.HSet(context.Background(), key, seed.fields).Err())
				if seed.expiry != nil {
					require.NoError(t, client.PExpireAt(context.Background(), key, *seed.expiry).Err())
				}
				beforeFields := client.HGetAll(context.Background(), key).Val()
				beforeExpiry := client.PExpireTime(context.Background(), key).Val()
				var err error
				switch call {
				case "inspect":
					_, err = ledger.InspectMirror(context.Background(), reservation.Binding.Key)
				case "attach":
					_, err = ledger.AttachPending(context.Background(), reservation)
				case "complete":
					_, err = ledger.CompleteMirror(context.Background(), reservation, receipt)
				}
				r223RequireDiverged(t, err, seed.class)
				require.Equal(t, beforeFields, client.HGetAll(context.Background(), key).Val())
				require.Equal(t, beforeExpiry, client.PExpireTime(context.Background(), key).Val())
			})
		}
	}
}

func TestRedisMirrorV2_DirectMismatchErrorsAndImmutability(t *testing.T) {
	operation := r223Operation(t, LifecycleMethodJoin, 0, true)
	base := r223NewReservation(t, "delegated_user", operation)
	receipt := r223NewReceipt(t, operation)
	key := r223KeyText(base.Binding.Key)
	bindings := map[string]struct {
		value RedisMirrorReservation
		class string
	}{}
	methodOp := r223Operation(t, LifecycleMethodLeave, 2, true)
	methodOp.ActorProfileID = operation.ActorProfileID
	methodOp.OperationID = operation.OperationID
	bindings["method"] = struct {
		value RedisMirrorReservation
		class string
	}{r223NewReservation(t, "delegated_user", methodOp), "binding_mismatch"}
	ownerOp := operation
	ownerOp.RedisOwnerToken = r223Digest(0x41)
	bindings["owner"] = struct {
		value RedisMirrorReservation
		class string
	}{r223NewReservation(t, "delegated_user", ownerOp), "owner_mismatch"}
	fingerprintOp := operation
	fingerprintOp.Fingerprint = r223Digest(0x21)
	bindings["fingerprint"] = struct {
		value RedisMirrorReservation
		class string
	}{r223NewReservation(t, "delegated_user", fingerprintOp), "binding_mismatch"}
	for name, item := range bindings {
		for _, call := range []string{"attach", "complete"} {
			t.Run(name+"/"+call, func(t *testing.T) {
				client, ledger := r223MiniredisLedger(t)
				require.NoError(t, client.HSet(context.Background(), key, r223ReservationFields(base)).Err())
				before := client.HGetAll(context.Background(), key).Val()
				var err error
				if call == "attach" {
					_, err = ledger.AttachPending(context.Background(), item.value)
				} else {
					_, err = ledger.CompleteMirror(context.Background(), item.value, receipt)
				}
				r223RequireDiverged(t, err, item.class)
				require.Equal(t, before, client.HGetAll(context.Background(), key).Val())
				require.Equal(t, time.Duration(-1), client.PTTL(context.Background(), key).Val())
			})
		}
	}
	changedBytes := receipt
	changedBytes.Bytes = append([]byte(nil), receipt.Bytes...)
	changedBytes.Bytes[0] ^= 0xff
	changedBytes.Hash = LifecycleDigest(sha256.Sum256(changedBytes.Bytes))
	changedHash := receipt
	changedHash.Hash = r223Digest(0x70)
	changedDeadline := receipt
	changedDeadline.ReplayUntil = receipt.ReplayUntil.Add(time.Millisecond)
	for name, item := range map[string]struct {
		value RedisMirrorReceipt
		class string
	}{"bytes": {changedBytes, "receipt_mismatch"}, "hash": {changedHash, "receipt_mismatch"}, "deadline": {changedDeadline, "deadline_mismatch"}} {
		t.Run(name, func(t *testing.T) {
			client, ledger := r223MiniredisLedger(t)
			require.NoError(t, client.HSet(context.Background(), key, r223CompletedFields(base, receipt)).Err())
			require.NoError(t, client.PExpireAt(context.Background(), key, receipt.ReplayUntil).Err())
			before := client.HGetAll(context.Background(), key).Val()
			beforeExpiry := client.PExpireTime(context.Background(), key).Val()
			_, err := ledger.CompleteMirror(context.Background(), base, item.value)
			r223RequireDiverged(t, err, item.class)
			require.Equal(t, before, client.HGetAll(context.Background(), key).Val())
			require.Equal(t, beforeExpiry, client.PExpireTime(context.Background(), key).Val())
		})
	}
}

type r223ResponseLossHook struct{ loseNextScript atomic.Bool }

func (h *r223ResponseLossHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *r223ResponseLossHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *r223ResponseLossHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		if err == nil && (cmd.Name() == "eval" || cmd.Name() == "evalsha") && h.loseNextScript.CompareAndSwap(true, false) {
			return io.ErrUnexpectedEOF
		}
		return err
	}
}

type r223CommandEvent struct {
	name string
	args []string
	err  error
}

type r223CommandAudit struct {
	mu     sync.Mutex
	events []r223CommandEvent
}

func (h *r223CommandAudit) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *r223CommandAudit) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, commands []redis.Cmder) error {
		err := next(ctx, commands)
		for _, command := range commands {
			h.record(command, command.Err())
		}
		return err
	}
}
func (h *r223CommandAudit) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		h.record(cmd, err)
		return err
	}
}
func (h *r223CommandAudit) record(command redis.Cmder, err error) {
	args := make([]string, len(command.Args()))
	for i, argument := range command.Args() {
		args[i] = fmt.Sprint(argument)
	}
	h.mu.Lock()
	h.events = append(h.events, r223CommandEvent{name: command.Name(), args: args, err: err})
	h.mu.Unlock()
}
func (h *r223CommandAudit) reset() { h.mu.Lock(); h.events = nil; h.mu.Unlock() }
func (h *r223CommandAudit) snapshot() []r223CommandEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]r223CommandEvent(nil), h.events...)
}

func r223Monitor(t *testing.T, address string) (<-chan string, func()) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	require.NoError(t, err)
	_, err = io.WriteString(conn, "*1\r\n$7\r\nMONITOR\r\n")
	require.NoError(t, err)
	reader := bufio.NewReader(conn)
	ack, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(ack, "+OK"))
	lines := make(chan string, 256)
	go func() {
		defer close(lines)
		for {
			line, e := reader.ReadString('\n')
			if e != nil {
				return
			}
			lines <- line
		}
	}()
	return lines, func() { _ = conn.Close() }
}
func r223MonitorCollect(lines <-chan string) []string {
	out := []string{}
	quiet := time.NewTimer(250 * time.Millisecond)
	defer quiet.Stop()
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return out
			}
			out = append(out, line)
			if !quiet.Stop() {
				select {
				case <-quiet.C:
				default:
				}
			}
			quiet.Reset(250 * time.Millisecond)
		case <-quiet.C:
			return out
		}
	}
}
func r223RequireOnlyLuaClientCommands(t *testing.T, events []r223CommandEvent, key string, expectation r223MonitorExpectation) []string {
	t.Helper()
	require.NotEmpty(t, events)
	commands := make([]string, 0, len(events))
	for _, event := range events {
		require.Contains(t, []string{"eval", "evalsha"}, event.name, "foreign client command: %v", event.args)
		require.Len(t, event.args, expectation.evalArgc, "EVAL/EVALSHA must have the exact operation argv count")
		require.Equal(t, event.name, strings.ToLower(event.args[0]), "hook command name and argv[0] must identify the same script command")
		require.Equal(t, "1", event.args[2], "Lua must receive exactly one KEYS entry")
		require.Equal(t, key, event.args[3])
		require.Equal(t, expectation.argv, event.args[4:], "Lua ARGV method/fingerprint/owner/receipt/hash/deadline values and positions are contract data")
		joined := strings.Join(event.args, " ")
		require.NotContains(t, joined, "voice:lifecycle:op:")
		require.NotContains(t, joined, strings.Replace(key, ":delegated_user:", ":system_disconnect:", 1))
		commands = append(commands, event.name)
	}
	switch len(events) {
	case 1:
		require.Equal(t, "evalsha", events[0].name, "a cached script must execute as one EVALSHA")
		require.NoError(t, events[0].err, "the single EVALSHA must be the successful execution")
	case 2:
		require.Equal(t, []string{"evalsha", "eval"}, commands, "the only fallback is EVALSHA NOSCRIPT followed by one EVAL")
		require.Error(t, events[0].err)
		noscript := strings.ToUpper(events[0].err.Error())
		require.True(t, noscript == "NOSCRIPT" || strings.HasPrefix(noscript, "NOSCRIPT "), "fallback is permitted only for the exact NOSCRIPT Redis class: %v", events[0].err)
		require.NoError(t, events[1].err, "the one fallback EVAL must execute successfully")
	default:
		require.FailNow(t, "invalid script execution sequence", "got %d top-level commands: %v", len(events), commands)
	}
	return commands
}

type r223LuaCommandExpectation struct {
	count  int
	argc   int
	fields []string
}

type r223MonitorExpectation struct {
	evalArgc int
	argv     []string
	lua      map[string]r223LuaCommandExpectation
	expiry   string
}

func r223MonitorCommand(line string) (scope string, arguments []string, ok bool) {
	marker := strings.Index(line, "] ")
	if marker < 0 {
		return "", nil, false
	}
	payload := line[marker+2:]
	if strings.Contains(line[:marker+1], " lua]") {
		scope = "lua"
	} else {
		scope = "client"
	}
	for cursor := 0; cursor < len(payload); {
		for cursor < len(payload) && (payload[cursor] == ' ' || payload[cursor] == '\r' || payload[cursor] == '\n') {
			cursor++
		}
		if cursor == len(payload) {
			break
		}
		if payload[cursor] != '"' {
			return "", nil, false
		}
		start := cursor
		cursor++
		escaped := false
		for cursor < len(payload) {
			character := payload[cursor]
			cursor++
			if escaped {
				escaped = false
				continue
			}
			if character == '\\' {
				escaped = true
				continue
			}
			if character == '"' {
				break
			}
		}
		if cursor > len(payload) || payload[cursor-1] != '"' {
			return "", nil, false
		}
		argument, err := strconv.Unquote(payload[start:cursor])
		if err != nil {
			return "", nil, false
		}
		arguments = append(arguments, argument)
	}
	return scope, arguments, len(arguments) > 0
}

func TestR223D2D3Harness_MonitorParserAndPipelineAuditAreComplete(t *testing.T) {
	key := "voice:lifecycle:v2:op:delegated_user:22222222-2222-4222-8222-222222222222:11111111-1111-4111-8111-111111111111"
	for _, sample := range []struct {
		line  string
		scope string
		args  []string
	}{
		{`1700000000.1 [0 127.0.0.1:1] "evalsha" "abc" "1" "` + key + `" "pending"`, "client", []string{"evalsha", "abc", "1", key, "pending"}},
		{`1700000000.2 [0 lua] "HSET" "` + key + `" "state" "pending"`, "lua", []string{"HSET", key, "state", "pending"}},
		{`1700000000.3 [0 lua] "HSET" "` + key + `" "receipt_bytes" "\xff\x00"`, "lua", []string{"HSET", key, "receipt_bytes", "\xff\x00"}},
	} {
		scope, arguments, ok := r223MonitorCommand(sample.line)
		require.True(t, ok)
		require.Equal(t, sample.scope, scope)
		require.Equal(t, sample.args, arguments)
	}

	client, _ := r223MiniredisLedger(t)
	audit := &r223CommandAudit{}
	client.AddHook(audit)
	audit.reset()
	_, err := client.Pipelined(context.Background(), func(pipe redis.Pipeliner) error {
		pipe.Set(context.Background(), "pipeline:key", "value", 0)
		pipe.Get(context.Background(), "pipeline:key")
		return nil
	})
	require.NoError(t, err)
	events := audit.snapshot()
	require.Len(t, events, 2)
	require.Equal(t, []string{"set", "get"}, []string{events[0].name, events[1].name})
	require.Equal(t, []string{"set", "pipeline:key", "value"}, events[0].args)
	require.Equal(t, []string{"get", "pipeline:key"}, events[1].args)
	require.NoError(t, events[0].err)
	require.NoError(t, events[1].err)
	pendingFields := []string{"schema_version", "state", "method", "fingerprint", "owner_token"}
	attachExpectation := r223MonitorExpectation{evalArgc: 7, argv: []string{"join", strings.Repeat("2", 64), strings.Repeat("4", 64)}, lua: map[string]r223LuaCommandExpectation{
		"hlen": {count: 1, argc: 2},
		"hset": {count: 1, argc: 12, fields: pendingFields},
	}}
	successCommands := r223RequireOnlyLuaClientCommands(t, []r223CommandEvent{{name: "evalsha", args: []string{"evalsha", "abc", "1", key, "join", strings.Repeat("2", 64), strings.Repeat("4", 64)}}}, key, attachExpectation)
	require.Equal(t, []string{"evalsha"}, successCommands)
	fallbackCommands := r223RequireOnlyLuaClientCommands(t, []r223CommandEvent{
		{name: "evalsha", args: []string{"evalsha", "abc", "1", key, "join", strings.Repeat("2", 64), strings.Repeat("4", 64)}, err: errors.New("NOSCRIPT No matching script")},
		{name: "eval", args: []string{"eval", "return 1", "1", key, "join", strings.Repeat("2", 64), strings.Repeat("4", 64)}},
	}, key, attachExpectation)
	require.Equal(t, []string{"evalsha", "eval"}, fallbackCommands)
	r223RequireExactMonitorAllowlist(t, []string{
		`1700000001.1 [0 127.0.0.1:1] "evalsha" "abc" "1" "` + key + `" "join" "` + strings.Repeat("2", 64) + `" "` + strings.Repeat("4", 64) + `"`,
		`1700000001.2 [0 lua] "HLEN" "` + key + `"`,
		`1700000001.3 [0 lua] "HSET" "` + key + `" "schema_version" "2" "state" "pending" "method" "join" "fingerprint" "` + strings.Repeat("2", 64) + `" "owner_token" "` + strings.Repeat("4", 64) + `"`,
	}, key, attachExpectation, successCommands)
}

func r223RequireExactMonitorAllowlist(t *testing.T, lines []string, key string, expectation r223MonitorExpectation, expectedClientCommands []string) {
	t.Helper()
	require.NotEmpty(t, lines)
	counts := map[string]int{}
	clientScripts := 0
	luaCommands := 0
	for _, line := range lines {
		scope, arguments, ok := r223MonitorCommand(line)
		require.True(t, ok, "unparseable MONITOR line: %q", line)
		command := strings.ToLower(arguments[0])
		switch scope {
		case "client":
			require.Contains(t, []string{"eval", "evalsha"}, command, "foreign top-level Redis command: %q", line)
			require.Len(t, arguments, expectation.evalArgc, "MONITOR must expose the exact EVAL argv count")
			require.Equal(t, "1", arguments[2], "script must declare exactly one key")
			require.Equal(t, key, arguments[3], "KEYS[1] must be the exact v2 key")
			require.Equal(t, expectation.argv, arguments[4:], "MONITOR Lua ARGV values and positions must exactly match the operation")
			clientScripts++
		case "lua":
			commandExpectation, commandAllowed := expectation.lua[command]
			require.True(t, commandAllowed, "Lua command %q is outside the exact operation schema %v", command, expectation.lua)
			luaCommands++
			counts[command]++
			require.Len(t, arguments, commandExpectation.argc, "Lua %s must have exact arity", command)
			if command == "time" {
				require.Len(t, arguments, 1)
			} else {
				require.Equal(t, key, arguments[1], "every keyed Lua command must use the exact key in argv[1]")
			}
			switch command {
			case "hmget":
				require.Equal(t, commandExpectation.fields, arguments[2:], "HMGET field order/count is contract data")
			case "hset":
				fields := make([]string, 0, (len(arguments)-2)/2)
				for index := 2; index < len(arguments); index += 2 {
					fields = append(fields, arguments[index])
				}
				require.Equal(t, commandExpectation.fields, fields, "HSET field order/count is contract data")
			case "pexpireat":
				require.Equal(t, expectation.expiry, arguments[2], "PEXPIREAT must use the PostgreSQL millisecond deadline")
			}
		}
		for index, argument := range arguments {
			if strings.Contains(argument, "voice:lifecycle") {
				wantIndex := 1
				if scope == "client" {
					wantIndex = 3
				}
				require.Equal(t, wantIndex, index, "a lifecycle key may occur only at the command's exact key argument")
				require.Equal(t, key, argument, "no v1, foreign-origin, or foreign lifecycle-key argument is allowed")
			}
		}
	}
	require.Equal(t, len(expectedClientCommands), clientScripts, "MONITOR must expose every and only the audited script commands")
	monitorClientCommands := make([]string, 0, clientScripts)
	for _, line := range lines {
		scope, arguments, ok := r223MonitorCommand(line)
		require.True(t, ok)
		if scope == "client" {
			monitorClientCommands = append(monitorClientCommands, strings.ToLower(arguments[0]))
		}
	}
	require.Equal(t, expectedClientCommands, monitorClientCommands, "MONITOR must preserve the exact EVALSHA-success or EVALSHA-NOSCRIPT/EVAL sequence")
	expectedLuaCommands := 0
	for command, commandExpectation := range expectation.lua {
		expectedLuaCommands += commandExpectation.count
		require.Equal(t, commandExpectation.count, counts[command], "exact internal %s command count", command)
	}
	require.Equal(t, expectedLuaCommands, luaCommands, "the executed script must expose exactly the operation's internal command count")
}

func TestRedisMirrorV2_RealRedisAllCommandsMonitorAndNoWriteReplay(t *testing.T) {
	ctx := context.Background()
	address, observer := r223StartRealRedis(t, ctx)
	r223RequireRedis7(t, ctx, observer)
	operation := r223Operation(t, LifecycleMethodJoin, 0, true)
	reservation := r223NewReservation(t, "delegated_user", operation)
	receipt := r223NewReceipt(t, operation)
	key := r223KeyText(reservation.Binding.Key)
	client := redis.NewClient(&redis.Options{Addr: address, MaxRetries: 0})
	audit := &r223CommandAudit{}
	client.AddHook(audit)
	require.NoError(t, client.Ping(ctx).Err())
	audit.reset()
	ledger := NewRedisLedger(client)
	defer client.Close()
	pendingFields := []string{"schema_version", "state", "method", "fingerprint", "owner_token"}
	completedFields := []string{"schema_version", "state", "method", "fingerprint", "owner_token", "receipt_bytes", "receipt_hash", "replay_until"}
	completionWriteFields := []string{"state", "receipt_bytes", "receipt_hash", "replay_until"}
	expiryMillis := fmt.Sprint(receipt.ReplayUntil.UnixMilli())
	reservationArgv := []string{r223MethodName(reservation.Binding.Method), hex.EncodeToString(reservation.Binding.Fingerprint[:]), hex.EncodeToString(reservation.OwnerToken[:])}
	completionArgv := append(append([]string(nil), reservationArgv...), string(receipt.Bytes), hex.EncodeToString(receipt.Hash[:]), expiryMillis)
	calls := []struct {
		name        string
		prepare     func()
		invoke      func() error
		expectation r223MonitorExpectation
	}{
		{"inspect", func() { observer.Del(ctx, key) }, func() error { _, e := ledger.InspectMirror(ctx, reservation.Binding.Key); return e }, r223MonitorExpectation{evalArgc: 4, argv: []string{}, lua: map[string]r223LuaCommandExpectation{"hlen": {count: 1, argc: 2}}}},
		{"attach", func() { observer.Del(ctx, key) }, func() error { _, e := ledger.AttachPending(ctx, reservation); return e }, r223MonitorExpectation{evalArgc: 7, argv: reservationArgv, lua: map[string]r223LuaCommandExpectation{"hlen": {count: 1, argc: 2}, "hset": {count: 1, argc: 12, fields: pendingFields}}}},
		{"complete", func() {
			observer.Del(ctx, key)
			_, e := ledger.AttachPending(ctx, reservation)
			require.NoError(t, e)
			audit.reset()
		}, func() error { _, e := ledger.CompleteMirror(ctx, reservation, receipt); return e }, r223MonitorExpectation{evalArgc: 10, argv: completionArgv, expiry: expiryMillis, lua: map[string]r223LuaCommandExpectation{"hlen": {count: 1, argc: 2}, "hmget": {count: 1, argc: 10, fields: completedFields}, "pttl": {count: 1, argc: 2}, "time": {count: 1, argc: 1}, "hset": {count: 1, argc: 10, fields: completionWriteFields}, "pexpireat": {count: 1, argc: 3}}}},
		{"replay", func() { _, e := ledger.CompleteMirror(ctx, reservation, receipt); require.NoError(t, e); audit.reset() }, func() error { _, e := ledger.CompleteMirror(ctx, reservation, receipt); return e }, r223MonitorExpectation{evalArgc: 10, argv: completionArgv, lua: map[string]r223LuaCommandExpectation{"hlen": {count: 1, argc: 2}, "hmget": {count: 1, argc: 10, fields: completedFields}, "pexpiretime": {count: 1, argc: 2}, "pttl": {count: 1, argc: 2}, "time": {count: 1, argc: 1}}}},
	}
	for _, call := range calls {
		t.Run(call.name, func(t *testing.T) {
			call.prepare()
			audit.reset()
			lines, stop := r223Monitor(t, address)
			err := call.invoke()
			require.NoError(t, err)
			monitored := r223MonitorCollect(lines)
			stop()
			events := audit.snapshot()
			expectedClientCommands := r223RequireOnlyLuaClientCommands(t, events, key, call.expectation)
			dump := strings.Join(monitored, "\n")
			require.Contains(t, dump, key)
			require.NotContains(t, dump, "voice:lifecycle:op:")
			require.NotContains(t, dump, strings.Replace(key, ":delegated_user:", ":system_disconnect:", 1))
			r223RequireExactMonitorAllowlist(t, monitored, key, call.expectation, expectedClientCommands)
		})
	}
}

type r223RaceResult struct {
	label string
	err   error
}

func r223Race(t *testing.T, address string, contenders map[string]func(*RedisLedger) error) []r223RaceResult {
	t.Helper()
	start := make(chan struct{})
	results := make(chan r223RaceResult, len(contenders)*8)
	var wg sync.WaitGroup
	for label, call := range contenders {
		for i := 0; i < 8; i++ {
			client := redis.NewClient(&redis.Options{Addr: address, MaxRetries: 0})
			wg.Add(1)
			go func(label string) {
				defer wg.Done()
				defer client.Close()
				<-start
				results <- r223RaceResult{label, call(NewRedisLedger(client))}
			}(label)
		}
	}
	close(start)
	wg.Wait()
	close(results)
	out := []r223RaceResult{}
	for result := range results {
		out = append(out, result)
	}
	return out
}

func TestRedisMirrorV2_RealRedisFirstWriterRacesExactLoserClasses(t *testing.T) {
	ctx := context.Background()
	address, observer := r223StartRealRedis(t, ctx)
	r223RequireRedis7(t, ctx, observer)
	baseOp := r223Operation(t, LifecycleMethodJoin, 0, true)
	base := r223NewReservation(t, "delegated_user", baseOp)
	receipt := r223NewReceipt(t, baseOp)
	key := r223KeyText(base.Binding.Key)
	pendingCases := map[string]struct {
		op    LifecycleOperation
		class string
	}{"method": {r223Operation(t, LifecycleMethodLeave, 2, true), "binding_mismatch"}, "owner": {baseOp, "owner_mismatch"}, "fingerprint": {baseOp, "binding_mismatch"}}
	method := pendingCases["method"]
	method.op.ActorProfileID = baseOp.ActorProfileID
	method.op.OperationID = baseOp.OperationID
	pendingCases["method"] = method
	owner := pendingCases["owner"]
	owner.op.RedisOwnerToken = r223Digest(0x41)
	pendingCases["owner"] = owner
	fp := pendingCases["fingerprint"]
	fp.op.Fingerprint = r223Digest(0x21)
	pendingCases["fingerprint"] = fp
	for name, item := range pendingCases {
		t.Run("absent_pending_"+name, func(t *testing.T) {
			observer.Del(ctx, key)
			other := r223NewReservation(t, "delegated_user", item.op)
			results := r223Race(t, address, map[string]func(*RedisLedger) error{"base": func(l *RedisLedger) error { _, e := l.AttachPending(ctx, base); return e }, "other": func(l *RedisLedger) error { _, e := l.AttachPending(ctx, other); return e }})
			fields := observer.HGetAll(ctx, key).Val()
			winner := ""
			if fmt.Sprint(fields) == fmt.Sprint(r223ReservationFields(base)) {
				winner = "base"
			} else if fmt.Sprint(fields) == fmt.Sprint(r223ReservationFields(other)) {
				winner = "other"
			}
			require.NotEmpty(t, winner)
			for _, result := range results {
				if result.label == winner {
					require.NoError(t, result.err)
				} else {
					r223RequireDiverged(t, result.err, item.class)
				}
			}
		})
	}
	changedDeadline := receipt
	changedDeadline.ReplayUntil = receipt.ReplayUntil.Add(time.Millisecond)
	receiptOperation := r223Operation(t, LifecycleMethodJoin, 1, true)
	receiptOperation.ActorProfileID = baseOp.ActorProfileID
	receiptOperation.OperationID = baseOp.OperationID
	changedReceipt := r223NewReceipt(t, receiptOperation)
	completedCases := map[string]struct {
		value RedisMirrorReceipt
		class string
	}{"receipt": {changedReceipt, "receipt_mismatch"}, "deadline": {changedDeadline, "deadline_mismatch"}}
	for _, transition := range []string{"absent_completed", "pending_completed"} {
		for conflict, contender := range completedCases {
			t.Run(transition+"_"+conflict, func(t *testing.T) {
				observer.Del(ctx, key)
				if strings.HasPrefix(transition, "pending") {
					_, e := NewRedisLedger(observer).AttachPending(ctx, base)
					require.NoError(t, e)
				}
				results := r223Race(t, address, map[string]func(*RedisLedger) error{"base": func(l *RedisLedger) error { _, e := l.CompleteMirror(ctx, base, receipt); return e }, "other": func(l *RedisLedger) error { _, e := l.CompleteMirror(ctx, base, contender.value); return e }})
				winnerHash := observer.HGet(ctx, key, "receipt_hash").Val()
				winnerDeadline := observer.HGet(ctx, key, "replay_until").Val()
				winner := "base"
				if winnerDeadline == fmt.Sprint(contender.value.ReplayUntil.UnixMilli()) && winnerHash == hex.EncodeToString(contender.value.Hash[:]) {
					winner = "other"
				}
				for _, result := range results {
					if result.label == winner {
						require.NoError(t, result.err)
					} else {
						r223RequireDiverged(t, result.err, contender.class)
					}
				}
				require.EqualValues(t, 8, observer.HLen(ctx, key).Val())
			})
		}
	}
}

func TestRedisMirrorV2_RealRedisResponseLossIsUnavailableAndRecoverable(t *testing.T) {
	ctx := context.Background()
	address, observer := r223StartRealRedis(t, ctx)
	operation := r223Operation(t, LifecycleMethodJoin, 0, true)
	reservation := r223NewReservation(t, "delegated_user", operation)
	receipt := r223NewReceipt(t, operation)
	hook := &r223ResponseLossHook{}
	client := redis.NewClient(&redis.Options{Addr: address, MaxRetries: 0})
	client.AddHook(hook)
	defer client.Close()
	for _, call := range []string{"attach", "complete"} {
		t.Run(call, func(t *testing.T) {
			require.NoError(t, observer.Del(ctx, r223KeyText(reservation.Binding.Key)).Err())
			hook.loseNextScript.Store(true)
			var err error
			if call == "attach" {
				_, err = NewRedisLedger(client).AttachPending(ctx, reservation)
			} else {
				_, err = NewRedisLedger(client).CompleteMirror(ctx, reservation, receipt)
			}
			require.True(t, errors.Is(err, ErrRedisMirrorUnavailable))
			if call == "attach" {
				observation, recoverErr := NewRedisLedger(observer).AttachPending(ctx, reservation)
				require.NoError(t, recoverErr)
				require.Equal(t, RedisMirrorPending, observation.State)
			} else {
				observation, recoverErr := NewRedisLedger(observer).CompleteMirror(ctx, reservation, receipt)
				require.NoError(t, recoverErr)
				require.Equal(t, receipt.Bytes, observation.Receipt.Bytes)
			}
		})
	}
}

func TestRedisMirrorV2_RealRedisAtomicAbsentCompletionExactPEXPIRETIME(t *testing.T) {
	ctx := context.Background()
	_, client := r223StartRealRedis(t, ctx)
	operation := r223Operation(t, LifecycleMethodJoin, 0, true)
	now, err := client.Time(ctx).Result()
	require.NoError(t, err)
	deadline := now.Add(time.Hour).Truncate(time.Millisecond)
	operation.ReplayUntil = &deadline
	reservation := r223NewReservation(t, "delegated_user", operation)
	receipt := r223NewReceipt(t, operation)
	observation, err := NewRedisLedger(client).CompleteMirror(ctx, reservation, receipt)
	require.NoError(t, err)
	require.Equal(t, RedisMirrorCompleted, observation.State)
	key := r223KeyText(reservation.Binding.Key)
	require.EqualValues(t, 8, client.HLen(ctx, key).Val())
	require.Equal(t, deadline.UnixMilli(), client.Do(ctx, "PEXPIRETIME", key).Val())
	beforeDump := client.Dump(ctx, key).Val()
	_, err = NewRedisLedger(client).CompleteMirror(ctx, reservation, receipt)
	require.NoError(t, err)
	require.Equal(t, beforeDump, client.Dump(ctx, key).Val())
	require.Equal(t, deadline.UnixMilli(), client.Do(ctx, "PEXPIRETIME", key).Val())
}

type r223StepClock struct {
	times []time.Time
	calls int
}

func (c *r223StepClock) Now() time.Time {
	if c.calls >= len(c.times) {
		panic("step clock exhausted")
	}
	value := c.times[c.calls]
	c.calls++
	return value
}

func TestRedisReplayDeadline_DirectRedisMirrorBridgeRealPGRedisStepClock(t *testing.T) {
	ctx := context.Background()
	fixture := r22NewStoreFixture(t, "r223bridge_"+strings.ReplaceAll(uuid.NewString(), "-", ""))
	address, observer := r223StartRealRedis(t, ctx)
	r223RequireRedis7(t, ctx, observer)
	redisNow, err := observer.Time(ctx).Result()
	require.NoError(t, err)
	type row struct {
		name     string
		deadline time.Time
		steps    []time.Time
		seedLag  bool
		kind     error
		scripts  int
		receipt  bool
	}
	rows := []row{{"before", redisNow.Add(5 * time.Minute).Truncate(time.Millisecond), []time.Time{redisNow.Add(4 * time.Minute), redisNow.Add(4*time.Minute + time.Second), redisNow.Add(4*time.Minute + 2*time.Second)}, false, nil, 1, true}, {"equality", redisNow.Add(6 * time.Minute).Truncate(time.Millisecond), []time.Time{redisNow.Add(6 * time.Minute).Truncate(time.Millisecond), redisNow.Add(6 * time.Minute).Truncate(time.Millisecond)}, false, ErrRedisMirrorExpired, 0, false}, {"post_lua_equality", redisNow.Add(7 * time.Minute).Truncate(time.Millisecond), []time.Time{redisNow.Add(7*time.Minute - time.Second), redisNow.Add(7*time.Minute - time.Millisecond), redisNow.Add(7 * time.Minute).Truncate(time.Millisecond)}, false, ErrRedisMirrorExpired, 1, false}, {"redis_lead", redisNow.Add(-time.Millisecond).Truncate(time.Millisecond), []time.Time{redisNow.Add(-3 * time.Millisecond), redisNow.Add(-2 * time.Millisecond)}, false, ErrRedisMirrorUnavailable, 1, false}, {"redis_lag", redisNow.Add(8 * time.Minute).Truncate(time.Millisecond), []time.Time{redisNow.Add(8 * time.Minute).Truncate(time.Millisecond), redisNow.Add(8 * time.Minute).Truncate(time.Millisecond)}, true, ErrRedisMirrorExpired, 0, false}}
	for _, testCase := range rows {
		t.Run(testCase.name, func(t *testing.T) {
			decision := fixture.decision(LifecycleMethodLeave)
			decision.OperationID = uuid.New()
			operation, e := fixture.store.CompleteNoOp(ctx, LifecycleNoOpDecision{Decision: decision, CompletedAt: testCase.deadline.Add(-25 * time.Hour), ReplayUntil: testCase.deadline})
			require.NoError(t, e)
			clock := &r223StepClock{times: testCase.steps}
			candidate, e := fixture.store.LoadRedisMirrorCandidate(ctx, "delegated_user", operation.ActorProfileID, operation.OperationID, clock.Now())
			require.NoError(t, e)
			reservation := r223NewReservation(t, "delegated_user", operation)
			receipt := r223NewReceipt(t, operation)
			key := r223KeyText(reservation.Binding.Key)
			var lagDump string
			var lagExpiry int64
			if testCase.seedLag {
				require.NoError(t, observer.HSet(ctx, key, r223CompletedFields(reservation, receipt)).Err())
				require.NoError(t, observer.PExpireAt(ctx, key, receipt.ReplayUntil.Add(time.Hour)).Err())
				lagDump, e = observer.Dump(ctx, key).Result()
				require.NoError(t, e)
				lagExpiry, e = observer.Do(ctx, "PEXPIRETIME", key).Int64()
				require.NoError(t, e)
			}
			audit := &r223CommandAudit{}
			client := redis.NewClient(&redis.Options{Addr: address, MaxRetries: 0})
			client.AddHook(audit)
			require.NoError(t, client.Ping(ctx).Err())
			audit.reset()
			bridge := NewRedisMirrorBridge(fixture.store, NewRedisLedger(client), clock.Now)
			result, bridgeErr := bridge.ReconcileCompleted(ctx, candidate)
			client.Close()
			require.Equal(t, len(testCase.steps), clock.calls, "candidate load and every required bridge deadline gate must consume one explicit instant")
			if testCase.kind == nil {
				require.NoError(t, bridgeErr)
			} else {
				require.True(t, errors.Is(bridgeErr, testCase.kind))
			}
			if testCase.receipt {
				require.Equal(t, operation.ReceiptBytes, result.ReceiptBytes)
			} else {
				require.Empty(t, result.ReceiptBytes)
			}
			events := audit.snapshot()
			if testCase.name == "equality" || testCase.name == "redis_lag" {
				require.Empty(t, events, "expired PG candidates must perform zero total Redis commands, including failed script attempts")
			}
			successfulScripts := 0
			for _, event := range events {
				if (event.name == "eval" || event.name == "evalsha") && event.err == nil {
					successfulScripts++
				}
			}
			require.Equal(t, testCase.scripts, successfulScripts)
			if testCase.name == "redis_lag" {
				require.EqualValues(t, 1, observer.Exists(ctx, key).Val())
				afterDump, dumpErr := observer.Dump(ctx, key).Result()
				require.NoError(t, dumpErr)
				afterExpiry, expiryErr := observer.Do(ctx, "PEXPIRETIME", key).Int64()
				require.NoError(t, expiryErr)
				require.Equal(t, lagDump, afterDump, "zero-work lag row must preserve source DUMP")
				require.Equal(t, lagExpiry, afterExpiry, "zero-work lag row must preserve source PEXPIRETIME")
			}
			if testCase.name == "redis_lead" {
				require.Zero(t, observer.Exists(ctx, key).Val())
			}
		})
	}
}
