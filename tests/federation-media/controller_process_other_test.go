//go:build !linux

package federationmedia

import (
	"testing"
	"time"
)

func startControllerProcess(t *testing.T, _ string, _ fixtureParameters, _ *projectionPublisher) func() time.Time {
	t.Helper()
	t.Skip("owned Linux controller/SFU fixture required")
	return nil
}
