package grpcsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/correlation"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	authv1 "voice/backend/user/pb/voice/auth/v1"
)

const deletedAccountsTimeout = 2 * time.Second

// DeletedAccountsChecker is User's narrow, synchronous Auth authority for
// account deletion visibility. Callers must treat every error as fail-closed.
type DeletedAccountsChecker interface {
	DeletedAmong(ctx context.Context, accountIDs []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

// AuthGRPCDeletedAccounts calls Auth's FilterDeletedAccountIDs S2S RPC.
type AuthGRPCDeletedAccounts struct {
	Client  authv1.AuthServiceClient
	Timeout time.Duration
}

// NewAuthGRPCDeletedAccounts constructs the bounded Auth deletion checker.
func NewAuthGRPCDeletedAccounts(cc grpc.ClientConnInterface) *AuthGRPCDeletedAccounts {
	if cc == nil {
		return nil
	}
	return &AuthGRPCDeletedAccounts{
		Client:  authv1.NewAuthServiceClient(cc),
		Timeout: deletedAccountsTimeout,
	}
}

// DeletedAmong returns only requested account IDs. Invalid, duplicate, or
// unrequested IDs in Auth's response are protocol violations and fail closed.
func (c *AuthGRPCDeletedAccounts) DeletedAmong(ctx context.Context, accountIDs []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	deleted := make(map[uuid.UUID]struct{})
	if len(accountIDs) == 0 {
		return deleted, nil
	}
	if c == nil || c.Client == nil {
		return nil, fmt.Errorf("auth deleted-account checker is not configured")
	}

	requested := make(map[uuid.UUID]struct{}, len(accountIDs))
	requestIDs := make([]string, 0, len(accountIDs))
	for _, id := range accountIDs {
		if _, ok := requested[id]; ok {
			continue
		}
		requested[id] = struct{}{}
		requestIDs = append(requestIDs, id.String())
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = deletedAccountsTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	callCtx = correlation.OutgoingGRPC(callCtx, "")
	callCtx = metadata.AppendToOutgoingContext(callCtx, authctx.HeaderInternalCaller, "user")
	resp, err := c.Client.FilterDeletedAccountIDs(callCtx, &authv1.FilterDeletedAccountIDsRequest{
		AccountIds: requestIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("auth FilterDeletedAccountIDs: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("auth FilterDeletedAccountIDs returned an empty response")
	}
	for _, raw := range resp.GetDeletedAccountIds() {
		id, err := uuid.Parse(raw)
		if err != nil || raw != id.String() {
			return nil, fmt.Errorf("auth FilterDeletedAccountIDs returned invalid account id")
		}
		if _, ok := requested[id]; !ok {
			return nil, fmt.Errorf("auth FilterDeletedAccountIDs returned unrequested account id")
		}
		if _, duplicate := deleted[id]; duplicate {
			return nil, fmt.Errorf("auth FilterDeletedAccountIDs returned duplicate account id")
		}
		deleted[id] = struct{}{}
	}
	return deleted, nil
}

func (s *UserGRPC) filterDeletedAccountProfiles(ctx context.Context, rows []*store.ProfileRow) ([]*store.ProfileRow, error) {
	if len(rows) == 0 || s.DeletedAccounts == nil {
		return rows, nil
	}
	accountIDs := make([]uuid.UUID, 0, len(rows))
	requested := make(map[uuid.UUID]struct{}, len(rows))
	for _, row := range rows {
		if row != nil {
			accountIDs = append(accountIDs, row.AccountID)
			requested[row.AccountID] = struct{}{}
		}
	}
	deleted, err := s.DeletedAccounts.DeletedAmong(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	for accountID := range deleted {
		if _, ok := requested[accountID]; !ok {
			return nil, fmt.Errorf("deleted-account checker returned unrequested account id")
		}
	}
	visible := make([]*store.ProfileRow, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			if _, hidden := deleted[row.AccountID]; hidden {
				continue
			}
		}
		visible = append(visible, row)
	}
	return visible, nil
}

func deletedAccountCheckUnavailable(err error) error {
	_ = err
	return status.Error(codes.Unavailable, "account visibility unavailable")
}
