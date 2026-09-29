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
	bucket, _, err := recipientInboxClassification(ctx, callerProfile, recipientProfile, friends, contacts)
	return bucket, err
}

func recipientInboxClassification(ctx context.Context, callerProfile, recipientProfile uuid.UUID, friends ProfileFriendChecker, contacts ProfileContactChecker) (string, bool, error) {
	if friends != nil {
		ok, err := friends.AreFriends(ctx, callerProfile, recipientProfile)
		if err != nil {
			return "", false, err
		}
		if ok {
			return inboxMain, true, nil
		}
	}
	if contacts != nil {
		ok, err := contacts.HasContact(ctx, recipientProfile, callerProfile)
		if err != nil {
			return "", false, err
		}
		if ok {
			return inboxMain, false, nil
		}
	}
	return inboxRequests, false, nil
}
