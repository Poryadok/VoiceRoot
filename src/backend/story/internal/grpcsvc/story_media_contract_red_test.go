package grpcsvc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RED: implementation must replace metadata reconstruction with the protected
// File attestation before a media Story can be persisted or published.
func TestStoryMediaContractRED_usesProtectedAttestationAndNoForwardedIdentity(t *testing.T) {
	clientSource, err := os.ReadFile(filepath.Join("..", "clients", "clients.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(clientSource)
	if !strings.Contains(source, "ValidateStoryMedia") {
		t.Fatal("RED: Story must call File ValidateStoryMedia for photo/video")
	}
	if strings.Contains(source, "GetFileMetadata(ctx") {
		t.Fatal("RED: Story media admission must not reconstruct File policy via GetFileMetadata")
	}
	if strings.Contains(source, "x-voice-profile-id") || strings.Contains(source, "x-voice-user-id") {
		t.Fatal("RED: protected Story->File context must not forward raw identity metadata")
	}
}
