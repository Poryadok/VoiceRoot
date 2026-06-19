package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	authv1 "voice.app/voice/auth/v1"
)

// GRPCAuthPhoneHashLookup resolves phone hashes via Auth internal RPC.
type GRPCAuthPhoneHashLookup struct {
	Client authv1.AuthServiceClient
}

func NewGRPCAuthPhoneHashLookup(cc grpc.ClientConnInterface) *GRPCAuthPhoneHashLookup {
	if cc == nil {
		return nil
	}
	return &GRPCAuthPhoneHashLookup{Client: authv1.NewAuthServiceClient(cc)}
}

func (a *GRPCAuthPhoneHashLookup) ProfileIDsByPhoneHashes(ctx context.Context, hashes []string) (map[string]uuid.UUID, error) {
	out := make(map[string]uuid.UUID)
	if a == nil || a.Client == nil || len(hashes) == 0 {
		return out, nil
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal", "true")
	resp, err := a.Client.ResolvePhoneHashes(ctx, &authv1.ResolvePhoneHashesRequest{PhoneHashes: hashes})
	if err != nil {
		return nil, err
	}
	for _, m := range resp.GetMatches() {
		hash := m.GetPhoneHash()
		pid, err := uuid.Parse(m.GetProfileId())
		if err != nil || hash == "" {
			continue
		}
		out[hash] = pid
	}
	return out, nil
}
