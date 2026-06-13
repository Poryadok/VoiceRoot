package pushenrich

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	messagingv1 "voice.app/voice/messaging/v1"
	userv1 "voice.app/voice/user/v1"
)

// Resolver loads message preview text and sender display labels for push copy.
type Resolver interface {
	MessagePreview(ctx context.Context, messageID string) (string, error)
	SenderLabel(ctx context.Context, profileID string) (string, error)
}

// GRPCResolver uses Messaging GetMessage and User GetProfile (S2S).
type GRPCResolver struct {
	messages messagingv1.MessagingServiceClient
	users    userv1.UserServiceClient
}

func NewGRPCResolver(messagingAddr, userAddr string) (*GRPCResolver, error) {
	messagingAddr = strings.TrimSpace(messagingAddr)
	userAddr = strings.TrimSpace(userAddr)
	if messagingAddr == "" || userAddr == "" {
		return nil, fmt.Errorf("push enrich: messaging and user grpc addrs required")
	}
	msgConn, err := grpc.NewClient(messagingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	userConn, err := grpc.NewClient(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = msgConn.Close()
		return nil, err
	}
	return &GRPCResolver{
		messages: messagingv1.NewMessagingServiceClient(msgConn),
		users:    userv1.NewUserServiceClient(userConn),
	}, nil
}

func (r *GRPCResolver) MessagePreview(ctx context.Context, messageID string) (string, error) {
	if r == nil || r.messages == nil {
		return "", fmt.Errorf("push enrich: messaging unavailable")
	}
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "", nil
	}
	resp, err := r.messages.GetMessage(ctx, &messagingv1.GetMessageRequest{MessageId: messageID})
	if err != nil {
		return "", err
	}
	msg := resp.GetMessage()
	if msg == nil {
		return "", nil
	}
	return msg.GetContent(), nil
}

func (r *GRPCResolver) SenderLabel(ctx context.Context, profileID string) (string, error) {
	if r == nil || r.users == nil {
		return "", fmt.Errorf("push enrich: user service unavailable")
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return "", nil
	}
	resp, err := r.users.GetProfile(ctx, &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID},
	})
	if err != nil {
		return "", err
	}
	prof := resp.GetProfile()
	if prof == nil {
		return "", nil
	}
	if label := strings.TrimSpace(prof.GetDisplayName()); label != "" {
		return label, nil
	}
	user := strings.TrimSpace(prof.GetUsername())
	if user == "" {
		return "", nil
	}
	disc := strings.TrimSpace(prof.GetDiscriminator())
	if disc != "" {
		return user + "#" + disc, nil
	}
	return user, nil
}

// NoopResolver returns empty strings (generic push copy).
type NoopResolver struct{}

func (NoopResolver) MessagePreview(context.Context, string) (string, error) {
	return "", nil
}

func (NoopResolver) SenderLabel(context.Context, string) (string, error) {
	return "", nil
}
