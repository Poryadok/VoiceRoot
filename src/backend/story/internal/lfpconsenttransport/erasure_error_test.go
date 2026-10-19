package lfpconsenttransport

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/story/internal/lfpconsentstore"
)

func TestActionErasureTerminalDiffersFromUnknown(t *testing.T) {
	terminal := actionError(lfpconsentstore.ErrErased)
	unknown := actionError(lfpconsentstore.ErrUnavailable)
	require.Equal(t, codes.FailedPrecondition, status.Code(terminal))
	require.Equal(t, codes.Unavailable, status.Code(unknown))
	require.Equal(t, "LFP action ineligible", status.Convert(terminal).Message())
	require.Equal(t, "LFP action unavailable", status.Convert(unknown).Message())
}
