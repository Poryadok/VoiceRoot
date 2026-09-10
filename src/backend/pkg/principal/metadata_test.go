package principal

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestIncomingMetadata_RequiresOneBearerAndRequestID(t *testing.T) {
	metadataValue, err := IncomingMetadata(metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer signed-principal",
		"x-request-id", "request-1",
	)))
	if err != nil {
		t.Fatal(err)
	}
	if metadataValue.BearerToken != "signed-principal" || metadataValue.RequestID != "request-1" {
		t.Fatalf("unexpected metadata: %#v", metadataValue)
	}
}

func TestIncomingMetadata_RejectsAmbiguousOrRawIdentityHeaders(t *testing.T) {
	tests := []struct {
		name  string
		pairs []string
	}{
		{name: "missing authorization", pairs: []string{"x-request-id", "request-1"}},
		{name: "duplicate authorization", pairs: []string{"authorization", "Bearer one", "authorization", "Bearer two", "x-request-id", "request-1"}},
		{name: "non bearer", pairs: []string{"authorization", "Basic value", "x-request-id", "request-1"}},
		{name: "duplicate request id", pairs: []string{"authorization", "Bearer token", "x-request-id", "request-1", "x-request-id", "request-2"}},
		{name: "raw profile", pairs: []string{"authorization", "Bearer token", "x-request-id", "request-1", "x-voice-profile-id", "attacker"}},
		{name: "raw caller", pairs: []string{"authorization", "Bearer token", "x-request-id", "request-1", "x-voice-internal-caller", "attacker"}},
		{name: "legacy internal", pairs: []string{"authorization", "Bearer token", "x-request-id", "request-1", "x-voice-internal", "true"}},
		{name: "legacy account type", pairs: []string{"authorization", "Bearer token", "x-request-id", "request-1", "x-voice-account-type", "guest"}},
		{name: "unprefixed raw identity", pairs: []string{"authorization", "Bearer token", "x-request-id", "request-1", "x-profile-id", "attacker"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := IncomingMetadata(metadata.NewIncomingContext(context.Background(), metadata.Pairs(tt.pairs...)))
			if !errors.Is(err, ErrInvalidPrincipalMetadata) {
				t.Fatalf("error = %v, want generic metadata rejection", err)
			}
		})
	}
}
