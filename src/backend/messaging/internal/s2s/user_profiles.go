package s2s

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/correlation"

	userv1 "voice.app/voice/user/v1"
)

const profileOwnerLookupTimeout = 2 * time.Second

// UserGRPCProfiles resolves profile_id → account_id via UserService's internal ownership RPC.
type UserGRPCProfiles struct {
	Client userv1.UserServiceClient
}

func (u *UserGRPCProfiles) AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if u == nil || u.Client == nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "user service not configured")
	}
	callCtx, cancel := context.WithTimeout(ctx, profileOwnerLookupTimeout)
	defer cancel()
	callMD := metadata.Pairs("x-voice-internal-caller", "messaging")
	if requestID := correlation.FromGRPC(ctx); requestID != "" {
		callMD.Append(correlation.GRPCMetadataKey, requestID)
	}
	callCtx = metadata.NewOutgoingContext(callCtx, callMD)
	resp, err := u.Client.ResolveAccountIDForProfile(callCtx, &userv1.ResolveAccountIDForProfileRequest{ProfileId: profileID.String()})
	if err != nil {
		return uuid.Nil, err
	}
	if resp == nil {
		return uuid.Nil, status.Error(codes.Internal, "profile owner response missing account_id")
	}
	aid := strings.TrimSpace(resp.GetAccountId())
	if aid == "" {
		return uuid.Nil, status.Error(codes.Internal, "profile owner response missing account_id")
	}
	out, err := uuid.Parse(aid)
	if err != nil {
		return uuid.Nil, status.Error(codes.Internal, "invalid account_id on profile owner response")
	}
	return out, nil
}
