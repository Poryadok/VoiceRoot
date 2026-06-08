package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
)

func TestGRPCCodeToHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.OK, http.StatusOK},
		{codes.InvalidArgument, http.StatusBadRequest},
		{codes.OutOfRange, http.StatusBadRequest},
		{codes.NotFound, http.StatusNotFound},
		{codes.AlreadyExists, http.StatusConflict},
		{codes.PermissionDenied, http.StatusForbidden},
		{codes.Unauthenticated, http.StatusUnauthorized},
		{codes.FailedPrecondition, http.StatusPreconditionFailed},
		{codes.ResourceExhausted, http.StatusTooManyRequests},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.Internal, http.StatusInternalServerError},
		{codes.Unknown, http.StatusInternalServerError},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.code.String(), func(t *testing.T) {
			t.Parallel()
			if got := grpcCodeToHTTP(tc.code); got != tc.want {
				t.Fatalf("grpcCodeToHTTP(%v) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}

func TestWriteGRPCError_NonGRPCError(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	writeGRPCError(rec, errors.New("plain failure"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestGRPCCodeToErrorCode(t *testing.T) {
	t.Parallel()
	if got := grpcCodeToErrorCode(codes.ResourceExhausted); got != "resource_exhausted" {
		t.Fatalf("got %q", got)
	}
	if got := grpcCodeToErrorCode(codes.Unknown); got != "internal" {
		t.Fatalf("got %q", got)
	}
}
