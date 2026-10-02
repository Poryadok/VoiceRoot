//go:build !linux

package federationmedia

import (
	"testing"
	"time"
	"voice/backend/federation/mediaauthority"
)

func startControllerProcess(t *testing.T, _ string, _ fixtureParameters, _ *projectionPublisher) func() time.Time {
	t.Helper()
	t.Skip("owned Linux controller/SFU fixture required")
	return nil
}

func startNodeMediaProcess(t *testing.T, _ string, _ fixtureParameters, _ *projectionPublisher) func(mediaauthority.Grant, time.Duration) (string, int) {
	t.Helper()
	t.Skip("owned Linux node media fixture required")
	return nil
}
