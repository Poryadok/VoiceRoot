package main

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"strings"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/pushenrich"
	"voice/backend/notification/internal/recipientauth"
	"voice/backend/pkg/notificationrouting"
)

func newNotificationRecipientAuthority() (*recipientauth.Authorizer, func(), error) {
	var connections []*grpc.ClientConn
	closeAll := func() {
		for _, c := range connections {
			_ = c.Close()
		}
	}
	clients := make([]*grpc.ClientConn, 0, 3)
	for _, key := range []string{"CHAT_GRPC_ADDR", "USER_GRPC_ADDR", "SOCIAL_GRPC_ADDR"} {
		addr := strings.TrimSpace(os.Getenv(key))
		if addr == "" {
			closeAll()
			return nil, func() {}, fmt.Errorf("%s is required for notification recipient routing", key)
		}
		c, e := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if e != nil {
			closeAll()
			return nil, func() {}, e
		}
		connections = append(connections, c)
		clients = append(clients, c)
	}
	return &recipientauth.Authorizer{Chat: chatv1.NewChatServiceClient(clients[0]), Profiles: userv1.NewUserServiceClient(clients[1]), Blocks: socialv1.NewSocialServiceClient(clients[2])}, closeAll, nil
}

func bindMessageRoutingResponder(ctx context.Context, nc *nats.Conn, members chatmembers.Lister, pusher *dispatch.MessagePusher, enrich pushenrich.Resolver, authority notificationRecipientAuthorizer) (*nats.Subscription, error) {
	sub, e := nc.QueueSubscribe(notificationrouting.Subject, notificationrouting.Queue, func(msg *nats.Msg) {
		if !notificationrouting.ValidReply(msg.Reply) {
			return
		}
		routeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		targets, routeErr := messageRoutingDecision(routeCtx, msg.Data, members, pusher, enrich, authority)
		if routeErr != nil {
			targets = nil
		}
		wire, encodeErr := notificationrouting.Encode(msg.Data, targets, routeErr != nil)
		if encodeErr != nil || len(wire) > notificationrouting.MaxBytes {
			return
		}
		_ = msg.Respond(wire)
	})
	if e != nil {
		return nil, e
	}
	if e = sub.SetPendingLimits(64, 8*notificationrouting.MaxBytes); e != nil {
		_ = sub.Unsubscribe()
		return nil, e
	}
	if e = nc.FlushTimeout(5 * time.Second); e != nil {
		_ = sub.Unsubscribe()
		return nil, e
	}
	// Core subscriptions report broker permission failures asynchronously. The
	// flush orders the server's SUB result before readiness can be published.
	if e = nc.LastError(); e != nil {
		_ = sub.Unsubscribe()
		return nil, e
	}
	return sub, nil
}
