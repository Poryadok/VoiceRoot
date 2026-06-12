package main

import (
	"context"
	"testing"
)

func TestEnvVersionStoreSetUpdatesConfig(t *testing.T) {
	t.Parallel()

	store := newEnvVersionStore(map[string]versionConfig{
		"windows": {
			MinSupportedVersion: "1.0.0",
			LatestVersion:       "1.0.0",
			UpdateURL:           "https://example.com/a.xml",
		},
	})

	err := store.Set(context.Background(), clientVersionRecord{
		Platform:            "windows",
		MinSupportedVersion: "1.2.0",
		LatestVersion:       "1.3.0",
		UpdateURL:           "https://example.com/b.xml",
		ReleaseNotes:        "updated",
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := store.Get(context.Background(), "windows")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.LatestVersion != "1.3.0" || got.UpdateURL != "https://example.com/b.xml" {
		t.Fatalf("got = %+v", got)
	}
}
