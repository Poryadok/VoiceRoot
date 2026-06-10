package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/space/internal/store"
)

func TestSpaceRowToProto_Nil(t *testing.T) {
	t.Parallel()
	require.Nil(t, spaceRowToProto(nil))
}

func TestSpaceRowToProto_OptionalURLs(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	owner := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	icon := "https://cdn.voice.gg/spaces/icon.webp"
	banner := "https://cdn.voice.gg/spaces/banner.webp"

	row := &store.SpaceRow{
		ID:               id,
		Name:             "Proto space",
		Description:      "About",
		IconURL:          &icon,
		BannerURL:        &banner,
		Visibility:       "private",
		OwnerProfileID:   owner,
		MemberCount:      1,
		IsVerified:       false,
		VerificationType: "none",
		EntryRequirement: "none",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	out := spaceRowToProto(row)
	require.NotNil(t, out)
	require.Equal(t, id.String(), out.GetId())
	require.Equal(t, icon, out.GetIconUrl())
	require.Equal(t, banner, out.GetBannerUrl())
}
