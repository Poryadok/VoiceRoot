package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
)

const defaultE2EPreKeyGateTimeout = 3 * time.Second

// E2EPreKeyGate verifies DM members have uploaded Signal pre-key bundles before E2E enable.
type E2EPreKeyGate interface {
	EnsureAllMembersHavePreKeys(ctx context.Context, memberProfileIDs []uuid.UUID) error
}

// MessagingE2EPreKeyGate calls Messaging GetPreKeyBundle for each member profile.
type MessagingE2EPreKeyGate struct {
	Client  messagingv1.MessagingServiceClient
	Timeout time.Duration
}

func NewMessagingE2EPreKeyGate(client messagingv1.MessagingServiceClient) *MessagingE2EPreKeyGate {
	return &MessagingE2EPreKeyGate{
		Client:  client,
		Timeout: defaultE2EPreKeyGateTimeout,
	}
}

func (g *MessagingE2EPreKeyGate) EnsureAllMembersHavePreKeys(ctx context.Context, memberProfileIDs []uuid.UUID) error {
	if g == nil || g.Client == nil {
		return errors.New("e2e pre-key gate: messaging client not configured")
	}
	callCtx := ctx
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		callCtx = metadata.NewOutgoingContext(ctx, md.Copy())
	}
	if g.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(callCtx, g.Timeout)
		defer cancel()
	}
	for _, profileID := range memberProfileIDs {
		resp, err := g.Client.GetPreKeyBundle(callCtx, &messagingv1.GetPreKeyBundleRequest{
			ProfileId: profileID.String(),
		})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return status.Error(codes.FailedPrecondition, "all chat members must upload pre-key bundles before enabling e2e")
			}
			return err
		}
		if resp == nil || resp.GetBundle() == "" {
			return status.Error(codes.FailedPrecondition, "all chat members must upload pre-key bundles before enabling e2e")
		}
	}
	return nil
}
