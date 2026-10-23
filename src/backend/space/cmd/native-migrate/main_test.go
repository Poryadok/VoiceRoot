package main

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
	"voice/backend/space/internal/nativeinstaller"
)

func TestInstallerDoesNotDefaultToTrustExisting(t *testing.T) {
	for _, mode := range []string{"", "up", "force", "trust-existing", "resume"} {
		require.Error(t, nativeinstaller.Run(context.Background(), nativeinstaller.Config{Mode: mode, Output: io.Discard}))
	}
}
