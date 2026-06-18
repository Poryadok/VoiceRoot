package clients

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/privacy"
	"voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/storyevents"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
	userv1 "voice.app/voice/user/v1"
)

// WireGRPC attaches optional upstream clients to StoryGRPC and returns a FileDeleter for purge jobs.
func WireGRPC(logger *slog.Logger, svc *grpcsvc.StoryGRPC) jobs.FileDeleter {
	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		pub, err := storyevents.NewJetStreamPublisher(natsURL)
		if err != nil {
			if logger != nil {
				logger.Error("story events publisher failed", slog.String("error", err.Error()))
			}
		} else {
			pub.Logger = logger
			svc.Events = pub
		}
	} else {
		svc.Events = storyevents.NoopPublisher{}
	}

	if addr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR")); addr != "" {
		if conn, err := dial(addr); err == nil {
			svc.Chat = &chatClient{client: chatv1.NewChatServiceClient(conn)}
		} else if logger != nil {
			logger.Error("story chat dial failed", slog.String("error", err.Error()))
		}
	}

	if addr := strings.TrimSpace(os.Getenv("MESSAGING_GRPC_ADDR")); addr != "" {
		if conn, err := dial(addr); err == nil {
			svc.Messaging = &messagingClient{client: messagingv1.NewMessagingServiceClient(conn)}
		} else if logger != nil {
			logger.Error("story messaging dial failed", slog.String("error", err.Error()))
		}
	}

	if addr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); addr != "" {
		if conn, err := dial(addr); err == nil {
			svc.Privacy = &UserStoryPrivacy{client: userv1.NewUserServiceClient(conn)}
		} else if logger != nil {
			logger.Error("story user dial failed", slog.String("error", err.Error()))
		}
	}

	if addr := strings.TrimSpace(os.Getenv("SUBSCRIPTION_GRPC_ADDR")); addr != "" {
		if conn, err := dial(addr); err == nil {
			svc.Subscriptions = &PremiumChecker{client: subscriptionv1.NewSubscriptionServiceClient(conn)}
		} else if logger != nil {
			logger.Error("story subscription dial failed", slog.String("error", err.Error()))
		}
	}

	var deleter jobs.FileDeleter
	if addr := strings.TrimSpace(os.Getenv("FILE_GRPC_ADDR")); addr != "" {
		if conn, err := dial(addr); err == nil {
			fileClient := filev1.NewFileServiceClient(conn)
			deleter = &FileDeleter{client: fileClient}
			svc.Files = &FileMetadataReader{client: fileClient}
		} else if logger != nil {
			logger.Error("story file dial failed", slog.String("error", err.Error()))
		}
	}
	return deleter
}

func dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(insecure.NewCredentials()))
}

type chatClient struct {
	client chatv1.ChatServiceClient
}

func (c *chatClient) CreateDM(ctx context.Context, in *chatv1.CreateDMRequest) (*chatv1.CreateDMResponse, error) {
	return c.client.CreateDM(ctx, in)
}

type messagingClient struct {
	client messagingv1.MessagingServiceClient
}

func (m *messagingClient) SendMessage(ctx context.Context, in *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	return m.client.SendMessage(ctx, in)
}

// UserStoryPrivacy loads show_stories audience from User Service.
type UserStoryPrivacy struct {
	client userv1.UserServiceClient
}

func (u *UserStoryPrivacy) ShowStoriesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.client == nil {
		return privacy.FriendsAndFoF(), nil
	}
	resp, err := u.client.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetShowStories()), nil
}

// PremiumChecker reports active personal subscription for anonymous story views.
type PremiumChecker struct {
	client subscriptionv1.SubscriptionServiceClient
}

func (p *PremiumChecker) HasActivePremium(ctx context.Context, accountID uuid.UUID) (bool, error) {
	if p == nil || p.client == nil {
		return false, nil
	}
	resp, err := p.client.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{
		AccountId: accountID.String(),
	})
	if err != nil {
		return false, err
	}
	sub := resp.GetSubscription()
	if sub == nil {
		return false, nil
	}
	switch strings.TrimSpace(sub.GetStatus()) {
	case "active", "grace_period":
		return true, nil
	default:
		return false, nil
	}
}

// FileDeleter removes orphaned story media via File Service.
type FileDeleter struct {
	client filev1.FileServiceClient
}

func (f *FileDeleter) DeleteFile(ctx context.Context, fileID string) error {
	if f == nil || f.client == nil {
		return nil
	}
	_, err := f.client.DeleteFile(ctx, &filev1.DeleteFileRequest{FileId: fileID})
	return err
}

// FileMetadataReader loads file metadata for story validation.
type FileMetadataReader struct {
	client filev1.FileServiceClient
}

func (f *FileMetadataReader) GetFileDurationSeconds(ctx context.Context, fileID uuid.UUID) (int32, error) {
	if f == nil || f.client == nil {
		return 0, nil
	}
	resp, err := f.client.GetFileMetadata(ctx, &filev1.GetFileMetadataRequest{FileId: fileID.String()})
	if err != nil {
		return 0, err
	}
	meta := resp.GetFileMetadata()
	if meta == nil || meta.DurationSeconds == nil {
		return 0, nil
	}
	return meta.GetDurationSeconds(), nil
}
