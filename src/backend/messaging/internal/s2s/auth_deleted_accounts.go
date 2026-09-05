package s2s

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authv1 "voice.app/voice/auth/v1"
)

const authDeletedAccountsTimeout = 2 * time.Second

// AuthGRPCDeletedAccounts implements Messaging's AccountDeletedChecker using
// Auth's internal batch account-lifecycle RPC.
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
	if a == nil || a.Client == nil {
		return nil, status.Error(codes.Unavailable, "auth deleted-account lookup unavailable")
	}
	out := make(map[uuid.UUID]struct{})
	if len(accountIDs) == 0 {
		return out, nil
	}
	raw := make([]string, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID == uuid.Nil {
			return nil, status.Error(codes.Unavailable, "auth deleted-account lookup malformed")
		}
		raw = append(raw, accountID.String())
	}
	callCtx, cancel := context.WithTimeout(ctx, authDeletedAccountsTimeout)
	defer cancel()
	callCtx = metadata.AppendToOutgoingContext(callCtx, "x-voice-internal", "true")
	resp, err := a.Client.FilterDeletedAccountIDs(callCtx, &authv1.FilterDeletedAccountIDsRequest{AccountIds: raw})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, status.Error(codes.Unavailable, "auth deleted-account lookup malformed")
	}
	requested := make(map[uuid.UUID]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		requested[accountID] = struct{}{}
	}
	for _, rawID := range resp.GetDeletedAccountIds() {
		accountID, err := uuid.Parse(rawID)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "auth deleted-account lookup malformed")
		}
		if _, ok := requested[accountID]; !ok {
			return nil, status.Error(codes.Unavailable, "auth deleted-account lookup malformed")
		}
		out[accountID] = struct{}{}
	}
	return out, nil
}
