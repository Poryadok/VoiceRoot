package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/user/internal/store"
)

func TestRowToProto_ExposesBannerURL(t *testing.T) {
	t.Parallel()
	banner := "https://cdn.example/banners/p1.png"
	avatar := "https://cdn.example/avatars/p1.png"
	now := time.Now().UTC().Truncate(time.Second)
	row := &store.ProfileRow{
		ID:            uuid.New(),
		AccountID:     uuid.New(),
		Username:      "alice",
		Discriminator: "0001",
		DisplayName:   "Alice",
		AvatarURL:     &avatar,
		BannerURL:     &banner,
		Locale:        "en",
		Theme:         "dark",
		IsPrimary:     true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	out := rowToProto(row)
	require.Equal(t, banner, out.GetBannerUrl())
	require.Equal(t, avatar, out.GetAvatarUrl())
}

func TestRowToProto_OmitsNilBannerURL(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	row := &store.ProfileRow{
		ID:            uuid.New(),
		AccountID:     uuid.New(),
		Username:      "bob",
		Discriminator: "0002",
		DisplayName:   "Bob",
		Locale:        "ru",
		Theme:         "light",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	out := rowToProto(row)
	require.Nil(t, out.BannerUrl)
	require.Empty(t, out.GetBannerUrl())
}
