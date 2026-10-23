package main

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestInstallationOwnerClosesPartialFactoryResult(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	owner := new(installationOwner)
	err = owner.construct(func(context.Context) (*installationRuntime, error) {
		return &installationRuntime{listener: listener}, errors.New("dependency unavailable")
	})
	if err == nil {
		t.Fatal("partial startup succeeded")
	}
	replacement, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("partial listener leaked: %v", err)
	}
	_ = replacement.Close()
	owner.Close()
	owner.Close()
	called := false
	err = owner.construct(func(context.Context) (*installationRuntime, error) { called = true; return nil, nil })
	if err == nil || called {
		t.Fatal("closed owner restarted")
	}
}

func TestInstallationOwnerClosesPublishedRuntime(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	owner := new(installationOwner)
	if err = owner.construct(func(context.Context) (*installationRuntime, error) {
		return &installationRuntime{listener: listener}, nil
	}); err != nil {
		t.Fatal(err)
	}
	owner.Close()
	owner.Close()
	replacement, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("owned listener leaked: %v", err)
	}
	_ = replacement.Close()
}
