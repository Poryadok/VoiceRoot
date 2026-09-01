package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	socialv1 "voice.app/voice/social/v1"
)

// ProfileContactChecker reports whether owner has contact in their contact list.
type ProfileContactChecker interface {
	HasContact(ctx context.Context, ownerProfileID, contactProfileID uuid.UUID) (bool, error)
}

type SocialGRPCContacts struct {
	Client socialv1.SocialServiceClient
}

func NewSocialGRPCContacts(cc grpc.ClientConnInterface) *SocialGRPCContacts {
	return &SocialGRPCContacts{Client: socialv1.NewSocialServiceClient(cc)}
}

func (s *SocialGRPCContacts) HasContact(ctx context.Context, ownerProfileID, contactProfileID uuid.UUID) (bool, error) {
	if s == nil || s.Client == nil {
		return false, nil
	}
	resp, err := s.Client.HasContact(ctx, &socialv1.HasContactRequest{
		OwnerProfileId:   ownerProfileID.String(),
		ContactProfileId: contactProfileID.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetContact(), nil
}
