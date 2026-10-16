package main

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
)

// privacyFixture exists only in the disposable writer probe's network namespace.
// The production Story image still performs its real, fail-closed User RPC.
type privacyFixture struct {
	userv1.UnimplementedUserServiceServer
	profile string
	calls   atomic.Int64
}

func (p *privacyFixture) GetPrivacySettings(ctx context.Context, req *userv1.GetPrivacySettingsRequest) (*userv1.GetPrivacySettingsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	callers := md.Get("x-voice-internal-caller")
	if req.GetProfileId() != p.profile || len(callers) != 1 || callers[0] != "story" {
		return nil, status.Error(codes.PermissionDenied, "fixture identity rejected")
	}
	p.calls.Add(1)
	return &userv1.GetPrivacySettingsResponse{PrivacySettings: &userv1.PrivacySettings{
		ProfileId: p.profile, ShowStories: &userv1.PrivacyAudience{Friends: true},
	}}, nil
}

func startPrivacyFixture(author uuid.UUID) (*privacyFixture, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:9091")
	if err != nil {
		return nil, nil, err
	}
	fixture := &privacyFixture{profile: author.String()}
	server := grpc.NewServer()
	userv1.RegisterUserServiceServer(server, fixture)
	go func() { _ = server.Serve(listener) }()
	return fixture, server.Stop, nil
}
