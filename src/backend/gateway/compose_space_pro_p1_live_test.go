package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestComposeSpaceProMemberCap_live documents SP-11 / SUB-03:
// free tier blocks 51st join; after Space Pro webhook, join succeeds (no SeedSpaceProActive).
func TestComposeSpaceProMemberCap_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 120 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()

	owner := registerComposeUser(t, client, base, formatComposeEmail("sp11-owner", n), "VoiceQaTest1!")
	spaceID := createComposeSpace(t, client, base, owner.AccessToken, "Member Cap QA", "sp-11")

	seedComposeSpaceMembersNearCap(t, spaceID, 50)

	blocked := registerComposeUser(t, client, base, formatComposeEmail("sp11-blocked", n), "VoiceQaTest1!")
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	require.Equal(t, http.StatusTooManyRequests,
		joinComposeSpaceByInviteStatus(t, client, base, blocked.AccessToken, invite.Code),
		"51st member must be ResourceExhausted on free tier")

	composeActivateSpaceProWebhook(t, client, base, spaceID, owner.AccountID)

	// Wait briefly for Subscription→Space entitlement sync (NATS/S2S).
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		row := composeQueryPostgres(t, "space_db",
			fmt.Sprintf("SELECT status FROM space_subscriptions WHERE space_id = '%s' LIMIT 1", spaceID))
		if strings.Contains(row, "active") || strings.Contains(row, "grace_period") {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	allowed := registerComposeUser(t, client, base, formatComposeEmail("sp11-allowed", n), "VoiceQaTest1!")
	invite2 := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	require.Equal(t, http.StatusOK,
		joinComposeSpaceByInviteStatus(t, client, base, allowed.AccessToken, invite2.Code),
		"join after Space Pro must succeed past free member cap")
}

// TestComposeSubscriptionGraceReminder_live documents SUB-04:
// seed grace_period → sweeper emits D1 → grace_reminders_sent contains 1.
func TestComposeSubscriptionGraceReminder_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	sess := registerComposeUser(t, client, base, formatComposeEmail("grace-sub", n), "VoiceQaTest1!")

	subID := uuid.NewString()
	providerSubID := "grace-live-" + subID
	// Day 1 of a 7-day grace window: grace_period_end ≈ now+6d.
	composeExecPostgres(t, "subscription_db", fmt.Sprintf(`
INSERT INTO subscriptions (
  id, account_id, plan, billing_period, status, provider, provider_subscription_id,
  current_period_start, current_period_end, grace_period_end, grace_reminders_sent
) VALUES (
  '%s', '%s', 'premium', 'monthly', 'grace_period', 'paddle', '%s',
  now() - interval '30 days', now() - interval '1 day', now() + interval '6 days', '{}'
);`, subID, sess.AccountID, providerSubID))

	deadline := time.Now().Add(90 * time.Second)
	var sent string
	for time.Now().Before(deadline) {
		sent = composeQueryPostgres(t, "subscription_db",
			fmt.Sprintf("SELECT grace_reminders_sent::text FROM subscriptions WHERE id = '%s'", subID))
		if strings.Contains(sent, "1") {
			break
		}
		time.Sleep(3 * time.Second)
	}
	require.Contains(t, sent, "1", "sweeper must mark D1 grace reminder (grace_reminders_sent=%s)", sent)
}
