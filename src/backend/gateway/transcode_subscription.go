package main

import (
	"io"
	"net/http"
	"strings"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

func (t *transcoder) serveSubscription(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t.clients.subscription == nil {
		return false
	}
	ctx := withGRPCMetadata(r.Context(), r)
	accountID := strings.TrimSpace(r.Header.Get("X-Voice-User-Id"))

	switch {
	case r.Method == http.MethodGet && rest == "me":
		resp, err := t.clients.subscription.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{
			AccountId: accountID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "checkout":
		req := &subscriptionv1.CreateCheckoutSessionRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.subscription.CreateCheckoutSession(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && strings.HasPrefix(rest, "webhooks/"):
		body, err := readRawBody(r)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		sig := strings.TrimSpace(r.Header.Get("Paddle-Signature"))
		if strings.TrimPrefix(rest, "webhooks/") == "paddle" {
			_, err := t.clients.subscription.HandlePaddleWebhook(ctx, &subscriptionv1.HandlePaddleWebhookRequest{
				RawBody:   string(body),
				Signature: sig,
			})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusOK)
			return true
		}
		http.NotFound(w, r)
		return true

	default:
		return false
	}
}

func readRawBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r.Body, 4<<20))
}
