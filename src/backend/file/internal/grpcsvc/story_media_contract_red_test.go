package grpcsvc

import (
	"os"
	"strings"
	"testing"
)

// RED: only a verified service:story principal may reach an atomic File-owned
// predicate. This intentionally fails until the protected listener/interceptor
// and handler are implemented.
func TestValidateStoryMediaContractRED_requiresVerifiedStoryPrincipalAndAtomicPredicate(t *testing.T) {
	source, err := os.ReadFile("file_grpc.go")
	if err != nil {
		t.Fatal(err)
	}
	code := string(source)
	for _, required := range []string{
		"func (s *FileGRPC) ValidateStoryMedia",
		"requireFileService(ctx, \"story\"",
		"FOR SHARE",
	} {
		if !strings.Contains(code, required) {
			t.Fatalf("RED: ValidateStoryMedia contract missing %q", required)
		}
	}
}
