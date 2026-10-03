package spaceprincipalruntime

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestJWKSDependencyFailuresRemainUnavailable(t *testing.T) {
	for _, cause := range []error{
		errors.New("connection refused"),
		status.Error(codes.Internal, "invalid upstream response"),
	} {
		if got := status.Code(unavailableJWKS(cause)); got != codes.Unavailable {
			t.Errorf("unavailableJWKS(%T) code = %s, want %s", cause, got, codes.Unavailable)
		}
	}
}
