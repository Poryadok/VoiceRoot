package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/pkg/grpcclient"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	socialv1 "voice.app/voice/social/v1"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

type grpcClients struct {
	user      userv1.UserServiceClient
	social    socialv1.SocialServiceClient
	chat      chatv1.ChatServiceClient
	messaging messagingv1.MessagingServiceClient
	voice     callsv1.VoiceServiceClient
	file      filev1.FileServiceClient
	space     spacev1.SpaceServiceClient
}

type transcoder struct {
	clients grpcClients
}

func grpcClientsFromEnv() *grpcClients {
	addrFor := func(namespace string) string {
		specific := strings.TrimSpace(os.Getenv("GATEWAY_" + strings.ToUpper(namespace) + "_GRPC_ADDR"))
		if specific != "" {
			return specific
		}
		var urls map[string]string
		loadJSONEnv("GATEWAY_GRPC_UPSTREAMS_JSON", &urls)
		return strings.TrimSpace(urls[namespace])
	}

	dial := func(addr string) (*grpc.ClientConn, error) {
		addr = grpcclient.DialTarget(addr)
		if addr == "" {
			return nil, nil
		}
		return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	clients := &grpcClients{}
	if conn, err := dial(addrFor("users")); err != nil {
		log.Printf("gateway grpc dial users: %v", err)
	} else if conn != nil {
		clients.user = userv1.NewUserServiceClient(conn)
	}
	if conn, err := dial(addrFor("friends")); err != nil {
		log.Printf("gateway grpc dial friends: %v", err)
	} else if conn != nil {
		clients.social = socialv1.NewSocialServiceClient(conn)
	}
	if conn, err := dial(addrFor("chats")); err != nil {
		log.Printf("gateway grpc dial chats: %v", err)
	} else if conn != nil {
		clients.chat = chatv1.NewChatServiceClient(conn)
	}
	if conn, err := dial(addrFor("messages")); err != nil {
		log.Printf("gateway grpc dial messages: %v", err)
	} else if conn != nil {
		clients.messaging = messagingv1.NewMessagingServiceClient(conn)
	}
	if conn, err := dial(addrFor("voice")); err != nil {
		log.Printf("gateway grpc dial voice: %v", err)
	} else if conn != nil {
		clients.voice = callsv1.NewVoiceServiceClient(conn)
	}
	if conn, err := dial(addrFor("files")); err != nil {
		log.Printf("gateway grpc dial files: %v", err)
	} else if conn != nil {
		clients.file = filev1.NewFileServiceClient(conn)
	}
	if conn, err := dial(addrFor("spaces")); err != nil {
		log.Printf("gateway grpc dial spaces: %v", err)
	} else if conn != nil {
		clients.space = spacev1.NewSpaceServiceClient(conn)
	}
	if clients.user == nil && clients.social == nil && clients.chat == nil && clients.messaging == nil && clients.voice == nil && clients.file == nil && clients.space == nil {
		return nil
	}
	return clients
}

func newTranscoder(clients *grpcClients) *transcoder {
	if clients == nil {
		return nil
	}
	return &transcoder{clients: *clients}
}

func (t *transcoder) serveNamespace(w http.ResponseWriter, r *http.Request, namespace string) bool {
	if t == nil {
		return false
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/"+namespace)
	rest = strings.TrimPrefix(rest, "/")
	switch namespace {
	case "users":
		if t.clients.user == nil {
			return false
		}
		return t.serveUsers(w, r, rest)
	case "friends":
		if t.clients.social == nil {
			return false
		}
		return t.serveFriends(w, r, rest)
	case "chats":
		if t.clients.chat == nil {
			return false
		}
		return t.serveChats(w, r, rest)
	case "messages":
		if t.clients.messaging == nil {
			return false
		}
		return t.serveMessages(w, r, rest)
	case "voice":
		if t.clients.voice == nil {
			return false
		}
		return t.serveVoice(w, r, rest)
	case "files":
		if t.clients.file == nil {
			return false
		}
		return t.serveFiles(w, r, rest)
	case "spaces":
		if t.clients.space == nil {
			return false
		}
		return t.serveSpaces(w, r, rest)
	case "invites":
		if t.clients.space == nil {
			return false
		}
		return t.serveInvites(w, r, rest)
	default:
		return false
	}
}
