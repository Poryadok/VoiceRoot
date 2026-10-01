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

## 2026-09-30 follow-up

- PR [#578](https://github.com/Poryadok/VoiceRoot/pull/578) merged as `15a389237502ebbd89bee28cfcba135d39dc1e55`. Full staging deploy [36709748475](https://github.com/Poryadok/VoiceRoot/actions/runs/36709748475) completed successfully with smoke enabled. The workflow did not wipe staging data or reset namespaces. Read-only NATS diagnosis [36710617712](https://github.com/Poryadok/VoiceRoot/actions/runs/36710617712) completed successfully; generation `r20260930a3` was active, the hub was ready `1/1`, and the service deployments reported ready replicas.
- Preliminary two-account Web check: a DM sent from the owner’s `+2` test account appeared in the `+4` account’s conversation after signing in. A pending friend request appeared in the recipient profile dialog; accepting it updated the recipient Friends list immediately, and the sender Friends list showed the accepted account after the sender signed in again. This does not yet prove live notification/inbox delivery without a new login or refresh.
- An attachment card appeared in the conversation and remained in history after signing in again as the recipient. The contents were not opened or downloaded, so storage download/hash verification remains open.
- Additional owner-side hard-refresh check: after reloading the `+2` account, the DM history and the previously sent attachment card reappeared after opening the chat. The UI's Realtime badge continued to show `Offline`, but staging logs show the Gateway issued a WebSocket ticket successfully and Realtime accepted a socket for the active profile shortly afterward, with no disconnect in the captured window. The mismatch between the client badge and server connection is unresolved; live event delivery after this refresh is unverified. No attachment content was accessed.
- Local follow-up fix initializes each chat controller from the current Realtime status instead of waiting for a future status change. A regression test covers opening a chat after the app-level connection is already established. The entire `chat_state_test.dart` file passes (12 tests), and Flutter analysis passes with 8 existing `info` notices. This change is not deployed yet; recheck the badge and live event delivery on staging after the batch PR.
- Profile accent check: changing the primary profile accent, reopening Settings, and hard-reloading preserved the selection; the original accent was restored after the check. The reported cross-profile reset is not reproduced yet. Selecting the secondary profile from the avatar menu closed the menu but the primary profile remained selected; investigate profile switching and its failure feedback.
- Owner reported sending another file. The outgoing attachment card and the recipient's earlier incoming attachment card are both visible in the conversation after reconnect; the new card also remained after reopening the chat. No file contents were opened or downloaded. During this check the UI stayed on `Reconnecting...` with a loading history for several minutes; sanitized Gateway access logs show repeated `401` responses for realtime WebSocket tickets after profile switching, followed by a `200`, after which the UI showed `Live` and history loaded. Treat profile-switch/realtime recovery as a reproducible acceptance concern; identify why the first ticket attempts are rejected and verify prompt recovery plus live delivery after the batch fix.
- The refresh/profile race is now reproduced in regression tests: a refresh following a profile switch issued the primary profile, and an overlapping client refresh could replace the switched session. Auth now binds newly issued refresh-token rows to the active profile, while Flutter discards a refresh response if its source token is no longer current. The Auth suite passes locally (557/557); `auth_controller_test.dart` passes 38/38 and `chat_state_test.dart` 12/12. PR [#579](https://github.com/Poryadok/VoiceRoot/pull/579) is open at `d6be991b97a067ab4170db4512cbd4811198e3b6`; required GitHub checks and review are not complete, so this code is not deployed. A fresh owner-uploaded attachment card is visible in the recipient conversation while Realtime shows `Live`; no file was opened or downloaded.
- Quick Access inspection confirmed the add action was hidden behind a long-press-only row menu; there was no visible desktop button and right-click did nothing. A visible star button is now on each eligible chat row and calls the existing 15-slot replace flow; a widget regression assertion covers discoverability. This is only local on PR #579 follow-up and has not been deployed; add/remove/reorder and 15-slot replacement still need live acceptance.
- Other owner-reported gaps still need verification/fixes: the recipient needs a refresh/login to see friend and DM requests; search can return the signed-in profile; blocked profiles are shown by GUID and a blocked DM has an unclear send error; profile palette persistence across profiles is unresolved. The primary accent survived a hard reload in the earlier check. Group/channel creation, participant-specific unread/read state, avatar upload, attachment download/hash, and selected-chat history catch-up have not been verified.
- `docs/PLAN.md` includes minimum account soft-delete in A1 scope, while this note previously deferred it. Treat that as an acceptance-scope conflict to resolve; the prior `+3` deletion/recovery attempt did not establish a working restore path. Do not perform a further deletion until the restore/relogin path can be checked safely.
- A1 remains open: complete live two-client delivery, attachments/avatar, Quick Access, group/channel/thread and unread/read states, reconnect/history catch-up, block/privacy, offline/error/profile switching, email reset, and the A1 soft-delete scope above. Do not start A2 before the complete A1 DoD passes.

## 2026-09-30 follow-up — deployed acceptance fixes

- PR [#584](https://github.com/Poryadok/VoiceRoot/pull/584) merged as `a85f4c80322e13b121fc8abd8a9f76e38dd58890`. Master CI [36760440022](https://github.com/Poryadok/VoiceRoot/actions/runs/36760440022), read-only NATS diagnostic [36762095992](https://github.com/Poryadok/VoiceRoot/actions/runs/36762095992), and full data-preserving staging deploy with smoke [36762240257](https://github.com/Poryadok/VoiceRoot/actions/runs/36762240257) succeeded on that exact SHA. The diagnostic reported `NATS_SEARCH_JS_INFO=PASS`.
- The deployed change clears a conversation's local unread badge after `MarkRead` succeeds, unless a newer message arrived during the request; otherwise it refreshes the inbox. It also exposes group/space creation entry points in expanded navigation. Local `make flutter-ci` passed before merge.
- Owner live-accepted Quick Access capacity and replacement behavior: 15 items fit; attempting to add a 16th asks which item to replace. The replacement selector shows GUIDs; owner accepts this as a UI-only issue to address during the planned UI redesign.
- Post-deploy live verification remains blocked by the browser session. Reloading the existing tab showed `@xronos4#1078`, persistent `Reconnecting…`, and an empty conversation list; therefore this pass does not claim that unread-badge clearing or group/space creation works end-to-end on staging. The browser had shown prior chat rows before reload, so the post-reload list/connectivity regression needs diagnosis. No staging data or namespace was deleted or reset.
- A1 remains open. Still required: recover reliable live Realtime/session state, retest immediate badge clearing and group/space creation, then finish no-refresh inbox propagation, group/channel/thread and per-member unread, reconnect/global inbox/history catch-up, attachment download/hash and avatar upload, block/privacy, archive/folders, profile/chat switching, email reset, and the minimum soft-delete/recovery scenario from `docs/PLAN.md`. Keep A2 out of WIP until all A1 DoD items pass.

## 2026-10-01 follow-up — unread fix deployed

- PR [#586](https://github.com/Poryadok/VoiceRoot/pull/586) merged as `f9c4abf9d208bec6af95a02b8545b9fc3a4213a7`. Full CI [run 36767611478](https://github.com/Poryadok/VoiceRoot/actions/runs/36767611478) passed, including Flutter, Windows Flutter, Compose E2E, A1 E2E, attachment restart proof, and Flutter profile handoff.
- That CI run's isolated A1 Flutter profile-handoff job includes T106, which exercises account soft-delete with generated disposable Compose identities. Treat it as automated contract evidence; it does not authorize deleting an existing staging account or replace live staging acceptance.
- The deployed Search service excludes every profile owned by the viewer's account in both global and user search; targeted Go tests cover primary and sibling-profile exclusion. The exact-SHA full CI run passed these tests. Recheck visible results on staging when signed in, since this does not explain the earlier UI report by itself.
- Current source and passing exact-SHA CI also cover the earlier privacy/UI reports: block-list rows suppress UUID-shaped labels and fall back to neutral text, and blocked-DM errors map to localized copy. These reports are not confirmed code gaps at the deployed SHA; verify the live route before considering changes.
- Flutter's in-app notification controller invalidates incoming friend requests on `friend_request`, updates the request inbox on `message_request`/`chat_update`, and refreshes missed friend requests after reconnect. `in_app_notifications_test.dart` covers these no-reload and reconnect paths; exact-SHA Flutter CI passed. The historical staging report still requires a live two-account check because unit fakes do not prove WebSocket delivery.
- The exact merge SHA was deployed with the data-preserving full staging workflow and smoke enabled: [run 36779116303](https://github.com/Poryadok/VoiceRoot/actions/runs/36779116303). The run completed successfully; image verification, NATS storage and ACL preflights, Auth email preflight, secret restoration, manifest apply, and staging smoke passed. No namespace reset or staging data wipe was requested.
- Independent read-only checks returned HTTP 200 from Gateway `/health` and Flutter Web `/health`. The Windows version endpoint returned `force_update=false` and `update_available=false`.
- The current File service signs browser-facing requests against `FILE_R2_SIGNING_ENDPOINT`, rendered to the configured public HTTPS storage origin; the internal `http://voice-minio:9000` endpoint remains for server-side object operations. The successful deploy smoke performs read-only CORS preflights for file and avatar buckets. This indicates the previously reported internal-host URL path has deployment support now, but does not verify an actual browser upload, download, or content hash.
- Independent read-only OPTIONS probes to the public file and avatar bucket paths returned HTTP 204 with `Access-Control-Allow-Origin: https://app.comrade.click`, `Access-Control-Allow-Methods: PUT`, and the requested `content-type` header. No objects were created or modified.
- Before deployment, the browser showed the existing `@xronos4#1078` account and an unread badge on the existing `+5` conversation; that conversation was left unopened. After deployment, a hard reload showed the sign-in screen. This is not enough evidence to attribute the session loss to client or Auth behavior: the browser storage state and sanitized `/api/v1/auth/refresh` result are not available yet. No password was guessed, and no reset, new account, or account deletion was attempted.
- A1 remains open. The already-deployed unread fix still needs live verification, and all remaining two-user acceptance gates listed above remain outstanding. Do not start A2 until the complete A1 DoD passes.

## 2026-10-01 live follow-up — Quick Access, group, space, unread separator

- The owner confirmed that adding a 16th Quick Access item opened the replacement flow and replaced an existing selected item. This completes the previously open live check for 15-item capacity and replacement; the GUID labels remain an accepted UI-only issue for the planned redesign.
- Signed in as the owner's `+2` staging account. The previously created two-member group is visible, its synthetic acceptance message is present, and the UI reports `Live`. The user confirmed that `debug.md` was sent; the visible outgoing attachment card showed `Read`. Its contents were not opened or downloaded, so recipient download/hash and restart verification remain open.
- Created a synthetic staging Space through the Web UI and verified that it appeared under My spaces and opened to its detail view. No channel was created. A read-only source audit found no Flutter create-channel UI, but the A1 plan requires channel messaging/read behavior rather than the Space-tree creation flow; that UI is A2 scope. A1 still needs a usable existing or preseeded channel for its channel-specific messaging/unread check.
- Profile switching to the existing secondary `2q` profile did change the active profile to `@2q#6508`; this does not reproduce the earlier failure to switch. Realtime briefly displayed `Reconnecting…` during the switch. Switching back to `@xronos2#6340` restored the primary profile and its inbox after selecting the All filter. Live delivery/reconnect on the secondary profile remains unverified.
- Read-only source tracing confirms the switch coordinator commits the new auth session, retires the old room subscriptions, and reconnects the realtime hub using the new session. The short `Reconnecting…` state is expected while ticket or WebSocket attempts retry; no code gap is confirmed. A persistent reconnect would still need sanitized ticket/upgrade status and confirmation that `hello` arrives. This environment exposes only the in-app browser, so a separate concurrent `+4` sender session could not be opened for no-refresh delivery verification.
- Live global search for the signed-in `xronos2` profile returned “No matching contacts,” “No matching spaces,” and “No matching messages.” This verifies the visible self-search exclusion on staging for that query.
- The existing `+2` → `+4` DM opens and reports `Live`. A separate `+4` API login succeeded, but the manual chat-list response did not identify the existing DM, so no message was sent and new no-refresh delivery remains unverified. Check the repository’s two-account live harness before attempting another manual API session.
- Reproduced a stale `Unread messages` separator leaking from the previously selected conversation into a newly created empty group. The local Flutter fix resets the unread snapshot when `ChatRoomPanel` changes `chatId`; it does not change the historical separator within one chat visit. Added a regression covering unread chat → zero-unread chat switching. This fix is local and has not been deployed.
- Verification: the focused chat widget batch passes (48 tests), and `make flutter-ci` completed successfully (1,189 passed, 96 skipped). The full target ran against the same behavior before removing formatter-only churn; the focused batch was rerun after narrowing the diff. `graphify update .` completed. No staging reset, account deletion, or cleanup was performed.
- A1 remains open. The local unread-separator fix must be included in the next implementation batch and deployed once the batch is ready. Remaining live gates include two-account no-refresh DM/friend delivery, per-member channel/thread unread/read and reconnect/history catch-up, attachment download/hash and avatar upload, block/privacy, archive/folders, profile switching/offline errors, email reset, and the minimum soft-delete/recovery path. Do not start A2 until the complete A1 DoD passes.

## 2026-10-01 friends-area follow-up

- The owner clarified that the 16th item was added by replacing an existing member of the 15-item Quick Access list; the staging UI currently shows 15 Quick Access entries, consistent with that result.
- Read-only Friends UI inspection in the same `@xronos2#6340` session showed no friend requests and no blocked accounts. The Friends list includes the existing `+4` staging account.
- Friends → Favorites is a separate list from chat Quick Access. It currently shows “No favorite people yet”; the Quick Access replacement result does not verify the friend-favorites behavior.

## 2026-10-01 existing-account DM delivery

- Added `src/frontend/test/existing_accounts_dm_ws_live_test.dart`, an opt-in staging check that logs into two existing accounts, pages both main inboxes, requires one reciprocal mutual DM, keeps the receiver subscribed over WebSocket, sends one uniquely tagged message, and asserts the matching `message_create` frame without fetching history. It creates no accounts and does not alter privacy settings. Usage and required environment variables are documented in [TESTING.md](../TESTING.md).
- Ran it with the existing `+4` account as sender and the existing `+2` account as receiver against `https://voice.comrade.click`. The REST send and receiver WebSocket assertions passed; the message appeared in the already-open `+2` conversation without a page reload. A rerun completed with both temporary sessions logged out successfully.
- The first run delivered its message but failed in teardown when the reused HTTP connection closed before the receiver logout response; it also stopped before attempting sender logout. The harness now uses a fresh HTTP client for each logout and attempts both cleanups independently. The first run's temporary session IDs were not retained, so whether those two sessions remain active is unverified; no existing sessions were revoked by enumeration.
- Local validation: focused test passes by default as an opt-in skip; `flutter analyze test/existing_accounts_dm_ws_live_test.dart` reports no issues; the explicit staging run passed. Full `make flutter-ci` passed with 1,189 tests passed and 97 skipped.
- After the earlier receiver login was rate-limited, a single retry following the 10-minute cooldown passed with the same `+4` sender → `+2` receiver pair. The receiver WebSocket received the matching `message_create`, and the uniquely tagged message appeared in the already-open `+2` → `+4` conversation without a page reload. Current browser state shows the new message; no history fetch or reload was used for this UI observation. The test logs out both temporary sessions. This closes live no-refresh DM delivery for this pair; the test message remains in staging history.

## 2026-10-01 unread and onboarding follow-up

- In the signed-in `@xronos2#6340` staging session, opening the existing `q9` group and `+8` DM cleared each unread badge after the message view loaded. A hard reload preserved both cleared states. This verifies persisted read state for one existing group and one existing DM; it does not close the broader per-member group/channel/thread matrix.
- PR [#587](https://github.com/Poryadok/VoiceRoot/pull/587) merged as `ed429261b56237c51b31c262cdb15eaa09ae2fb0`; master CI run [36825595424](https://github.com/Poryadok/VoiceRoot/actions/runs/36825595424) and full, data-preserving staging deploy [36827535118](https://github.com/Poryadok/VoiceRoot/actions/runs/36827535118) passed. The deploy verified images, applied manifests, and passed staging smoke without resetting namespaces or wiping data.
- After that deploy, the “Set up your profile” onboarding prompt still returned after Skip and a hard reload in the signed-in `@xronos2#6340` session. The app also showed `Reconnecting…` during this observation. Source audit confirms that a failed dismissal POST leaves controller state incomplete while the UI can retry the same step; the staging POST/GET response was not captured, so this remains a confirmed UI recurrence with an unconfirmed cause. Keep this acceptance gate open; do not claim PR #587 resolved it.
- The 15-item Quick Access replacement result remains confirmed separately; the 16th entry replaced a selected existing item, leaving 15 visible entries.
- Local verification before merge after the onboarding and unread-separator changes: the focused onboarding/chat widget batch passed (19 tests), and `make flutter-ci` passed with 1,190 tests passed and 97 skipped. PR #587 exact-SHA CI and the post-merge master CI passed.
- The existing synthetic attachment restart/hash proof is unsafe for the staging file because it creates and deletes its own fixture. Keep the existing `debug.md` untouched; its staging IDs, authorized recipient, and a pre-restart metadata hash still need to be identified before planning a file-service-only restart and streaming hash check.

## 2026-10-01 post-auth deployment follow-up

- PR [#589](https://github.com/Poryadok/VoiceRoot/pull/589) merged as `b126a62b502da1f7d8687116bdb74b9cdb43e84f`. Exact-merge master CI [run 36834748802](https://github.com/Poryadok/VoiceRoot/actions/runs/36834748802) passed all 38 jobs.
- A full staging deployment attempt, [run 36836837716](https://github.com/Poryadok/VoiceRoot/actions/runs/36836837716), was canceled before any cluster mutation because the state-preserving NATS migration still lacks its required approval and evidence. The web/Auth patch was then deployed through the data-preserving app-only workflow [run 36837174161](https://github.com/Poryadok/VoiceRoot/actions/runs/36837174161) at the exact merge SHA. Source, NATS rotation/ACL guards, image verification, secret preflights, manifest apply, and staging smoke passed; the NATS storage preflight was skipped by app-only mode. No staging namespace reset or data wipe ran.
- The first hard reload after that rollout landed on the sign-in page. Signing into the already-authorized `+2` staging account reached `@xronos2#6340`; a subsequent hard reload retained that signed-in profile. The cause of the first sign-in page is unknown, so this does not establish whether the prior browser session had expired or failed restoration.
- The profile setup dialog appeared after sign-in. Selecting Skip closed it, and another hard reload kept the account signed in without showing the dialog again. This verifies onboarding dismissal persistence for this staging account on the deployed build; Realtime recovery was not evaluated in this pass.
- The owner confirmed that adding a 16th Quick Access item replaced one of the selected existing items. The visible Quick Access rail still contains 15 entries, completing the live capacity/replacement check. Friend Favorites are a separate feature and were not changed by this action.
- A1 remains open: the NATS migration gate and the two-account, attachment, avatar, unread/read matrix, history catch-up, privacy/block, archive/folders, offline/profile-switch, email-reset, and soft-delete/recovery checks documented above still need completion. Do not start A2 before the complete A1 DoD passes.

## 2026-10-01 friend-request live delivery

- PR [#591](https://github.com/Poryadok/VoiceRoot/pull/591) merged as `67adf35fd486b94e1d9a7358c10c93631c037850`. It adds the opt-in existing-account friend-request WebSocket check and documents its one-shot declined-record behavior. Focused Flutter analysis, default opt-out test, exact-SHA Flutter CI, and Docs link check passed.
- One live attempt with a fresh existing-account pair passed the pair preflight, the REST send returned success, and the already-connected recipient WebSocket received the matching `friend_request` notification with the request ID. This confirms transport delivery while the recipient connection remained open; it does not establish that the Flutter Friends → Requests screen updated without a reload.
- The recipient's subsequent `ListFriendRequests` REST snapshot timed out after 15 seconds, so the full harness failed. Teardown reported no cleanup failure; the pair should be treated as consumed because decline leaves a durable outgoing `declined` row. A separate recipient login attempt stopped on `invalid_credentials` before any request was sent.
- A later preflight-clean `+10` requester → `+2` recipient attempt timed out in `SendFriendInvitation` after 15 seconds. The test teardown reported no cleanup failure and the still-open `+2` Requests tab remained empty. Since the server may commit before a lost response, treat this pair as consumed and do not retry it. This is a second timeout in the same live path, but it does not establish whether the cause is Social service, Gateway, or staging latency; a read-only code-path audit is in progress.
- A1 remains open: verify the recipient UI state and REST request snapshot on a fresh eligible pair, then continue the remaining two-account checks. Do not start A2 before the complete A1 DoD passes.

## 2026-10-01 friend-request timeout remediation

- The source audit found that `SendFriendInvitation` committed the friendship row, then synchronously attempted JetStream publication and discarded publication errors. That could leave a committed request with a timed-out client and no recipient notification.
- A local remediation now stores a friend-request event in a Social DB outbox in the same transaction as the friendship change, publishes it from a retry worker, uses stable event IDs for JetStream deduplication, and cancels undispatched requests on accept/decline. Migration `000006_friend_request_outbox` is forward-only for deployment purposes. The Realtime bootstrap now updates only the existing stream's duplicate-window field in place after validating its subjects, storage, retention, and max age; it does not delete or recreate the stream. This change is not deployed, so it does not resolve staging acceptance yet.
- Delivery remains at-least-once. A JetStream-acknowledged event cannot be retracted if Social loses the DB commit before marking the outbox row delivered; in that narrow crash window, clients must reconcile against the authoritative request list. The 24-hour dedupe window suppresses stored retries only within that window.
- Verification: focused outbox/JetStream tests passed (7 tests), race-enabled focused tests passed (7), `go test -short ./...` passed (59), `go build ./...` passed, `golangci-lint run ./...` passed, NATS bootstrap contract passed, and both NATS YAML documents parsed with exact embedded-script parity.
- Full `go test ./...` reached the Social integration suite but one PostgreSQL integration test panicked because rootless Docker is unsupported on this Windows host; the short suite and targeted outbox tests passed. No local Compose/Realtime stack was started.
- The app-only staging workflow runs Social DB migrations before applying app manifests. It does not update NATS streams; activating the duplicate-window change requires the separate state-preserving NATS bootstrap path. The existing approval/evidence gate for that NATS migration remains in force, so no staging deployment was started.
- The previously timed-out test pairs remain consumed/ambiguous and were not retried. A1 remains open until the NATS gate is cleared, this batch is deployed safely, and a fresh eligible account pair confirms both the REST request snapshot and the live Friends → Requests UI update.

## 2026-10-01 post-merge NATS gate recheck

- PR [#593](https://github.com/Poryadok/VoiceRoot/pull/593) merged at `19f322c1fb05f6dee93439ec37f5b46455f278f1`; exact-master CI [36853518821](https://github.com/Poryadok/VoiceRoot/actions/runs/36853518821) passed. The latest successful staging app-only deploy remains `b126a62b502da1f7d8687116bdb74b9cdb43e84f` ([36837174161](https://github.com/Poryadok/VoiceRoot/actions/runs/36837174161)).
- Ran the built-in read-only Search JetStream permission probe from exact master SHA `19f322c1`; staging run [36855499387](https://github.com/Poryadok/VoiceRoot/actions/runs/36855499387) reported `NATS_SEARCH_JS_INFO=PASS` and `NATS_SEARCH_JS_CLEANUP=PASS`. It created and removed only its diagnostic Job; it did not change NATS state.
- Ran NATS root-rotation `diagnose` from exact master SHA `19f322c1`; staging run [36855714456](https://github.com/Poryadok/VoiceRoot/actions/runs/36855714456) confirmed marker `active`, generation `r20260930a4`, previous generation `r20260930a3`, one ready NATS hub replica, and the `voice-nats-jsdata-r20260930a4` PVC. The four bootstrap Jobs report completed for generation `r20260930a4`; this diagnostic did not rotate roots or mutate NATS state.
- The staging Environment proof generation still matches `r20260930a4`, but `VOICE_NATS_ACL_PROOF_SHA` remains `cfbc383b48e18075686d20267edb76bc5b2c1a36372909535217ac3170bba7f1`, while the merged ACL intent hashes to `58ff1d13fe2f5ce32c6ae5ba0d119296ee30d1dad757ace72d83272c60d04fb5`. The Environment has no `STAGING_NATS_PROOF_CREDS_B64` secret. Do not change proof variables without a live proof from the active generation.
- The master CI and read-only probes are green, but they do not prove the new bootstrap `$JS.API.STREAM.UPDATE.social_events` permission. No supported bootstrap-only issuance/install workflow is documented, and the account migration still requires inventory of the 566 legacy consumers and proof that eight existing stream messages can be preserved. No full/app-only deployment or root rotation was attempted.
- A1 remains open until the active-generation ACL proof and state-preserving stream migration path are resolved, the merged change is safely deployed, and remaining staging acceptance checks pass. Do not start A2 before A1 closeout.

## 2026-10-01 post-reload attachment inspection

- Reloading the existing staging browser restored `@xronos2#6340` and its inbox; the earlier empty-list/reconnect symptom did not recur in this observation. The visible Quick Access rail still contains 15 entries.
- The `q12` conversation contains the existing outgoing `debug.md` attachment card (11.4 KB, marked Read) after reload. The card is exposed as plain text in the accessibility tree; clicking its icon/name did not start a download. Chat Info → Files shows “Nothing here yet” despite the visible card.
- No message, file, or conversation data was changed. A recipient download and same-content hash comparison remain unverified, and no File-service restart was attempted without a downloadable baseline and confirmed target metadata. Keep attachment persistence/download/hash acceptance open.

## 2026-10-01 attachment download fix verification

- Added a tap target for ordinary document attachments. Tapping requests a fresh File-service presigned URL; HTTPS is accepted everywhere, and plain HTTP is restricted to local development hosts (`localhost`, loopback, or `host.docker.internal`). Failures show a localized message. Encrypted attachments keep their existing decrypt-and-save flow.
- Added widget regression coverage for HTTPS staging URLs and the HTTP `host.docker.internal` local endpoint. The full attachment test file passed (4 tests), targeted Flutter analysis passed, and `make flutter-ci` passed all checks, including 1,195 passed and 98 skipped Flutter tests; analysis reported nine informational findings accepted by the CI `--no-fatal-infos` setting.
- Graphify extracted changed sources but remained idle after AST extraction on two attempts, so the graph finalization did not complete. CodeGraph had also reported its index frozen by a held file lock.
- The fix has not been deployed. Staging attachment download and same-content hash verification remain open behind the existing NATS staging gate.

## 2026-10-01 owner findings — local remediation batch

- The owner reported asymmetric friend removal, reset-password sign-in returning to email verification without a delivered code, a profile-setup modal shown for a regular account, stale unread/read state, difficulty finding archive, blocked-account messages visible in shared chats, and invite links that open onboarding without joining the Space.
- Local changes now fan out `friend_removed` to both profile peers and invalidate the Flutter Friends list; reconcile inbox snapshots after read/live activity; make the archive action available through each chat row's overflow menu; generate invite links from the current web origin and route successful joins to the returned Space; and skip the legacy save-account modal for regular accounts while keeping a separate reminder for guests. The archive action is already available and locally tested, so it is not a missing capability. Staging discoverability still needs verification. Realtime live delivery and Messaging history now hide a blocked sender from that viewer in group/channel chats while preserving reverse visibility. No staging account or content was modified for these code changes.
- Local verification passed: 78 Flutter tests across the affected onboarding, inbox, friends, archive, chat, and invite suites; targeted Flutter analysis found no issues; the Messaging short grpcsvc/s2s suites passed 124 tests; the full Realtime Go package passed 395 tests; the blocked-mention regression also passed under `go test -race`; and `scripts/ci/nats-realtime-bootstrap-contract-test.sh` passed after syncing the Compose and Kubernetes bootstrap consumer declarations. `git diff --check` is clean. These are branch results, not deployed acceptance.
- The current staging browser is still running the older build at `b126a62b502da1f7d8687116bdb74b9cdb43e84f`; a fresh screenshot of the signed-in `@xronos2#6340` session still shows “Set up your profile” for this regular account. The guest-only prompt fix and the other local fixes cannot be credited as staging-passed until deployed.
- The reset-password report is not yet isolated to mail delivery: Auth leaves the account email-pending after a reset when it was not previously verified, and sign-in requires verification. The user must use the explicit resend flow; capture the resend response and verify delivery before concluding whether the mail provider failed. Registration-code delivery is a separate path and its prior success does not prove reset verification delivery.
- An integration review found a second Realtime privacy route: `mention.added` had bypassed the new block filter for both its direct mention operation and personal notification. The regression first failed with both operations reaching the viewer who blocked the sender; Realtime now applies the same directional account decision to both fanouts. It also verifies an unblocked live mentioned profile still receives its direct operation without a chat subscription. The Realtime service contract now documents this scope. This remains local and requires staging re-verification after deployment.
- Tightening the onboarding failure assertion exposed duplicate legacy-step completion attempts for a regular account (two POST attempts in one visit). The overlay now records one attempt per active profile/session key; the focused onboarding suite passes 16 tests, including the pre-existing guest auto-skip one-shot case. This change is local and awaits deployment.
- The latest staging app-only deployment remains `b126a62b502da1f7d8687116bdb74b9cdb43e84f` (workflow [36837174161](https://github.com/Poryadok/VoiceRoot/actions/runs/36837174161)). Deployment remains blocked by the active NATS generation `r20260930a4`: no `STAGING_NATS_PROOF_CREDS_B64` is available and the active ACL proof hash does not match the changed ACL intent. Do not alter the proof or run a full deployment until the state-preserving migration gate is satisfied. Broader Space hierarchy expansion remains outside this A1 batch.
- A1 remains open. After the safe deployment gate is resolved, retest friend removal on both online clients; guest/regular onboarding; inbox preview, durable mark-read and unread separators; archive discoverability; invite joining from a second account; directional block visibility over REST and live WebSocket; password-reset verification resend; and attachment download/hash. Continue the remaining account, reconnect/history, offline/error, and soft-delete/recovery checks recorded above.

## 2026-10-01 user findings — local versus staging proof

The pasted acceptance findings are the source for the reported behavior. Preserve
their distinctions when recording results:

- Earlier local checks for friend-removal fanout, inbox/unread reconciliation,
  archive access, invite origin/join routing, regular-account onboarding,
  attachment download, and directional block filtering are recorded in the
  preceding owner-findings entry. Those checks establish local behavior only.
- The current integrated worktree adds a Social profile-pair block contract,
  title-only DM peer projection through User/Chat, unblock cache/history refresh,
  generated protobufs, and regressions. Focused local checks pass: 126 short
  Messaging tests plus 58 targeted history/deleted-peer/thread tests; 31 Chat
  `ListChats` integration tests; Social and User RPC regressions; five Flutter
  title/visibility widget tests plus the unblock suite's 30 tests; scoped Flutter
  analysis and protobuf lint/breaking/Dart generation checks. This is local
  evidence only: the changes remain uncommitted and have no exact-head CI or
  staging acceptance.
- Read-only staging inspection still shows the profile-setup modal on the
  deployed app. No controls were used, and this is not evidence for the local
  onboarding fix.
- The pasted findings still require live confirmation for two-client friend
  removal and request UI/REST state; password-reset resend and code delivery;
  profile switch with reconnect; per-chat unread refresh and channel read state;
  archive persistence; invite links that both use the configured web origin and
  successfully join a Space/group; attachment download and hash after restart;
  directional block behavior in existing DM/shared-chat history and Realtime;
  archive discoverability in the deployed UI; and soft-delete/recovery states.
  The owner could not independently inspect phone-book hash transmission,
  Auth-to-User contract isolation, durable read cursor boundaries, or
  cursor-based history after restart; keep these explicit acceptance checks
  open rather than interpreting “not observed” as proof.
- Broader Space hierarchy expansion remains outside this A1 batch. The active
  category and text-chat controls are included to exercise API-backed text
  channel creation in an existing Space.
- Staging is still on app-only deploy SHA
  `b126a62b502da1f7d8687116bdb74b9cdb43e84f` (workflow
  [36837174161](https://github.com/Poryadok/VoiceRoot/actions/runs/36837174161)).
  The full deploy remains gated on active-generation NATS ACL proof and
  state-preserving migration evidence for `r20260930a4`. No proof variable,
  root, namespace, or staging data may be changed to bypass that gate. The
  latest live observations recorded above apply only to the deployed build;
  they cannot validate the current local worktree.

A1 remains open. Close each acceptance item only with the evidence level that
was actually observed (local check, exact-SHA CI, or deployed staging flow).
Do not mark A1 accepted or start A2 until the migration gate and full A1 DoD are
complete.

## 2026-10-02 active remediation slices

The following four slices are active local work. Their state below is the
current proof boundary; none has staging acceptance yet.

| Slice | Local evidence/status | Staging acceptance |
|---|---|---|
| Incoming notification duplicate and selected-chat inbox reconciliation | Focused regression showed the defect (RED) and the fix passing (GREEN). The integrated current head still needs scoped verification. | Open: verify incoming request notification and selected-chat inbox/read/preview state on staging without duplicate or stale entries. |
| Category controls and Space text-chat creation | API-backed text-channel create flow is being covered; tests are in progress. Canonical tree reorder and category placement are being added. Do not mark verified yet. | Open: create a category and a text chat in an existing Space, then verify reorder and category placement in the resulting tree. |
| Web invite retention and native share origin | Preserve the original invite URL through unauthenticated login and nickname completion before joining; native share must use the configured application origin. Implementation/test status is not yet recorded. | Open: start from an invite while logged out, complete required account steps, confirm the same invite is joined, and verify the shared URL origin. |
| Pending-email login after password reset | Owner-requested behavior update: automatically resend verification on password-correct login when the account remains email-pending, using Auth's existing throttle. This was not part of the prior documented contract. Implementation/test status is not yet recorded. | Open: verify resend delivery/response and existing throttle behavior on the deployed build. |

Archive is available from chat-row overflow and has been tested locally. Keep
the staging discoverability check open: confirm a user can find and use that
overflow action in the deployed UI. It is not an unimplemented archive feature.

Review follow-up: blocking a peer while their shared chat is already open can
leave that peer's cached messages visible if the filtered REST history refresh
fails or the client is offline. A fail-closed cache scrub is being implemented;
add and pass a regression that seeds cached blocked-peer messages, fails the
refresh, and proves the messages are removed before display. Keep the behavior
open for staging until verified on the deployed build.

Category API gap: Space protobuf RPCs and target documentation define category
update/delete operations, but API Gateway has no PATCH/DELETE routes for them.
Category rename/delete therefore remain unsupported in the current UI/API path.
Keep this gap explicit unless the routes are implemented. The active local
scope adds canonical tree reorder and category placement plus API-backed text
creation; those items do not imply rename/delete support.

The current staging build and NATS gate remain as recorded above. Exact-SHA CI,
the active-generation ACL proof, and state-preserving migration evidence must
precede deployment; do not credit local RED/GREEN or in-progress tests as
staging proof.

## 2026-10-02 integrated owner-findings batch

The reported defects have local implementations and regression coverage on
branch `codex/a1-inbox-reconcile-fix`; the first implementation and evidence
commits (`6d1310ab9`, `4db8b6322`) are pushed to PR #596. The first code CI run
`36935069635` was superseded when the evidence-only commit arrived. It exposed
three findings now fixed locally; the follow-up code batch is not yet pushed and
none of this work has staging acceptance:

- Friend removal updates both participants; incoming requests, notifications,
  inbox preview, and unread/read state reconcile without a page reload.
- Invite URLs use the web page origin (or configured native share origin), are
  retained through auth/profile completion, and proceed to the joined Space.
- Password-correct login for an email-pending account resends verification via
  Auth's existing throttle. Live provider delivery and the deployed response
  remain unverified.
- Regular accounts no longer receive the guest save-profile prompt. The
  current staging page still displays that prompt for `@xronos2#6340`, confirming
  that this fix is not deployed.
- Space UI supports category creation, text-chat creation, category placement,
  and contract-limited reorder. Partial/uncertain network outcomes are guarded
  against duplicate creation. Rename/delete are still not available through
  Gateway's missing PATCH/DELETE routes.
- Ordinary attachment cards request a fresh download URL. Staging download,
  checksum comparison, and service-restart persistence remain unverified.
- Blocked peers retain their real title in an existing authorized DM; their
  group/channel history and Realtime deliveries are filtered directionally.
  Unblock refreshes without a full-page reload. Cross-profile/room cache writes
  are fenced and blocked messages are purged before API refresh, including
  offline/failing-refresh cases.
- CI found that `MarkRead` or resumed activity could silently restart all inbox
  scopes after one page failed. `reconcileAfterMutation()` now preserves that
  error and opaque cursor until the user retries; its new regression first
  failed by adding three unexpected requests and now passes without extra calls.
  CI also exposed stale unused Messaging fakes and one direct-service fixture
  without the mandatory Social profile-pair policy; both are corrected.

Integrated local verification on this branch:

- `make go-test-short-chat go-test-short-social go-test-short-user go-test-short-messaging go-test-short-realtime` passed.
- `make buf-ci buf-breaking` passed.
- Latest `make flutter-ci` passed: 1,239 passed, 98 skipped; analyzer reported
  eight informational lints accepted by CI.
- Latest `make golangci-ci` passed across all 20 Go modules. Focused inbox
  reconciler (16), notification/chat (40), Messaging short tests and Messaging
  lint passed after the CI-driven corrections.
- Focused privacy/history/cache tests passed (37); focused Space client/widget
  tests and targeted Flutter analysis passed; `git diff --check` is clean.
- `make ci-script-tests` could not run because `jq` is not installed on this
  host. This is an environment limitation, not a passing check.
- Realtime's full package suite previously passed 395 tests. Full Go integration
  suites remain limited by rootless Docker support on this host.
- Before cancellation, protobuf, docs links, admin, Flutter device-driver,
  backend package, most service tests, E2E and attachment proof passed on
  `36935069635`. Lint, the profile-handoff live test and Messaging integration
  failed; those findings are corrected locally. The run was canceled when the
  follow-up docs commit landed. Fresh exact-head CI is required.
- `make buf-go-pb-check` passed after regenerating the Go protobuf trees. The
  Graphify source graph update completed; its HTML visualization was skipped
  because the repository graph exceeds the visualization node limit.

Read-only staging observation on 2026-10-02: the existing signed-in session is
`@xronos2#6340`, the visible inbox loads, and the regular-account “Set up your
profile” modal is still shown. No staging data was changed in this observation.

Still open and not credited as passed:

- Commit/push the CI-driven fixes, then complete exact-head CI for PR #596.
- Safe exact-SHA staging deployment. Active NATS generation remains
  `r20260930a4`; `STAGING_NATS_PROOF_CREDS_B64` is unavailable and the ACL proof
  hash does not match current intent. Do not change proof variables, rotate
  roots, reset namespaces, or wipe staging data to bypass the gate.
- Live email verification delivery, invite join from another account, two-way
  friend removal/request UI, channel unread/read and durable cursor semantics,
  profile switch with reconnect, history catch-up after restart, archive
  discoverability/persistence, attachment download/hash/restart, directional
  block checks on live REST/WS, and soft-delete/recovery.
- Phonebook hashing and Auth-to-User credential boundary require contract/code
  evidence; the absence of a browser permission prompt is not proof of the
  absence of phonebook hash transmission.

A1 remains open. Continue after exact-head CI and only deploy once the active
NATS proof and state-preserving migration gate are satisfied.

## 2026-10-02 CI follow-up — inbox reconciliation

- CI run [36937915331](https://github.com/Poryadok/VoiceRoot/actions/runs/36937915331)
  found two remaining batch regressions: the profile-handoff check observed 12
  inbox requests where it expected 6, and a direct Messaging service fixture
  did not provide the required profile-pair block policy. The fixture now uses
  an explicit allow-all policy, matching the test's intent.
- Concurrent full reconciliations now share one in-flight three-scope snapshot.
  Activity received during pagination schedules one follow-up after current
  pages finish, preserving fresh preview and unread state without overlapping
  generations. The retry path follows the same rule; a failed scope is not
  implicitly retried.
- Regression tests cover a full snapshot and a scope-local retry with activity
  arriving during paging. They verify one follow-up refresh and the final
  server preview/unread rows. The inbox reconciler suite passes 18 tests.
- Final local checks for this batch: `make flutter-ci` passed (1,241 passed,
  98 skipped; eight informational lints accepted by CI),
  `make go-test-short-messaging` and `make golangci-ci` passed, and
  `git diff --check` passed. The exact-head GitHub CI run is still required.
- The GitHub staging environment's secret-name inventory confirms that
  `STAGING_NATS_PROOF_CREDS_B64` is absent. No deployment or staging data
  changes were made; the active-generation ACL proof and state-preserving
  migration gate remain unresolved.
