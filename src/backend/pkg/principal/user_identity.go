package principal

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// ErrInvalidDelegatedUser discloses no identity or request details.
var ErrInvalidDelegatedUser = errors.New("invalid delegated user")

// UserIdentity is request-bound data read only after the network verifier has
// authenticated a Gateway delegated principal. Empty AccountType stays empty.
type UserIdentity struct {
	AccountID    uuid.UUID
	ProfileID    uuid.UUID
	SessionEpoch int64
	AccountType  string
}

// RequireDelegatedUser reads an already verified principal; it does not verify
// tokens or infer authority from incoming/outgoing metadata or request fields.
func RequireDelegatedUser(ctx context.Context, audience, fullMethod string, request proto.Message) (UserIdentity, error) {
	p, ok := FromContext(ctx)
	if !ok || audience == "" || fullMethod == "" || p.Kind != "delegated_user" || p.Issuer != "gateway" || p.Subject != p.AccountID || p.Audience != audience || p.RPC != fullMethod || p.RequestID == "" || p.SessionEpoch <= 0 || !validAccountType(p.AccountType) || p.ActorProfileID != "" || p.BotID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return UserIdentity{}, ErrInvalidDelegatedUser
	}
	account, accountErr := uuid.Parse(p.AccountID)
	profile, profileErr := uuid.Parse(p.ProfileID)
	if accountErr != nil || profileErr != nil || account == uuid.Nil || profile == uuid.Nil || account.String() != p.AccountID || profile.String() != p.ProfileID || request == nil || !request.ProtoReflect().IsValid() {
		return UserIdentity{}, ErrInvalidDelegatedUser
	}
	hash, err := RequestHash(request)
	if err != nil || hash != p.RequestHash {
		return UserIdentity{}, ErrInvalidDelegatedUser
	}
	return UserIdentity{AccountID: account, ProfileID: profile, SessionEpoch: p.SessionEpoch, AccountType: p.AccountType}, nil
}
