package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"

	authv1 "voice.app/voice/auth/v1"
)

// AuthGRPCDeletedAccounts implements AccountDeletedChecker via Auth internal RPC.
type AuthGRPCDeletedAccounts struct {
	Client authv1.AuthServiceClient
}

func NewAuthGRPCDeletedAccounts(client authv1.AuthServiceClient) *AuthGRPCDeletedAccounts {
	if client == nil {
		return nil
	}
	return &AuthGRPCDeletedAccounts{Client: client}
}

func (a *AuthGRPCDeletedAccounts) DeletedAmong(ctx context.Context, accountIDs []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	out := make(map[uuid.UUID]struct{})
	if a == nil || a.Client == nil {
		return nil, errors.New("auth deleted-account client not configured")
	}
	if len(accountIDs) == 0 {
		return out, nil
	}
	raw := make([]string, 0, len(accountIDs))
	for _, id := range accountIDs {
		raw = append(raw, id.String())
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal", "true")
	resp, err := a.Client.FilterDeletedAccountIDs(ctx, &authv1.FilterDeletedAccountIDsRequest{AccountIds: raw})
	if err != nil {
		return nil, err
	}
	for _, sid := range resp.GetDeletedAccountIds() {
		id, err := uuid.Parse(sid)
		if err != nil {
			continue
		}
		out[id] = struct{}{}
	}
	return out, nil
}
