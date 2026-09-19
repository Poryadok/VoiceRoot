package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/guestguard"
)

// presenceViewer delegates viewer-aware presence privacy decisions to User.
// Realtime deliberately does not reproduce privacy audience rules locally.
type presenceViewer interface {
	PresenceForViewer(ctx context.Context, request presenceViewerRequest) (viewerPresence, error)
}

type presenceViewerRequest struct {
	TargetProfileID    string
	ViewerAccountID    string
	ViewerProfileID    string
	ViewerAccountType  string
}

type viewerPresence struct {
	Status       string
	CustomStatus string
	LastSeen     *time.Time
}

type grpcPresenceViewer struct {
	client userv1.UserServiceClient
}

func newGRPCPresenceViewer(cc *grpc.ClientConn) *grpcPresenceViewer {
	if cc == nil {
		return nil
	}
	return &grpcPresenceViewer{client: userv1.NewUserServiceClient(cc)}
}

func (g *grpcPresenceViewer) PresenceForViewer(ctx context.Context, request presenceViewerRequest) (viewerPresence, error) {
	if g == nil || g.client == nil {
		return viewerPresence{}, fmt.Errorf("user presence viewer is not configured")
	}
	targetProfileID, err := presenceIdentityUUID(request.TargetProfileID)
	if err != nil {
		return viewerPresence{}, err
	}
	viewerAccountID, err := presenceIdentityUUID(request.ViewerAccountID)
	if err != nil {
		return viewerPresence{}, err
	}
	viewerProfileID, err := presenceIdentityUUID(request.ViewerProfileID)
	if err != nil {
		return viewerPresence{}, err
	}
	accountType := request.ViewerAccountType
	if accountType != guestguard.AccountTypeRegular && accountType != guestguard.AccountTypeGuest {
		return viewerPresence{}, fmt.Errorf("presence viewer requires a known account type")
	}
	ctx = presenceIdentityContext(ctx, viewerAccountID, viewerProfileID, accountType)
	resp, err := g.client.GetPresence(ctx, &userv1.GetPresenceRequest{ProfileId: targetProfileID})
	if err != nil {
		return viewerPresence{}, err
	}
	status := resp.GetPresenceStatus()
	if status == nil {
		return viewerPresence{}, fmt.Errorf("user presence viewer received empty response")
	}
	responseProfileID, err := presenceIdentityUUID(status.GetProfileId())
	if err != nil || responseProfileID != targetProfileID {
		return viewerPresence{}, fmt.Errorf("user presence response does not match target")
	}
	out := viewerPresence{Status: status.GetStatus(), CustomStatus: status.GetCustomStatus()}
	if lastSeen := status.GetLastSeen(); lastSeen != nil && lastSeen.IsValid() {
		at := lastSeen.AsTime().UTC()
		out.LastSeen = &at
	}
	return out, nil
}

// presenceFanoutPayload turns User's already filtered snapshot into the sparse
// WebSocket payload. Invisible transitions conceal a contemporaneous timestamp.
func presenceFanoutPayload(ctx context.Context, viewer presenceViewer, targetProfileID, sourceStatus string, reg *connReg, chatID string) (json.RawMessage, error) {
	if viewer == nil || reg == nil {
		return nil, fmt.Errorf("presence privacy viewer is not configured")
	}
	snapshot, err := viewer.PresenceForViewer(ctx, presenceViewerRequest{
		TargetProfileID: targetProfileID,
		ViewerAccountID: reg.accountID,
		ViewerProfileID: reg.profileID,
		ViewerAccountType: reg.accountType,
	})
	if err != nil {
		return nil, err
	}
	body := map[string]any{"profile_id": targetProfileID}
	invisible := strings.EqualFold(strings.TrimSpace(sourceStatus), "invisible")
	if !invisible && snapshot.Status != "" {
		body["status"] = snapshot.Status
		if snapshot.CustomStatus != "" {
			body["custom_status"] = snapshot.CustomStatus
		}
	}
	if !invisible && snapshot.LastSeen != nil {
		body["last_seen"] = snapshot.LastSeen.UTC().Format(time.RFC3339)
	}
	if chatID != "" {
		body["chat_id"] = chatID
	}
	return json.Marshal(body)
}
