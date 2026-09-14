package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestIsJetStreamNotFound(t *testing.T) {
	if !isJetStreamNotFound(nats.ErrStreamNotFound) {
		t.Fatal("expected ErrStreamNotFound")
	}
	if !isJetStreamNotFound(errors.New("jetstream subscribe message.events: nats: stream not found")) {
		t.Fatal("expected wrapped stream not found")
	}
	if isJetStreamNotFound(errors.New("connection refused")) {
		t.Fatal("unexpected match")
	}
}

func TestSubscribeJetStreamWithRetry_WaitsForStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempts := 0
	sub, err := subscribeJetStreamWithRetry(ctx, "test", func() (*nats.Subscription, error) {
		attempts++
		if attempts < 3 {
			return nil, nats.ErrStreamNotFound
		}
		return &nats.Subscription{}, nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if sub == nil {
		t.Fatal("expected subscription")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestSubscribeJetStreamWithRetryStopsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	_, err := subscribeJetStreamWithRetry(ctx, "test", func() (*nats.Subscription, error) {
		attempts++
		return nil, nats.ErrStreamNotFound
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("subscribe error = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRoleEventsConsumerUsesSharedJetStreamRetry(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	roleFile := filepath.Join(filepath.Dir(thisFile), "role_events_consumer.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), roleFile, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", roleFile, err)
	}

	usesSharedRetry := false
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name == nil || function.Name.Name != "runRoleEventsConsumer" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if ok && identifier.Name == "subscribeJetStreamWithRetry" {
				usesSharedRetry = true
			}
			return true
		})
	}
	if !usesSharedRetry {
		t.Fatal("runRoleEventsConsumer must subscribe through subscribeJetStreamWithRetry")
	}
}
