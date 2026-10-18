package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

type stagingMediaPeer struct {
	AccountID   string `json:"account_id"`
	ProfileID   string `json:"profile_id"`
	AccessToken string `json:"access_token"`
}

type stagingMediaFixture struct {
	Schema        string           `json:"schema"`
	APIOrigin     string           `json:"api_origin"`
	LivekitOrigin string           `json:"livekit_origin"`
	ChatID        string           `json:"chat_id"`
	Caller        stagingMediaPeer `json:"caller"`
	Callee        stagingMediaPeer `json:"callee"`
}

func parseStagingMediaFixture(raw []byte, expectedOrigin string) (stagingMediaFixture, error) {
	var fixture stagingMediaFixture
	invalid := errors.New("invalid staging media fixture")
	if len(raw) > 65536 {
		return fixture, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&fixture) != nil || decoder.Decode(new(any)) != io.EOF {
		return stagingMediaFixture{}, invalid
	}
	if fixture.Schema != "voice-staging-media-v1" ||
		!stagingMediaOrigin(fixture.APIOrigin, "https") || fixture.APIOrigin != expectedOrigin ||
		!stagingMediaOrigin(fixture.LivekitOrigin, "wss") ||
		!stagingMediaID(fixture.ChatID) || !stagingMediaID(fixture.Caller.ProfileID) ||
		!stagingMediaID(fixture.Callee.ProfileID) || fixture.Caller.ProfileID == fixture.Callee.ProfileID ||
		!stagingMediaID(fixture.Caller.AccountID) || !stagingMediaID(fixture.Callee.AccountID) ||
		fixture.Caller.AccountID == fixture.Callee.AccountID ||
		!stagingMediaToken(fixture.Caller.AccessToken) || !stagingMediaToken(fixture.Callee.AccessToken) ||
		fixture.Caller.AccessToken == fixture.Callee.AccessToken {
		return stagingMediaFixture{}, invalid
	}
	return fixture, nil
}

func stagingMediaOrigin(raw, scheme string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == scheme && u.Hostname() != "" && u.User == nil &&
		u.Path == "" && u.RawPath == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == ""
}

func stagingMediaID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}

func stagingMediaToken(raw string) bool {
	return raw != "" && len(raw) <= 16384 && !strings.ContainsAny(raw, " \t\r\n")
}

func checkStagingMediaProfile(client *http.Client, origin string, peer stagingMediaPeer) error {
	invalid := errors.New("media participant session does not match its expected profile")
	req, err := http.NewRequest(http.MethodGet, origin+"/api/v1/users/me", nil)
	if err != nil {
		return invalid
	}
	req.Header.Set("Authorization", "Bearer "+peer.AccessToken)
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("media participant profile lookup failed")
	}
	defer resp.Body.Close()
	var body struct {
		Profile struct {
			ID        string `json:"id"`
			AccountID string `json:"account_id"`
		} `json:"profile"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&body) != nil ||
		body.Profile.ID != peer.ProfileID || body.Profile.AccountID != peer.AccountID {
		return invalid
	}
	return nil
}
