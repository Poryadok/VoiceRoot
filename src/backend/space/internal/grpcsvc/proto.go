package grpcsvc

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

func spaceRowToProto(r *store.SpaceRow) *spacev1.Space {
	if r == nil {
		return nil
	}
	out := &spacev1.Space{
		Id:               r.ID.String(),
		Name:             r.Name,
		Description:      r.Description,
		Visibility:       r.Visibility,
		OwnerProfileId:   r.OwnerProfileID.String(),
		MemberCount:      r.MemberCount,
		IsVerified:       r.IsVerified,
		VerificationType: r.VerificationType,
		EntryRequirement: r.EntryRequirement,
		CreatedAt:        timestamppb.New(r.CreatedAt),
		UpdatedAt:        timestamppb.New(r.UpdatedAt),
	}
	if r.IconURL != nil {
		out.IconUrl = r.IconURL
	}
	if r.BannerURL != nil {
		out.BannerUrl = r.BannerURL
	}
	return out
}
