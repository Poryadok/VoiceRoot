package main

import (
	"net/http"
	"strings"

	userv1 "voice.app/voice/user/v1"
	commonv1 "voice.app/voice/common/v1"
)

func (t *transcoder) serveUsers(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t.serveUsersPhase13(w, r, rest) {
		return true
	}
	ctx := withGRPCMetadata(r.Context(), r)
	profileID := strings.TrimSpace(r.Header.Get("X-Voice-Profile-Id"))

	switch {
	case r.Method == http.MethodGet && rest == "me/privacy":
		resp, err := t.clients.user.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
			ProfileId: profileID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPatch && rest == "me/privacy":
		req := &userv1.UpdatePrivacySettingsRequest{ProfileId: profileID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.ProfileId == "" {
			req.ProfileId = profileID
		}
		if req.GetSettings() != nil && req.GetSettings().GetProfileId() == "" {
			req.Settings.ProfileId = profileID
		}
		resp, err := t.clients.user.UpdatePrivacySettings(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "me":
		resp, err := t.clients.user.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID},
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPatch && rest == "me":
		req := &userv1.UpdateProfileRequest{ProfileId: profileID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.ProfileId == "" {
			req.ProfileId = profileID
		}
		resp, err := t.clients.user.UpdateProfile(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "profiles":
		resp, err := t.clients.user.ListMyProfiles(ctx, &userv1.ListMyProfilesRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "search":
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.PageSize == 0 {
			if n := queryFirst(r, "page_size"); n != "" {
				page.PageSize = parseInt32Query(n)
			}
		}
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		resp, err := t.clients.user.SearchProfiles(ctx, &userv1.SearchProfilesRequest{
			Query: queryFirst(r, "q"),
			Page:  page,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && strings.HasPrefix(rest, "profiles/") && strings.HasSuffix(rest, "/presence"):
		target := strings.TrimSuffix(strings.TrimPrefix(rest, "profiles/"), "/presence")
		resp, err := t.clients.user.GetPresence(ctx, &userv1.GetPresenceRequest{ProfileId: target})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && strings.HasPrefix(rest, "profiles/"):
		target := strings.TrimPrefix(rest, "profiles/")
		if target == "" || strings.Contains(target, "/") {
			return false
		}
		resp, err := t.clients.user.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_ProfileId{ProfileId: target},
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPatch && rest == "me/presence":
		req := &userv1.UpdatePresenceRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.user.UpdatePresence(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "presence/bulk":
		req := &userv1.GetBulkPresenceRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.user.GetBulkPresence(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "me/avatar/presigned-upload":
		req := &userv1.CreateAvatarPresignedUploadRequest{ProfileId: profileID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.ProfileId == "" {
			req.ProfileId = profileID
		}
		resp, err := t.clients.user.CreateAvatarPresignedUpload(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "me/onboarding":
		resp, err := t.clients.user.GetOnboardingState(ctx, &userv1.GetOnboardingStateRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "me/onboarding/steps":
		req := &userv1.CompleteOnboardingStepRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.user.CompleteOnboardingStep(ctx, req)
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

func parseInt32Query(raw string) int32 {
	var n int32
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int32(c-'0')
		if n > 1000 {
			return 1000
		}
	}
	return n
}
