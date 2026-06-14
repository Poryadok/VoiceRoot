package main

import (
	"encoding/json"
	"net/http"
	"strings"

	authv1 "voice.app/voice/auth/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
	userv1 "voice.app/voice/user/v1"
)

func (t *transcoder) serveAuthREST(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodGet && rest == "linked-accounts":
		writeJSON(w, http.StatusOK, map[string]any{"linked_accounts": []any{}})
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/link"):
		writeJSON(w, http.StatusOK, map[string]any{"authorization_url": "https://id.twitch.tv/oauth2/authorize"})
		return true

	case r.Method == http.MethodPut && rest == "e2e-key-backup":
		if t.clients.auth == nil {
			http.NotFound(w, r)
			return true
		}
		req := &authv1.PutE2EKeyBackupRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		_, err := t.clients.auth.PutE2EKeyBackup(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodGet && rest == "e2e-key-backup":
		if t.clients.auth == nil {
			http.NotFound(w, r)
			return true
		}
		resp, err := t.clients.auth.GetE2EKeyBackup(ctx, &authv1.GetE2EKeyBackupRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "switch-profile":
		if t.clients.auth == nil {
			http.NotFound(w, r)
			return true
		}
		req := &authv1.SwitchActiveProfileRequest{
			AccessToken:    strings.TrimPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer "),
			DeviceInfoJson: "{}",
		}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.AccessToken == "" {
			req.AccessToken = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer "))
		}
		resp, err := t.clients.auth.SwitchActiveProfile(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		sess := resp.GetSession()
		writeProtoJSON(w, http.StatusOK, sess)
		return true

	default:
		return false
	}
}

func (t *transcoder) serveUsersPhase13(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	profileID := strings.TrimSpace(r.Header.Get("X-Voice-Profile-Id"))

	switch {
	case r.Method == http.MethodPost && rest == "profiles":
		req := &userv1.CreateProfileRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.user.CreateProfile(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "me/verification":
		resp, err := t.clients.user.GetVerificationStatus(ctx, &userv1.GetVerificationStatusRequest{
			ProfileId: profileID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "me/verification/organization":
		req := &userv1.StartOrganizationVerificationRequest{ProfileId: profileID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.ProfileId == "" {
			req.ProfileId = profileID
		}
		resp, err := t.clients.user.StartOrganizationVerification(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		return false
	}
}

func (t *transcoder) serveSubscriptionPhase13(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t.clients.subscription == nil {
		return false
	}
	ctx := withGRPCMetadata(r.Context(), r)
	accountID := strings.TrimSpace(r.Header.Get("X-Voice-User-Id"))

	switch {
	case r.Method == http.MethodGet && rest == "limits":
		resp, err := t.clients.subscription.GetLimits(ctx, &subscriptionv1.GetLimitsRequest{
			AccountId: accountID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "downgrade/profiles":
		var body struct {
			KeptProfileIds []string `json:"kept_profile_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.subscription.ApplyDowngradeProfiles(ctx, &subscriptionv1.ApplyDowngradeProfilesRequest{
			AccountId:       accountID,
			KeptProfileIds: body.KeptProfileIds,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		return false
	}
}
