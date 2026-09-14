package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/guestguard"
)

// presenceViewer delegates viewer-aware presence privacy decisions to User.
// Realtime deliberately does not reproduce privacy audience rules locally.
type presenceViewer interface {
	PresenceForViewer(ctx context.Context, targetProfileID, viewerProfileID, viewerAccountType string) (viewerPresence, error)
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

func (g *grpcPresenceViewer) PresenceForViewer(ctx context.Context, targetProfileID, viewerProfileID, viewerAccountType string) (viewerPresence, error) {
	if g == nil || g.client == nil {
		return viewerPresence{}, fmt.Errorf("user presence viewer is not configured")
	}
	targetProfileID = strings.TrimSpace(targetProfileID)
	viewerProfileID = strings.TrimSpace(viewerProfileID)
	if targetProfileID == "" || viewerProfileID == "" {
		return viewerPresence{}, fmt.Errorf("presence viewer requires target and viewer profile IDs")
	}
	if strings.TrimSpace(viewerAccountType) == "" {
		viewerAccountType = guestguard.AccountTypeRegular
	}
	ctx = metadata.AppendToOutgoingContext(ctx,
		grpcMDVoiceProfileID, viewerProfileID,
		guestguard.HeaderAccountType, viewerAccountType,
	)
	resp, err := g.client.GetPresence(ctx, &userv1.GetPresenceRequest{ProfileId: targetProfileID})
	if err != nil {
		return viewerPresence{}, err
	}
	status := resp.GetPresenceStatus()
	if status == nil {
		return viewerPresence{}, fmt.Errorf("user presence viewer received empty response")
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
	snapshot, err := viewer.PresenceForViewer(ctx, targetProfileID, reg.profileID, reg.accountType)
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
