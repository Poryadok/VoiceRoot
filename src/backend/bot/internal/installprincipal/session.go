package installprincipal

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"time"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

type SessionClient interface {
	GetBotSessionEpochFloor(context.Context, *authv1.GetBotSessionEpochFloorRequest, ...grpc.CallOption) (*authv1.GetBotSessionEpochFloorResponse, error)
}
type SessionChecker struct {
	Client SessionClient
	Issuer *principal.Issuer
}

func (c *SessionChecker) Check(ctx context.Context, account string, epoch int64) error {
	id, err := uuid.Parse(account)
	if err != nil || id == uuid.Nil || id.String() != account || epoch <= 0 || c == nil || c.Client == nil || c.Issuer == nil {
		return ErrAdministrator
	}
	req := &authv1.GetBotSessionEpochFloorRequest{AccountId: account}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return err
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: authv1.AuthService_GetBotSessionEpochFloor_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return err
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	resp, err := c.Client.GetBotSessionEpochFloor(call, req)
	if err != nil || resp == nil || len(resp.ProtoReflect().GetUnknown()) != 0 || resp.GetSessionEpochFloor() <= 0 {
		return errors.New("bot session floor unavailable")
	}
	if epoch < resp.GetSessionEpochFloor() {
		return ErrAdministrator
	}
	return nil
}
