# A1 staging acceptance evidence — 2026-09-27

## Deployment

- Full, data-preserving staging deploy: [run 36336866095](https://github.com/Poryadok/VoiceRoot/actions/runs/36336866095), source SHA `0dd31ed04a832d26dadb3162256664ce6d898905`; conclusion `success`.
- All 30 staging Deployments were observed ready `1/1`. Public `/health` returned HTTP 200. `/api/v1/version?platform=windows&version=1.0.0` returned HTTP 200 with `force_update=false` and `update_available=false`.
- The deployment did not reset namespaces or wipe staging data. All eight namespaces remained `Active` afterward.
- Follow-up direct requests still return `200` from `/health` and the Windows version endpoint. The browser automation attempt to open `https://voice.comrade.click/` was blocked by the browser with `ERR_BLOCKED_BY_CLIENT`, so this pass did not claim UI acceptance.
- The public frontend root `https://app.comrade.click/` responds `200` to `HEAD`; this confirms static site availability only, not an interactive UI flow.

## Automated acceptance at the deployed SHA

Full CI was dispatched at the exact deployed SHA: [run 36338102698](https://github.com/Poryadok/VoiceRoot/actions/runs/36338102698); the run completed successfully at 2026-09-27 18:54:15 UTC. The A1 multi-account Compose proof, attachment restart proof, Flutter profile handoff, and Compose E2E all completed successfully. `make build-all` passed at 18:48:39 UTC and `make flutter-ci` passed at 18:54:10 UTC.

## Email delivery

- The owner reports that a confirmation email arrived at their Zoho address from the staging app.
- Subsequent registration attempts to Yandex and Google addresses returned HTTP 503 in the browser. Auth maps an outbound mail-send failure to `auth_unavailable`/503; therefore these errors are consistent with Resend rejecting the recipient, though the exact Resend response for those requests has not yet been inspected.
- A later repeat `POST /api/v1/auth/register` to Zoho returned HTTP 429. Auth's `otp_rate_limited` path returns 429 with a conservative `Retry-After: 600`; the default configured send cooldown is `PT1M`, and the staging override and response header for this exact request were not captured. The repository's Auth recovery plan explicitly treats 600 seconds as a conservative retry window, so this is consistent with OTP throttling but does not establish the precise cooldown or prove the source of this response.
- The provided staging template sets `AUTH_RESEND_FROM` to `Voice <onboarding@resend.dev>`. When asked for the verified domain, the owner identified `onboarding@resend.dev`; this is the shared test sender, not an owned sending domain. Resend documents shared test senders as testing-only and requires a domain the sender controls and has verified for sending to users ([test sender guidance](https://resend.com/changelog/v0-integration), [verified domains](https://resend.com/docs/dashboard/domains/introduction)). A send-only subdomain such as `mail.comrade.click` can use Resend DNS records without a separate mailbox or mail-hosting service.
- A public DNS lookup via `1.1.1.1` found no MX, TXT, or CNAME records for `mail.comrade.click` at lookup time; the records may be unadded or still propagating.
- Registration results for the rejected recipients may have persisted accounts before the mail send failed. This remains unverified; avoid retrying those same addresses until the sender is fixed, then verify whether those identities need a safe staging-only recovery.

## Remaining A1 acceptance

- Confirm delivery to a non-owner mailbox after configuring an owned, verified Resend sending domain; then verify registration, login, email verification, and reset.
- Run the two-user Web acceptance against staging for friends/requests/block, DM/group/channel/thread, reconnect and inbox/history catch-up, unread/read, archive/folders/Quick Access, attachments, and account soft-delete behavior.
- Guest sessions cannot substitute for this: the product rules prohibit guests from initiating friend invites and DMs, so this requires regular accounts with verified email.
- Verify the Web UI empty/error/offline states and profile/chat switching. A1 remains open; A2 must not enter WIP before these checks and the complete A1 DoD pass.

## 2026-09-28 follow-up

- PR [#526](https://github.com/Poryadok/VoiceRoot/pull/526) merged as `a813d82c6ea6b803bf2f83b906db75d58971bb97`. It rejects malformed, empty, and whitespace-only supplied email values before registration or OTP work while keeping omitted email valid for guest registration. Auth tests passed 554/554 and master CI run [36378284780](https://github.com/Poryadok/VoiceRoot/actions/runs/36378284780) succeeded.
- Full data-preserving staging deploy and smoke run [36379144973](https://github.com/Poryadok/VoiceRoot/actions/runs/36379144973) succeeded at the merge SHA. The Auth deployment rolled out successfully; the smoke checked Gateway, version, registration validation (malformed email returns HTTP 400), staff routes, web endpoints, and LiveKit signaling. Public Gateway and Flutter web `/health` returned `ok` afterward.
- The workflow set `VOICE_NATS_FRESH_INSTALL=false`; `voice-staging` was unchanged and no wipe or namespace reset ran. Auth email-secret preflight passed. The prior browser registration reached verification-code entry, but the user has not confirmed whether the most recent Zoho email arrived. The current app tab shows “Enable accessibility.” A1 remains open; do not start A2 until live acceptance is complete.
