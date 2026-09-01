package grpcsvc

import (
	"context"

	"github.com/google/uuid"
)

const (
	inboxMain     = "main"
	inboxRequests = "requests"
)

// recipientInboxBucket classifies DM recipient inbox per text-chat.md §«Запросы сообщений».
// Friends and recipient-side contacts bypass the requests folder.
func recipientInboxBucket(ctx context.Context, callerProfile, recipientProfile uuid.UUID, friends ProfileFriendChecker, contacts ProfileContactChecker) (string, error) {
	if friends != nil {
		ok, err := friends.AreFriends(ctx, callerProfile, recipientProfile)
		if err != nil {
			return "", err
		}
		if ok {
			return inboxMain, nil
		}
	}
	if contacts != nil {
		ok, err := contacts.HasContact(ctx, recipientProfile, callerProfile)
		if err != nil {
			return "", err
		}
		if ok {
			return inboxMain, nil
		}
	}
	return inboxRequests, nil
}
