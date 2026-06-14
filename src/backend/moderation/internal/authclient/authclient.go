package authclient

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	authv1 "voice.app/voice/auth/v1"
)

// GRPCAccountStatus syncs account status via Auth internal RPC.
type GRPCAccountStatus struct {
	Client authv1.AuthServiceClient
}

func Dial(addr string) (*GRPCAccountStatus, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCAccountStatus{Client: authv1.NewAuthServiceClient(conn)}, nil
}

func (c *GRPCAccountStatus) SetAccountStatus(ctx context.Context, accountID uuid.UUID, status, reason string) error {
	if c == nil || c.Client == nil {
		return nil
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal", "true")
	_, err := c.Client.SetAccountStatus(ctx, &authv1.SetAccountStatusRequest{
		AccountId: accountID.String(),
		Status:    status,
		Reason:    reason,
	})
	return err
}
