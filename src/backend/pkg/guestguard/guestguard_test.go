package guestguard

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAccountType(t *testing.T) {
	t.Parallel()

	ctxRegular := context.Background()
	if got := AccountType(ctxRegular); got != AccountTypeRegular {
		t.Fatalf("empty ctx: got %q want regular", got)
	}

	ctxGuest := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		HeaderAccountType, "guest",
	))
	if got := AccountType(ctxGuest); got != AccountTypeGuest {
		t.Fatalf("guest header: got %q want guest", got)
	}

	ctxUnknown := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		HeaderAccountType, "  GUEST  ",
	))
	if got := AccountType(ctxUnknown); got != AccountTypeGuest {
		t.Fatalf("case/space guest: got %q want guest", got)
	}

	ctxOther := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		HeaderAccountType, "trial",
	))
	if got := AccountType(ctxOther); got != AccountTypeRegular {
		t.Fatalf("unknown type: got %q want regular", got)
	}
}

func TestRequireRegular(t *testing.T) {
	t.Parallel()

	if err := RequireRegular(context.Background()); err != nil {
		t.Fatalf("regular caller: unexpected error %v", err)
	}

	ctxGuest := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		HeaderAccountType, AccountTypeGuest,
	))
	err := RequireRegular(ctxGuest)
	if err == nil {
		t.Fatal("guest caller: expected error")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("guest caller: code=%v want PermissionDenied", status.Code(err))
	}
}

func TestIsGuest(t *testing.T) {
	t.Parallel()

	if IsGuest(context.Background()) {
		t.Fatal("regular ctx should not be guest")
	}
	ctxGuest := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		HeaderAccountType, AccountTypeGuest,
	))
	if !IsGuest(ctxGuest) {
		t.Fatal("guest ctx should be guest")
	}
}
