package spacemedia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpaceMediaAdmissionCommitFailureLogUsesOnlyFixedClass(t *testing.T) {
	secret := errors.New("postgres: profile=private operation=secret detail=not-for-logs")
	cases := []struct {
		name  string
		err   error
		class string
	}{
		{name: "room head pending", err: fmt.Errorf("wrapped: %w", ErrRoomHeadPending), class: "room_head_pending"},
		{name: "admission conflict", err: fmt.Errorf("wrapped: %w", ErrAdmissionConflict), class: "admission_conflict"},
		{name: "admission unavailable", err: fmt.Errorf("wrapped: %w", ErrAdmissionUnavailable), class: "admission_unavailable"},
		{name: "store error", err: secret, class: "store_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			logSpaceMediaAdmissionCommitFailure(context.Background(), logger, tc.err)

			line := output.String()
			require.Contains(t, line, `"failure_class":"`+tc.class+`"`)
			require.NotContains(t, line, "postgres")
			require.NotContains(t, line, "private")
			require.NotContains(t, line, "secret")
			require.NotContains(t, line, "operation")
		})
	}
}
