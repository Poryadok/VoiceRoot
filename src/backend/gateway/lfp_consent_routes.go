package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	storyv1 "voice.app/voice/story/v1"
)

// Public selectors contain only profile/offer data. Caller account/type/epoch
// are not accepted; the exact verified Gateway principal supplies those fields.
type publicLfpOffer struct {
	ProtocolVersion uint64 `json:"protocol_version"`
	StoryID         string `json:"story_id"`
	Incarnation     string `json:"story_incarnation"`
	Revision        uint64 `json:"offer_revision"`
	AuthorProfileID string `json:"author_profile_id"`
	CriteriaSHA256  string `json:"criteria_sha256"`
	PayloadSHA256   string `json:"offer_payload_sha256"`
	ExpiresAt       int64  `json:"offer_expires_at_unix_millis"`
}

func (t *transcoder) serveStoryConsent(w http.ResponseWriter, r *http.Request, rest string) bool {
	parts := strings.Split(strings.TrimPrefix(rest, "/"), "/")
	if len(parts) != 2 || (parts[1] != "lfp-consent-offer" && parts[1] != "lfp-consent-response") {
		return false
	}
	if t == nil || t.clients.storyConsent == nil {
		writeGRPCError(w, status.Error(codes.Unavailable, "LFP consent unavailable"))
		return true
	}
	if !canonicalLifecycleUUID(parts[0]) {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid Story selector"))
		return true
	}
	storyID := uuid.MustParse(parts[0])
	invalid := func() { writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid LFP action")) }
	if parts[1] == "lfp-consent-offer" && r.Method == http.MethodGet {
		request := &storyv1.ObserveLfpConsentOfferRequest{ProtocolVersion: 1, StoryId: storyID[:]}
		ctx, e := t.storyConsentContext(r, request, storyv1.LfpConsentActionService_ObserveLfpConsentOffer_FullMethodName)
		if e != nil {
			writeGRPCError(w, e)
			return true
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		got, e := t.clients.storyConsent.ObserveLfpConsentOffer(ctx, request)
		if e != nil {
			writeGRPCError(w, e)
			return true
		}
		if got == nil || len(got.ProtoReflect().GetUnknown()) != 0 {
			writeGRPCError(w, status.Error(codes.Unavailable, "LFP offer unavailable"))
			return true
		}
		t.writePublicLfpOffer(w, storyID, got.Offer)
		return true
	}
	if parts[1] == "lfp-consent-offer" && r.Method == http.MethodPost {
		var body struct {
			OperationID string `json:"operation_id"`
		}
		if !readLfpJSON(w, r, &body) || !canonicalLifecycleUUID(body.OperationID) {
			invalid()
			return true
		}
		op := uuid.MustParse(body.OperationID)
		request := &storyv1.PublishLfpConsentOfferRequest{ProtocolVersion: 1, PublicationOperationId: op[:], StoryId: storyID[:]}
		ctx, e := t.storyConsentContext(r, request, storyv1.LfpConsentActionService_PublishLfpConsentOffer_FullMethodName)
		if e != nil {
			writeGRPCError(w, e)
			return true
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		got, e := t.clients.storyConsent.PublishLfpConsentOffer(ctx, request)
		if e != nil {
			writeGRPCError(w, e)
			return true
		}
		if got == nil || len(got.ProtoReflect().GetUnknown()) != 0 {
			writeGRPCError(w, status.Error(codes.Unavailable, "LFP offer unavailable"))
			return true
		}
		t.writePublicLfpOffer(w, storyID, got.Offer)
		return true
	}
	if parts[1] == "lfp-consent-response" && r.Method == http.MethodPost {
		var body struct {
			OperationID string          `json:"operation_id"`
			Offer       *publicLfpOffer `json:"offer"`
			Type        string          `json:"response_type"`
		}
		if !readLfpJSON(w, r, &body) || !canonicalLifecycleUUID(body.OperationID) || body.Offer == nil || body.Offer.StoryID != storyID.String() {
			invalid()
			return true
		}
		offer, e := body.Offer.wire()
		if e != nil {
			invalid()
			return true
		}
		var responseType storyv1.LfpConsentResponseType
		switch body.Type {
		case "join":
			responseType = storyv1.LfpConsentResponseType_LFP_CONSENT_RESPONSE_TYPE_JOIN
		case "invite":
			responseType = storyv1.LfpConsentResponseType_LFP_CONSENT_RESPONSE_TYPE_INVITE
		default:
			invalid()
			return true
		}
		op := uuid.MustParse(body.OperationID)
		request := &storyv1.RespondWithLfpConsentRequest{ProtocolVersion: 1, ResponseOperationId: op[:], Offer: offer, ResponseType: responseType}
		ctx, e := t.storyConsentContext(r, request, storyv1.LfpConsentActionService_RespondWithLfpConsent_FullMethodName)
		if e != nil {
			writeGRPCError(w, e)
			return true
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		got, e := t.clients.storyConsent.RespondWithLfpConsent(ctx, request)
		if e != nil {
			writeGRPCError(w, e)
			return true
		}
		if got == nil || len(got.ProtoReflect().GetUnknown()) != 0 {
			writeGRPCError(w, status.Error(codes.Unavailable, "LFP response unavailable"))
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
	return true
}

func readLfpJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF
}
func (p publicLfpOffer) wire() (*storyv1.LfpConsentOfferView, error) {
	if p.ProtocolVersion != 1 || !canonicalLifecycleUUID(p.StoryID) || !canonicalLifecycleUUID(p.Incarnation) || !canonicalLifecycleUUID(p.AuthorProfileID) || p.Revision == 0 || p.ExpiresAt <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid offer")
	}
	decode := func(s string) ([]byte, error) {
		b, e := hex.DecodeString(s)
		if e != nil || len(b) != 32 || s != hex.EncodeToString(b) || string(b) == string(make([]byte, 32)) {
			return nil, status.Error(codes.InvalidArgument, "invalid offer hash")
		}
		return b, nil
	}
	criteria, e := decode(p.CriteriaSHA256)
	if e != nil {
		return nil, e
	}
	payload, e := decode(p.PayloadSHA256)
	if e != nil {
		return nil, e
	}
	id, inc, author := uuid.MustParse(p.StoryID), uuid.MustParse(p.Incarnation), uuid.MustParse(p.AuthorProfileID)
	return &storyv1.LfpConsentOfferView{ProtocolVersion: 1, StoryId: id[:], StoryIncarnation: inc[:], AuthorProfileId: author[:], OfferRevision: p.Revision, CriteriaSha256: criteria, OfferPayloadSha256: payload, OfferExpiresAtUnixMillis: p.ExpiresAt}, nil
}
func (t *transcoder) writePublicLfpOffer(w http.ResponseWriter, id uuid.UUID, p *storyv1.LfpConsentOfferView) {
	if p == nil || len(p.ProtoReflect().GetUnknown()) != 0 || len(p.StoryId) != 16 || len(p.StoryIncarnation) != 16 || len(p.AuthorProfileId) != 16 {
		writeGRPCError(w, status.Error(codes.Unavailable, "LFP offer unavailable"))
		return
	}
	out := publicLfpOffer{ProtocolVersion: p.ProtocolVersion, StoryID: uuid.UUID(p.StoryId).String(), Incarnation: uuid.UUID(p.StoryIncarnation).String(), AuthorProfileID: uuid.UUID(p.AuthorProfileId).String(), Revision: p.OfferRevision, CriteriaSHA256: hex.EncodeToString(p.CriteriaSha256), PayloadSHA256: hex.EncodeToString(p.OfferPayloadSha256), ExpiresAt: p.OfferExpiresAtUnixMillis}
	if _, e := out.wire(); e != nil || out.StoryID != id.String() {
		writeGRPCError(w, status.Error(codes.Unavailable, "LFP offer unavailable"))
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Offer publicLfpOffer `json:"offer"`
	}{out})
}
