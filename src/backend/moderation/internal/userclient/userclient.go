package userclient

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	userv1 "voice.app/voice/user/v1"
)

// GRPCProfiles resolves profile_id to account_id via User Service.
type GRPCProfiles struct {
	Client userv1.UserServiceClient
}

func Dial(addr string) (*GRPCProfiles, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCProfiles{Client: userv1.NewUserServiceClient(conn)}, nil
}

func (c *GRPCProfiles) AccountIDForProfile(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if c == nil || c.Client == nil {
		return uuid.Nil, nil
	}
	resp, err := c.Client.GetProfile(ctx, &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()},
	})
	if err != nil {
		return uuid.Nil, err
	}
	p := resp.GetProfile()
	if p == nil {
		return uuid.Nil, nil
	}
	aid := strings.TrimSpace(p.GetAccountId())
	if aid == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(aid)
}
