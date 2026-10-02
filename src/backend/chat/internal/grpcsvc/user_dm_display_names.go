package grpcsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	userv1 "voice.app/voice/user/v1"
)

type userDMPeerDisplayNamesClient interface {
	GetDMPeerDisplayNames(context.Context, *userv1.GetDMPeerDisplayNamesRequest, ...grpc.CallOption) (*userv1.GetDMPeerDisplayNamesResponse, error)
}

// UserGRPCDMPeerDisplayNames retrieves the title-only projection Chat may show
// for a DM peer after Chat has established membership.
type UserGRPCDMPeerDisplayNames struct{ Client userDMPeerDisplayNamesClient }

func NewUserGRPCDMPeerDisplayNames(client userv1.UserServiceClient) *UserGRPCDMPeerDisplayNames {
	return &UserGRPCDMPeerDisplayNames{Client: client}
}

func (u *UserGRPCDMPeerDisplayNames) LookupDMPeerDisplayNames(ctx context.Context, profileIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	if u == nil || u.Client == nil {
		return nil, fmt.Errorf("user service not configured")
	}
	if len(profileIDs) == 0 || len(profileIDs) > 100 {
		return nil, fmt.Errorf("invalid profile id count")
	}
	ids := make([]string, 0, len(profileIDs))
	seen := make(map[uuid.UUID]struct{}, len(profileIDs))
	for _, id := range profileIDs {
		if id == uuid.Nil {
			return nil, fmt.Errorf("invalid profile id")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id.String())
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal-caller", "chat")
	resp, err := u.Client.GetDMPeerDisplayNames(ctx, &userv1.GetDMPeerDisplayNamesRequest{ProfileIds: ids})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("user service returned empty display name response")
	}
	out := make(map[uuid.UUID]string, len(resp.GetDisplayNames()))
	for _, row := range resp.GetDisplayNames() {
		if row == nil {
			continue
		}
		id, err := uuid.Parse(strings.TrimSpace(row.GetProfileId()))
		if err != nil || id == uuid.Nil {
			return nil, fmt.Errorf("user service returned invalid profile id")
		}
		if name := strings.TrimSpace(row.GetDisplayName()); name != "" {
			out[id] = name
		}
	}
	return out, nil
}
