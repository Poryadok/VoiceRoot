package squad

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProvisionErrorClassificationAlwaysUsesPersistableFiniteClass(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{errors.New("private transport error"), "unknown"},
		{status.Error(codes.DataLoss, "private error"), "unknown"},
		{status.Error(codes.InvalidArgument, "private error"), "invalid_argument"},
		{status.Error(codes.FailedPrecondition, "private error"), "failed_precondition"},
		{ErrPartyProvisionUnknown, "waiting_authority"},
	} {
		if got := matchSquadProvisionErrorClass(test.err); got != test.want {
			t.Errorf("class %q, want %q", got, test.want)
		}
	}
}
