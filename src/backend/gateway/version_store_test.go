package main

import (
	"context"
	"testing"
)

func TestMemoryVersionStoreGetSetRoundTrip(t *testing.T) {
	t.Parallel()

	store := newMemoryVersionStore(nil)
	ctx := context.Background()

	record := clientVersionRecord{
		Platform:            "windows",
		MinSupportedVersion: "1.4.0",
		LatestVersion:       "1.7.2",
		UpdateURL:           "https://updates.voice.example/windows/appcast.xml",
		ReleaseNotes:        "round trip",
	}
	if err := store.Set(ctx, record); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := store.Get(ctx, "windows")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != record {
		t.Fatalf("got = %+v, want %+v", got, record)
	}
}

func TestMemoryVersionStoreGetUnknownPlatform(t *testing.T) {
	t.Parallel()

	store := newMemoryVersionStore(nil)
	_, err := store.Get(context.Background(), "windows")
	if err == nil {
		t.Fatal("expected error for unknown platform")
	}
}

func TestGatewayVersionEndpointReadsFromVersionStore(t *testing.T) {
	t.Parallel()

	store := newMemoryVersionStore(map[string]clientVersionRecord{
		"windows": {
			Platform:            "windows",
			MinSupportedVersion: "1.4.0",
			LatestVersion:       "1.7.2",
			UpdateURL:           "https://updates.voice.example/windows/appcast.xml",
			ReleaseNotes:        "from store",
		},
	})

	h := newGatewayForContract(t, gatewayTestOptions{
		versionStore: store,
	})

	rec := performRequest(h, "GET", "/api/v1/version?platform=windows&version=1.7.2", "", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}

	var got struct {
		LatestVersion string `json:"latest_version"`
		ReleaseNotes  string `json:"release_notes"`
		UpdateURL     string `json:"update_url"`
	}
	decodeJSON(t, rec.Body, &got)
	if got.LatestVersion != "1.7.2" || got.ReleaseNotes != "from store" {
		t.Fatalf("response = %+v, want policy from versionStore", got)
	}
	if store.getCount() != 1 {
		t.Fatalf("version store gets = %d, want 1", store.getCount())
	}
}
