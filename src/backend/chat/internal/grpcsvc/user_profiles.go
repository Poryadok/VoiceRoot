package grpcsvc

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"
	"voice/backend/pkg/correlation"

	userv1 "voice.app/voice/user/v1"
)

// UserProfileLookup resolves profile_id → account_id for block checks (User Service source of truth).
type UserProfileLookup interface {
	AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error)
}

// LifecycleOwnerLookup resolves profile ownership for the ListChats deleted-peer gate.
// Unlike UserProfileLookup, it is an internal lifecycle seam and includes soft-deleted
// profiles without exposing public Profile data.
type LifecycleOwnerLookup interface {
	AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error)
}

type lifecycleOwnerClient interface {
	ResolveAccountIDForProfile(ctx context.Context, req *userv1.ResolveAccountIDForProfileRequest, opts ...grpc.CallOption) (*userv1.ResolveAccountIDForProfileResponse, error)
}

// UserGRPCLifecycleOwners resolves snapshot DM peer ownership through User's
// dedicated internal lifecycle RPC.
type UserGRPCLifecycleOwners struct {
	Client lifecycleOwnerClient
}

func (u *UserGRPCLifecycleOwners) AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if ctx == nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "missing lifecycle context")
	}
	if err := ctx.Err(); err != nil {
		return uuid.Nil, err
	}
	if profileID == uuid.Nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	if u == nil || u.Client == nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "user lifecycle owner lookup not configured")
	}

	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	md := metadata.Pairs(authctx.HeaderInternalCaller, "chat")
	if requestID := lifecycleRequestID(ctx); requestID != "" {
		md.Append(correlation.GRPCMetadataKey, requestID)
	}
	callCtx = metadata.NewOutgoingContext(callCtx, md)
	resp, err := u.Client.ResolveAccountIDForProfile(callCtx, &userv1.ResolveAccountIDForProfileRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return uuid.Nil, err
	}
	if resp == nil {
		return uuid.Nil, status.Error(codes.Internal, "user lifecycle owner lookup returned empty response")
	}
	accountID, err := uuid.Parse(resp.GetAccountId())
	if err != nil || accountID == uuid.Nil {
		return uuid.Nil, status.Error(codes.Internal, "user lifecycle owner lookup returned invalid account_id")
	}
	return accountID, nil
}

func lifecycleRequestID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(correlation.GRPCMetadataKey)
	if len(values) != 1 || !isSafeLifecycleRequestID(values[0]) {
		return ""
	}
	return values[0]
}

func isSafeLifecycleRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

// UserGRPCProfiles implements UserProfileLookup via UserService.GetProfile.
type UserGRPCProfiles struct {
	Client userv1.UserServiceClient
}

func (u *UserGRPCProfiles) AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if u == nil || u.Client == nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "user service not configured")
	}
	resp, err := u.Client.GetProfile(ctx, &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()},
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return uuid.Nil, status.Error(codes.NotFound, "profile not found")
		}
		return uuid.Nil, err
	}
	p := resp.GetProfile()
	if p == nil {
		return uuid.Nil, status.Error(codes.NotFound, "profile not found")
	}
	aid := strings.TrimSpace(p.GetAccountId())
	if aid == "" {
		return uuid.Nil, status.Error(codes.Internal, "profile missing account_id")
	}
	out, err := uuid.Parse(aid)
	if err != nil {
		return uuid.Nil, status.Error(codes.Internal, "invalid account_id on profile")
	}
	return out, nil
}
