package main

import (
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"testing"
)

func TestReleaseBootstrapRejectsBeforeRealtimeDependencies(t *testing.T) {
	if os.Getenv("R324_REALTIME_BAD_POLICY_CHILD") == "true" {
		buildSourceSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestReleaseBootstrapRejectsBeforeRealtimeDependencies$")
	cmd.Env = append(os.Environ(), "R324_REALTIME_BAD_POLICY_CHILD=true", "VOICE_RELEASE_PROFILE=free-invite-alpha", "VOICE_RELEASE_POLICY_FILE=relative.json", "VOICE_RELEASE_POLICY_SHA256=bad", "DATABASE_URL=invalid")
	output, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(output), "realtime release policy:")
	require.NotContains(t, string(output), "invalid realtime configuration")
}
