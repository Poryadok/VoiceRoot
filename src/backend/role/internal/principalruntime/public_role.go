package principalruntime

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
)

func (r *Runtime) verifyPublicRole(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if r == nil || !principalgrpc.IsDelegatedRoleMethod(method) {
		return principal.Principal{}, errors.New("delegated Role receiver inactive")
	}
	config := principal.VerifyConfig{ExpectedAudience: "role", ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: r.resolver.Resolve, ReplayGuard: r.recordReplay}
	if principalgrpc.IsSpaceRoleRead(method) {
		config.ExpectedIssuer = "space"
		verified, err := principal.VerifyService(ctx, token, config)
		if err == nil {
			return verified, nil
		}
		if principalgrpc.IsUnavailable(err) {
			return principal.Principal{}, err
		}
	}
	if !r.delegatedUserEnabled || r.sessionEpochChecker == nil {
		return principal.Principal{}, errors.New("delegated Role receiver inactive")
	}
	config.ExpectedIssuer = "gateway"
	config.SessionEpochChecker = func(ctx context.Context, account string, epoch int64) error {
		err := r.sessionEpochChecker(ctx, account, epoch)
		if status.Code(err) == codes.Unavailable || status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Canceled {
			return principalgrpc.Unavailable(err)
		}
		return err
	}
	return principal.VerifyDelegatedUser(ctx, token, config)
}
