# TODO — Client

[← Индекс](../TODO.md)

Flutter/web/mobile, a11y, onboarding, deep links, guest UX (`src/frontend/`).

## Critical

_Пока пусто — критичные клиентские блокеры добавляйте сюда._

## High

### Mobile & push setup (**Вы**)

- [ ] **iOS Team ID** — `Runner.entitlements` associated-domains: заменить `TEAMID`; SHA-256 в assetlinks вместо `PLACEHOLDER` ([`DEPLOYMENT.md`](../DEPLOYMENT.md)).
- [ ] **Firebase / FCM prod** — `google-services.json`, web config в CI secrets; FlutterFire для staging/prod клиента.

### Deep links & mobile acceptance

- [ ] **Приёмка invite→join** — universal link открывает приложение / web fallback ([deep-links.md](../features/deep-links.md)).
- [ ] **Mobile device E2E** — App Links / custom scheme on Android/iOS still open. Host scaffold: `integration_test/device_driver_smoke_test.dart` + CI `flutter-device-driver` (#42); Chrome parser: `deeplink_web_test.dart`. Physical App Links / AASA / NT-05 remain.

### Guest & onboarding live

- [x] **[A1] Session-bound email verification recovery** — on reload,
  call `GET /api/v1/auth/verification-status`; resume `EMAIL_PENDING`, show
  bounded `PROMOTION_PENDING` retry, and persist a replacement `REGULAR`
  session before routing. Never resend email or submit it with verification.
  Evidence: `auth_providers.dart` (`restore`, `resumeEmailVerificationPromotion`);
  `auth_controller_test.dart` (restore/login recovery without resend or OTP
  replay); `a1_pending_email_reentry_e2e_live_test.dart` (logout/login and
  promotion retry). Shipped in #425/#457/#458/#460.
- [x] **Onboarding coach-marks E2E** — `onboarding_coach_e2e_live_test` (spaces/MM + invite deep link); guest: `guest_onboarding_e2e_live_test`; widget-якоря: `guest_onboarding_anchor_keys_test` / `onboarding_overlay_test`.

### Flutter delivery

- [ ] **UIV3-PANEL-SPACE-CREATE — templates and image upload** — section 17
  requires an image picker and starter templates that create default channels.
  The current creation contract accepts name, description and visibility, with
  an optional icon URL persisted separately through UpdateSpace. There is no
  Space-specific image upload client contract; User avatar presign belongs to
  the profile and cannot stand in for Space-owned media. ListTemplates and
  CreateFromTemplate remain unimplemented in Space Service. The supported panel
  explains these unavailable capabilities; completing them requires the
  documented Space/File ownership contract and real template producer/consumer
  under PLAN A5, followed by H/V and runtime verification. This subset does not
  complete the original creation-window acceptance.

- [ ] **Commit waves A–J (Flutter client)** — один PR: state/ui/l10n/tests из аудита 2026-07-15; после merge — `make flutter-ci` на CI.

### AttachMenu rich-action contracts

- [ ] **UIV3-PANEL-CHAT-ATTACHMENU — Location picker/preview contract** —
  [screen-controls.md §3.6a](../design/screen-controls.md#36a-attach-popup)
  requires a map picker; §3.3 requires a static-map bubble opening a maps app.
  The payload is defined (`lat`, `lon`, optional `label` and `static_map_file_id`),
  but the map/provider integration, selection and confirmation interaction,
  current-location permission/fallback behavior, and preview production path
  are not specified. [file-storage.md](../features/file-storage.md) permits
  either client-generated or server-generated previews without selecting a
  consumer contract. Define these decisions and H/V picker states before
  implementing the action; a coordinate form or placeholder menu entry is not
  an approved substitute for the required map picker.
- [ ] **UIV3-PANEL-CHAT-ATTACHMENU — Article and Music consumers** —
  [text-chat.md](../features/text-chat.md#article-vs-inline-url) requires Article
  server-side metadata fetching by Messaging, not client-side inline-URL
  preview. [file-service.md](../microservices/file-service.md) requires Music
  metadata extraction and a Messaging metadata-update consumer. Complete or
  verify these producers/consumers before exposing the corresponding picker
  flows; typed attachment validation alone does not prove them.

The original five-action ticket remains open. The shipped Photo/Document
subset has H/V and keyboard/focus evidence from PR #767 (Flutter run
38493294129, capture artifact 11863412351). The retained document fixture also
passed post-deployment and post-restart verification on final source
`33a78c8e2e0824e1eb34a3cfa8f294f9015189c1` on 2026-10-15. These results establish
the supported subset, not the three missing actions or their visual acceptance.
This gap record does not advance the active milestone or waive the five-entry
DoD; see [PLAN.md](../PLAN.md) A6 and G1's unfinished-rich-type limitation.

### Game SDK authorization (T14 client slice)

- [ ] **T14 integration gates remain open** — the Flutter consent/callback
  consumer uses the existing Auth authorization routes only. The compact proof
  signer is an adapter keyed by Auth-issued `kid`; the T15 device lifecycle
  must provide platform secure-store P-256 enrollment, rotation/revocation, and
  signing of the exact payload bytes. Private keys must remain non-exportable.
  GAME-AUTH-02 defines no deny endpoint, so Cancel currently abandons the local
  screen and lets the Auth request expire; no remote denial is claimed.
  Gateway Auth route/principal publication is implemented by the initial T20
  Auth-consumer slice; broader route scopes, public-client conformance and T20
  integration remain downstream. Consent and linked-session rate quotas still
  use the existing AuthOAuth limit by the derived T20 decision. GIS active
  binding/T16, direct Voice-only first-login
  variant, device-code flow, and live Google/Q11 provider proof remain open.
  The client does not enumerate profiles beyond the authenticated user's own
  `/api/v1/users/profiles` response and requires a deliberate eligible-profile
  selection. Auth revalidates that profile and policy revision at approve and
  exchange.
### Multi-profile

- [x] **[Multi-Profile] No delete-profile UI** — `ManageProfilesSheet` in settings (`settings_manage_profiles`); `DELETE /api/v1/users/profiles/{id}` via `VoiceUsersClient.deleteProfile`; blocks primary/active delete (Batch 13).
- [x] **[Multi-Profile] Frozen profiles invisible in switcher UI** — `VoiceProfile.frozenAt` + `proto_mappers.dart`; `ProfileSwitcher` disabled items + `(Frozen)` label; `ProfileAvatarSwitcher` skips frozen on swipe (Batch 11).
- [x] **[Multi-Profile] Second profile creation flickered and appeared to lose the switch** — `profile_switch_entry_wiring_red_test.dart` reproduces the mounted desktop rail → create → switch transition with the real theme provider: the routed shell disappeared while the profile theme reloaded. `VoiceApp` now retains the last theme during reload; the regression test verifies that the shell and sheet remain mounted through handoff and that the rail displays the new active profile after completion.
- [x] **[Multi-Profile] `ProfileDowngradePickerScreen` unreachable** — **done (Batch 12):** routed when free tier + >2 profiles (`profileDowngradeRequiredProvider` in `app.dart`); screen + `submitDowngradeProfiles` existed.


### Auth UI (REST есть, экранов нет)

- [x] **AuthScreen: email registration + OTP** — email is the alpha default in [auth-and-contacts.md](../features/auth-and-contacts.md); `auth_screen.dart` resumes authoritative pending-email status after registration, login, and reload, and supplies verify/resend, bounded promotion retry, and logout. The initial authenticated send remains owned by Auth and is never replayed by recovery.
- [x] **Auth UI: password-reset** — `PasswordResetScreen` (email OTP → new password); forgot-password on `AuthScreen`; `VoiceAuthClient.sendPasswordResetOtp` / `resetPassword` (**Batch 30a**). **Sessions/revoke UI shipped** (`ActiveSessionsScreen`, Security settings, **Batch 29b**).
- [x] **Auth UI: delete-account** — `SecuritySettingsScreen` confirm+password → `POST /api/v1/auth/delete-account`; guest blocked; logout on success (**Batch 28b**). Restore-account UI deferred.
- [ ] **Verification OAuth callback and client resume** — Implement the default-off [transport v1 contract](#oauth-callbackresume-transport-contract-v1--target) below across Auth/Gateway/Flutter and prove Web/Windows resume, bound state/PKCE, replay/cancel/expiry and authoritative rereads. Exact registrations/origins/secrets are deployment configuration. YouTube positive membership+verified-channel evidence remains a separate qualification gap; long-upload permission must not grant/revoke a badge. No provider is enabled by this documentation. See [verification.md](../features/verification.md).

#### OAuth callback/resume transport contract v1 — target

Tracking: `UIV3-SETTINGS-OAUTH-RETURN-CONTRACT`, related to
`UIV3-PANEL-SETTINGS-LINKEDACCOUNTS`. Consumer: the Settings linked-account
callback implementation. This defines the target transport contract and test
plan for a default-off implementation; it does not claim implemented behavior,
provider enablement or acceptance. The open checkbox above remains open for
implementation/platform proof and YouTube qualification. Auth owns provider
credentials/identity checks; User owns the resulting public projection. Exact
provider registrations, origins and secrets are deployment configuration, not
prerequisites for implementing the protocol against a fixture.

**Observed source baseline.** Inspected at `1d18297eccead5898767f903f47b5f08fbb847c7`;
the separately inspected Settings PR [#906](https://github.com/Poryadok/VoiceRoot/pull/906)
was at `e12dc4f96e5ddb1ed4c92595428fcb038549bc1a`.

| Surface | Current fact and source |
| --- | --- |
| Auth REST | `AuthRestController` exposes authenticated `POST /api/v1/auth/linked-accounts/{twitch,youtube}/link`, returning `{authorization_url}`. The request accepts `redirect_uri` and optional client `state`; defaults use `https://app.voice.test/oauth/{provider}`. Start validates the bearer but does not persist an account/profile-bound attempt. |
| Callback | The existing authenticated `POST .../{provider}/callback` accepts `{code,redirect_uri}` and uses the **callback JWT's** account/profile; returns `{verification_type,badge}`. It has no state/verifier field or server attempt consumption. It is not a public provider GET callback. |
| Provider exchange | `LinkedAccountsService.buildTwitchAuthorizeUrl` / `buildYoutubeAuthorizeUrl` append optional state; `exchangeAuthorizationCode` sends client ID/secret, code, grant type and redirect, without PKCE. Twitch requires Partner; YouTube currently uses `longUploadsStatus` allowed/eligible as a YPP proxy. The proxy is implementation evidence, not proof of the full documented verified-YPP criterion. |
| Development fallback | `exchangeTwitchCode` / `exchangeYoutubeCode` use mock tokens for `mock-code` **or unconfigured providers**. A production callback must explicitly exclude this fallback; a returned authorization URL is not evidence of a configured provider. |
| Gateway | `transcode_profiles_verification.go:serveAuthREST` deliberately returns false for linked-account GET/list and POST subroutes, leaving them to the configured Auth REST upstream. New public GET callback and result/cancel routes need explicit authentication, routing and rate-limit classification; no gRPC callback contract exists here. |
| Flutter API/UI | `auth_client.dart:startLinkedAccountLink` sends only redirect URI; there is no linked-account callback/resume wrapper. Baseline sheet displays the URL; PR #906 launches HTTP(S) via external `url_launcher`, adds confirmation and session/profile fences. Both use the test callback origin. Browser handoff does not establish callback success. |
| Return plumbing | `routing/deep_link_listener.dart` reads initial and streamed app links; `deep_link_parser.dart:DeepLinkKind` has no OAuth target and rejects an OAuth path. Initial web recovery is invite-specific. Android manifest declares `voice:` and `https://voice.gg`; iOS Info.plist declares `voice:`. These declarations do not qualify OAuth return or mobile association/signing. Windows runner forwards process arguments but does not itself implement OAuth result handling. |
| Existing product rules | [verification.md](../features/verification.md), Auth service §Linked provider verification and screen-controls §§27–28 require selected-profile sources, explicit unlink→link for another profile, authoritative User status and LinkedAccounts→Verification navigation. [PLAN.md](../PLAN.md) marks callback/resume and live-provider acceptance outstanding. |

**Transport protocol v1.** Auth/Gateway service docs reference this contract.
Do not reuse Admin/Developer Portal OAuth or SDK linking:
those are different clients, credentials and trust boundaries.

1. Flutter generates a cryptographically random return verifier (RFC 7636 S256,
   43–128 unreserved characters), retaining it only for this attempt. Authenticated
   `POST .../{provider}/link` accepts `{client_surface,return_challenge,
   return_challenge_method:"S256"}`. No arbitrary callback URL is accepted.
   Auth derives account/profile/session authority from the bearer, validates
   current eligibility and provider availability, and maps surface to configured
   exact callback and return endpoints. Response:
   `{attempt_id,authorization_url,expires_at}`. Missing provider configuration
   returns unavailable before launch, with no mock grant. Authorization URLs use
   the configured provider's exact HTTPS origin/path; Flutter rejects unexpected
   origins/schemes before launch, rather than trusting every HTTP(S) URL.
2. Auth creates a durable attempt with random server `state` (at least 256 bits;
   store only its hash), account ID, profile ID, provider, initiating access JWT's
   `jti`, `session_epoch` and `exp`, environment/client registration, exact HTTPS
   provider callback,
   exact allowed app return destination, return challenge and expiry. No IDs,
   redirects or credentials are encoded in state. Attempt lifetime defaults to
   **300 seconds**, capped by the initiating JWT's `exp`, measured by server clock,
   never extended by polling/reload. For a given initiating `jti`/profile/provider,
   a new attempt supersedes an older pending attempt; independent providers remain
   distinct. Existing active
   source/profile conflict rules still apply at final commit.
3. Provider PKCE is a separate layer from the return proof: Auth generates and
   retains a provider verifier, sends `code_challenge_method=S256` and challenge,
   and includes the verifier in server token exchange. Both Google and Twitch
   advertise S256 in their primary issuer metadata:
   [Google discovery](https://accounts.google.com/.well-known/openid-configuration),
   [Twitch discovery](https://id.twitch.tv/oauth2/.well-known/openid-configuration).
   The advertised authorization/token endpoints match the existing Auth defaults.
   Require provider S256 and independent return S256, with no plain method,
   fallback or runtime downgrade; unknown provider metadata/capability fails closed.
   Metadata establishes protocol capability, not a successful live registration
   or exchange. Neither verifier nor client secret reaches a return URL. The
   inspected linked-account implementation currently implements neither layer.
4. Public `GET /api/v1/auth/linked-accounts/{provider}/oauth-callback`
   terminates HTTPS at Gateway and executes in Auth. No Voice bearer is expected
   from the provider browser. Accept exactly one state and either one nonempty
   code or one provider error; reject duplicates, mixed code/error, missing state,
   oversized/malformed parameters. URL fragments are not sent in HTTP requests;
   Twitch's documented singleton `scope` (success) and `error_description` (denial)
   are bounded inert metadata: ignore them without trust, persistence, logging,
   reflection, scope expansion or forwarding. They cannot replace required state
   or code/error inputs; duplicated metadata and unknown keys still reject.
   See [Twitch authorization-code callbacks](https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/#authorization-code-grant-flow).
   Gateway/Auth cannot inspect or reject a browser's fragment in this GET.
   Validate state, provider path, registration, bound callback and expiry before
   any token request. Also revalidate the stored original `jti` against revocation,
   original credential expiry, current active account/epoch and owned eligible
   profile; missing/unavailable authority fails closed. Use the **stored** callback
   for exchange; never Host,
   forwarded headers or request-supplied redirect. Atomic pending→processing
   claim allows one exchange across concurrent callbacks. Unknown/wrong state
   cannot cancel or consume another attempt and gets a neutral failure page.
5. Auth exchanges the code and checks provider identity/criterion. It records an
   encrypted short-lived pending result, **without linking a source yet**. This
   ensures final linking requires a current authenticated client and its return
   proof. Result handle: independent random opaque 256-bit value, hashed
   at rest, bound to the whole attempt tuple, single use, expires at the earlier
   of attempt expiry or **60 seconds after callback completion**. Tokens never
   appear in result JSON/URLs/logs; pending token material is deleted on expiry,
   cancellation, denied result or successful finalization. Provider failure gets
   a safe outcome enum, never raw provider descriptions. Callback replay cannot
   mint another handle or trigger another exchange. A worker crash/ambiguous token
   exchange ends the attempt safely; retry starts a new attempt rather than
   replaying the authorization code. The callback/bridge use `Cache-Control:
   no-store`, `Referrer-Policy: no-referrer` and a restrictive CSP without third
   party resources. Gateway/Auth access logs, tracing and analytics redact codes,
   state, handles and tokens. Provider codes enter only the server HTTPS callback;
   the bridge never forwards its original query to the app or another origin.
   Every callback/bridge HTTP redirect, including error redirects, constructs its
   allowlisted `Location` with an explicit empty fragment (`#`), preventing the
   browser from inheriting the incoming fragment across redirects
   ([RFC 9110 §§10.2.2, 17.11](https://www.rfc-editor.org/rfc/rfc9110.html#section-17.11)).
   Before analytics, logging, result consumption or OS handoff, the browser bridge
   rejects any nonempty `location.hash` and scrubs it via history replacement;
   Flutter likewise rejects a nonempty fragment in a directly delivered return
   URI and clears it from recoverable URL state. Neither component records the
   fragment or copies it into messages, return links or app-launch arguments.
6. Return only `{attempt_id,result_handle}` through a configured bridge/app
   destination; no provider code, provider token, Voice token, verifier, arbitrary
   next URL or claimed success status. Authenticated
   `POST .../{provider}/results/consume` body is
   `{attempt_id,result_handle,return_verifier}`. Auth requires the original
   account/profile/session authority, current epoch/eligibility and S256 match;
   mismatch, expiry, supersession, logout or revoked session fails closed without
   consuming another owner's valid result. Profile switching never rebinds or
   silently switches back. The incoming bearer must carry exactly the original
   `jti`/account/profile/epoch; no migration to a refreshed or newly signed JWT.
   Auth rechecks the stored original `jti` revocation, account/epoch and eligibility
   before processing and final commit. Local session replacement, profile switch
   or logout discards pending proof and delayed responses; recovery starts a new
   attempt. Flutter obtains a fresh access token before start when needed and
   never transparently refreshes/retries consume with a new identity. Existing
   `TokenClaims` has no stable refresh-family ID; `AuthService.consumeRefresh`
   revokes the old access JTI. This bound implements today's authority model
   without inventing session lineage.
7. For a verified pending result, final source persistence, durable verification
   sync target and result consumption commit atomically; serialize against unlink
   and conflicting link. Recheck owner/account activity at this commit boundary.
   The existing source revision/CAS prevents stale Auth→User sync from overriding
   a later source revision. `success` means the source was committed, not that
   User's projection has already converged. For denied/cancel/error outcomes,
   consume the result without mutating any source. Response:
   `{outcome}` with `success|cancelled|denied|provider_error|unavailable`.
   Replays receive `409 result_consumed` and do not reapply writes. Expired
   attempt/result receives `410 result_expired`; bad binding/proof receives
   generic `400 invalid_link_attempt`; authentication remains `401`.
8. Flutter clears the URL handle before navigation/analytics and rereads
   `GET /api/v1/auth/linked-accounts`, filtering the current profile, plus the
   existing User verification status endpoint. Open Settings→LinkedAccounts;
   its Verification status action opens Settings→Verification (§27). Source
   success with pending User sync displays “updating verification”, preserving
   actual status until reread confirms it. Transport failure after consume may
   have committed: reread state; never retry provider exchange or infer success
   from a handle/launch/HTTP redirect. Failure to reread displays retryable unknown
   state without replacing the last confirmed source/status with an empty list.

**Platform return and recovery.** Web uses an exact configured HTTPS same-origin
Settings return route. Prefer a full-page browser redirect so pending proof in
per-tab session storage survives reload; opener messaging is optional notification
only, with exact origin/target validation. If opening a separate browser tab,
the original tab must retain the proof and use a reviewed same-origin bridge or
authenticated attempt lookup; do not depend on copied session storage. Windows
uses the system browser, then a bridge with an explicit “Return to Voice” action
and a registered exact app return route carrying only the opaque handle. Persist
its proof in protected per-user temporary storage until expiry; test both a warm
instance and cold start and duplicate OS deliveries. If OS launch is unavailable,
the bridge offers a safe Settings/web recovery path; it never switches the
attempt's bound client or copies provider credentials. Mobile keeps the same
HTTPS server callback and returns via a separately qualified universal/app link
or reviewed custom-scheme handle route. Android/iOS manifest presence alone is
insufficient; signing/association and physical cold/warm-device checks remain
required. No mobile availability claim follows from Web/Windows completion.

| State | Required display and recovery |
| --- | --- |
| Waiting / browser launched | Waiting for provider; never connected/verified. Close browser has no reliable callback signal. Return to Settings rereads; expiry offers a new attempt. |
| Success | Show linked source only after authoritative reread. Verification may still be updating; navigate to Verification through the documented action. |
| Local cancel / close Settings | Authenticated `POST .../{provider}/attempts/{id}/cancel` atomically cancels pending/processing/result-ready work and discards staged tokens. If final commit already won, reread actual linked state; do not claim cancel undid a link. Late callback cannot apply a cancelled attempt. |
| Consent denied / criterion denied | Provider `access_denied` maps to cancelled; failed Partner/YPP criterion maps to denied. Neither creates/revokes a source. Explain safely; offer a new attempt. |
| Provider error / unavailable | Error/timeout/configuration unavailable are distinct safe states; retain all prior sources/status. Retry creates fresh state/proofs. |
| Reload / killed app / consumed result | Recover only matching unexpired protected attempt/proof. If absent, wrong account/profile, expired or already consumed, discard handle and reread. Offer restart; no automatic regrant/relink. |

**Discriminating test matrix for the implementation consumer (not executed by
this documentation task).** Use stub IdPs and real Gateway/Auth/Flutter boundaries
first; successful stubs cannot establish live provider qualification.

| Check | Expected assertion / layer |
| --- | --- |
| Happy Twitch / YouTube | Correct provider code→server exchange→criteria→authenticated consume; one source and one revision. Gateway route plus Auth integration and Flutter return test. |
| State/account/profile/provider/environment | Missing/random/swapped state; callback to other provider; result consumed with other account/profile/session/registration: zero exchange for invalid callback, zero source mutation for invalid consume. Valid unrelated attempt survives. |
| Redirect / open redirect | Reject attacker origin, suffix/subdomain trick, userinfo, path/port/query changes, encoded separators and unregistered return surface. Stored callback used verbatim in exchange; no untrusted Location. |
| PKCE | Wrong/missing/plain-method return proof rejects; correct S256 consumes once. Both provider stubs require matching S256/verifier; wrong/missing provider verifier fails even with a valid return proof. Unknown support does not silently downgrade or enable. |
| Replay / races | Duplicate concurrent callback exchanges once; duplicate consume writes once; old callback after cancel/supersession/logout/expiry writes nothing; unlink-vs-consume has a serialized outcome and later revision wins. |
| Lifetime / credential replacement | Defaults300s/60s capped at original JWT exp; equality means expired. Polling/reload cannot extend TTL. Refresh blacklists original JTI and new bearer cannot migrate the attempt; recheck original revocation/epoch before callback exchange and final commit. Expired/cancelled pending tokens/proofs are removed; independent providers do not consume each other's results. |
| Provider / criterion failure | Consent deny, malformed callback, non-200 token reply, timeout, missing token, non-Partner, YPP-proxy rejection: safe distinct UI and unchanged prior sources. Unconfigured production provider cannot use mock fallback. |
| Auth/User degradation | Source commit with User outage keeps durable sync pending; no false verified badge. Consume response lost after commit plus reload rereads actual state without another write. |
| Web browser | Real user gesture launch/redirect, exact callback/return, per-tab proof, reload before/after consume, browser Back, two tabs, blocked popup and missing opener; inspect URLs/history/network for credentials. |
| Fragment inheritance / direct delivery | Real browser regression starts at a callback URL with a synthetic credential-bearing fragment: assert it is absent from the HTTP request, every redirect `Location` has explicit empty fragment, and the resulting bridge/app URL inherits none. Direct bridge and Flutter return URLs with nonempty fragments must be rejected and scrubbed before analytics/consume/OS handoff; assert no fragment in history recovery, messages, logs or captured app-launch arguments. |
| Windows | Packaged protocol registration; external browser→warm/cold instance→Settings; duplicate delivery, expired proof and no handler. Real OS handoff is separate from widget mocks. |
| Mobile / UI safety | Android/iOS physical cold/warm return after association/signing; profile switch/logout at every await ignores stale responses; focus restored, safe localized messages and keyboard/accessibility states. |

**Configuration and persistence decisions.** Each provider and client surface is
default-off. Auth configuration contains exact provider authorization/token
endpoints and S256 capability, provider client ID/secret reference, absolute HTTPS
callback URL, and per-surface exact return endpoints. Callback URLs must have no
userinfo/query/fragment; production uses HTTPS, without wildcard matching or
origin inference from request headers. Web return is a same-origin dedicated
Settings return route; Windows uses its explicitly registered app route through
the HTTPS bridge; mobile return remains disabled until association/handler proof.
The API paths and payloads above are fixed; origins, app registration and secrets
are supplied later by deployment. Changed configuration revision invalidates old
attempts rather than rewriting stored bindings. Disabled providers require no
secrets and return unavailable; enabled incomplete/invalid configuration fails
startup. An explicit test fixture may use its own loopback endpoints and keys;
production must not select those through missing configuration.

Attempt/result TTL configuration uses positive ISO-8601 durations, default
`PT300S` / `PT60S`, bounded respectively by 300s / 60s and always by the original
JWT expiry. Equality with server `now` means expired. These are engineering
defaults selected from `SdkAuthorizationService.start`'s 300-second authorization
and `AuthProperties.OAuthClientSettings`/Auth service's 60-second OAuth code
conventions; neither is a claim that linked-account TTLs already exist.
Auth-owned PostgreSQL holds attempt/state/result hashes, original credential
bindings, callback processing claim, encrypted staged provider tokens and terminal
status; no cross-database FK or provider token reaches User. Record consumption
and source/sync-target mutation in one Auth transaction. A cleanup worker runs
at least once per minute to remove expired/cancelled staged token material.
Revocation/epoch validation immediately fences further processing and consume;
physical token deletion after logout is not claimed synchronous without a hook.
Local proof is cleared immediately on session replacement; failed cancel delivery
does not imply server cancellation and cannot restore or extend the attempt.

Gateway routes all listed methods to Auth REST explicitly. Start, result consume
and cancel require verified bearer account/profile context and `AuthOAuth` limits
keyed by account; the public provider callback has no bearer requirement and uses
the same existing `AuthOAuth` limit keyed by IP. Current Gateway convention is
30 requests per 15 minutes. Auth still validates every stored credential binding;
Gateway's public-callback exception is no authorization to link.

**Actual qualification gap: YouTube.** The primary
[Channels REST reference](https://developers.google.com/youtube/data/reference/rest/v3/channels)
defines `longUploadsStatus` only as permission/eligibility to upload videos longer
than 15 minutes. `isLinked` is public YouTube identity, not verification/YPP.
The official [YouTube discovery schema](https://www.googleapis.com/discovery/v1/apis/youtube/v3/rest)
(revision `20261022`) and [generated ChannelStatus SDK](https://googleapis.dev/dotnet/Google.Apis.YouTube.v3/latest/api/Google.Apis.YouTube.v3.Data.ChannelStatus.html)
also expose `isChannelMonetizationEnabled`, described as YPP monetization enabled;
the public REST prose does not define its availability or absent/false semantics.
Google's [paused monetization guidance](https://support.google.com/youtube/answer/10269388?hl=en)
states a paused channel can remain in YPP. Thus neither upload eligibility nor
absent/false monetization evidence is a documented proof of membership loss.
The feature additionally requires a verified channel but does not distinguish
[phone verification](https://support.google.com/youtube/answer/171664) from an
[official channel authenticity badge](https://support.google.com/youtube/answer/3046484)
or OAuth control. These are separate claims; neither OAuth `mine` nor upload
eligibility establishes the intended verification kind. YouTube qualification remains
`unavailable` until its positive membership+verification evidence and recheck
negative/unknown mapping are specified. Do not grant or revoke a source using
the unrelated long-upload proxy, assume a missing monetization field means
denied, scrape YouTube Studio, or silently change the product criterion. Unknown
evidence preserves prior source state per the existing outage rule. Any
remediation of historical proxy-issued badges needs an explicit correction
policy; it cannot be disguised as provider-confirmed criterion loss.
This gap does not block default-off transport implementation or the documented
Twitch Partner check. UI status/org work also proceeds independently.

Smallest next implementation stage: one stub-IdP vertical slice covering Auth
attempt/result persistence and atomic consumption, explicit
Gateway routes, and Flutter Web+Windows return/reread with the matrix above.
Use the target Auth/Gateway/data-store contract and write boundary-negative tests
first;
replace or block the legacy unbound POST callback so it cannot bypass the new
flow. Keep providers unavailable outside the qualified fixture until a separate
live-provider acceptance stage; no production switch or migration is authorized
by this target contract.

### Telegram-parity UX audit — Flutter (2026-08-28)

Источник: `tmp/telegram-ux-audit/AUDIT.md` (R2-A03–A05, R3-A08, R3-A11). Backend RPC-хвосты — [backend.md](backend.md) § Telegram-parity audit / § Chat navigation.

- [x] **R2-A03 — Profile switcher vs rail contract** — `ProfileAvatarMenuButton` §1.1a in rail (`profile_avatar_menu.dart`); desktop session bar is label-only (no combobox); mobile tap opens menu + swipe switch (`ProfileAvatarSwitcher`). Archive via profile RC → `/chats/archive` (`ChatArchiveScreen`, Batch 16).
- [ ] **R2-A04 — Mobile shell IA** — `MobileShellTabBar` (Chats/Social/Match) on `Scaffold.bottomNavigationBar` when narrow + no open chat (Batch 10 incremental); **incremental:** `MobileShellDrawer` hamburger stub (folders, QA, settings) shipped (parallel/client); **keyboard-hide tab bar** on narrow + open keyboard shipped (Batch 21b); **keyboard-hide active strip** on open chat shipped (Batch 28a); **incremental (Batch 29a):** pinned bar single-line collapse when >1 pin, text-scale ×1.5 smoke, strip hidden on full-screen shell overlays (MM/settings routes via `MobileShellOverlayObserver`); **deferred:** remaining stacked chrome polish.
- [x] **R2-A05 — Active strip LRU semantics** — strip tracks **opened** chats (≤100 LRU); `MobileChatStrip` + `mobile_opened_chat_strip.dart` shipped; long-press remove + 100-cap LRU eviction snackbar shipped (Batch 9).
- [x] **R3-A08 — Strip widget test wrong contract** — `mobile_chat_strip_test.dart` asserts opened-chat LRU, not inbox rows (§1.6).
- [x] **R3-A11 — Composer a11y parity** — transient §3.6b emoji panel + §3.6a attach popup with §3.6e focus trap/return (`composer_panels.dart`); click/tap activation on desktop (Batch 9).
- [x] **Archive list screen** — `Screen / Chat / Archive` via profile RC → `/chats/archive`; `ChatArchiveScreen` + `inbox=archive` list/unarchive (Batch 16).
- [ ] **Mobile stacked chrome** — active strip + app bar + pinned bar + composer + bottom tabs overflow rules §1.6a — Penpot · v3 + Flutter layout.


## Common

### Growth & accessibility



Baseline onboarding/deep-links/a11y — [PLAN.md](../PLAN.md); остаток vs [deep-links.md](../features/deep-links.md), [onboarding.md](../features/onboarding.md), [accessibility.md](../features/accessibility.md).

**Flutter UX waves A–J (2026-07-15):** закрыто в рабочей копии; остаток — commit PR + серверные хвосты ниже.


- [x] **Quiet hours: client dual-write** — `notification_settings_screen` reads/writes `Get/SetQuietHours` API; `notification_quiet_hours_storage.dart` is offline cache only (API-first load, local fallback on failure). Live: `quiet_hours_e2e_live_test` (NT-04).

- [x] **A11y: message list keyboard nav** — `↑`/`↓`, `R`, `E` wired in `voice_shortcuts.dart` + `chat_room_panel`; widget tests in `voice_shortcuts_keyboard_test.dart` (Batch 9).

- [x] **A11y: text-scale ×1.5 — расширить smoke** — `shell_text_scale_test.dart`: settings sheet, notification settings, game catalog, story archive; chat list/room remain in `chat_text_scale_test.dart` (Batch 10).

- [x] **A11y: focus return после modal** — `VoiceFocusReturn` + coach-mark `VoiceFocusTrap`; call overlay restores on dispose; `showVoiceBottomSheet` escape restores focus (Batch 10).

- [ ] **A11y: pre-release TalkBack / VoiceOver** — чеклист в [accessibility.md](../features/accessibility.md) §Pre-release; ручная приёмка перед mobile release (**Вы**).

- [x] **A11y: Axe / web accessibility CI** — landmark contract `make a11y-web-axe` (Axe analog) in flutter CI; full CanvasKit Axe deferred.

- [x] **VoiceListSkeleton / VoiceStatePanel — wave A residual loaders** — PR #128: `chat_room_panel` pagination, space members/bots/invites, `player_profile_sheet`, `story_viewer_screen`, settings/search/subscription list loads → skeleton/state panel; `VoiceListSkeleton` uses `shrinkWrap: true` + `primary: false` (sheet CI).


- [x] **VoiceListSkeleton + VoiceStatePanel widget tests** — `src/frontend/test/voice_state_components_test.dart`: `VoiceListSkeleton renders configured placeholder rows`; `VoiceStatePanel` empty-state semantics, keyboard retry action и suppression неполного action.

- [ ] **api_error_messages — extend residual coverage** — chat-room history now maps unknown upstream text and `permission_denied` to localized safe states; PR #128 helpers still cover chat/search/settings/subscription/space bots|members|invites/player profile. Audit the remaining action and loader surfaces individually before claiming that raw API strings are absent globally.

- [ ] **VoiceDisabledAction — остаточные permission-gated действия (CL-010)** — Space text chat проверяет существующий `TEXT_CHAT_MANAGE_MESSAGES`; согласно [text-chat.md](../features/text-chat.md), пункт удаления чужого сообщения скрыт, пока право явно не подтверждено, а pending/denied/error показывают только нейтральный информационный статус без команды Delete. Проверка не включает модерацию standalone-групп, пока не закрыт контракт BE-089. Остаток требует продуктового канона: у Flutter нет UI создания voice room и Role не называет право создания ([screen-controls](../design/screen-controls.md) §10.3 указывает только «Has right»); `matchmaking.md` не определяет запрет действий для guest-аккаунта. См. также [roles.md](../features/roles.md), [matchmaking.md](../features/matchmaking.md), [auth-and-contacts.md](../features/auth-and-contacts.md).

- [x] **MobileChatStrip — scope на full-screen фичах** — strip скрывается при `mobileShellOverlayDepthProvider > 0` (PageRoute поверх shell: MM catalog/search, settings sub-routes); `shouldShowMobileChatStrip` + `MobileShellOverlayObserver` (**Batch 29a**).

- [ ] **chat_info_panel notification tile — narrow E2E** — compact layout; overflow поправлен в widget-тестах; live/compose E2E на узкой ширине (связано с Critical Batch 2 при наличии стека).


### Chat UX (спека text-chat / navigation)

- [x] **Chat switch flicker: chat area flashes and a blue line appears when switching chats** — fixed in `ChatRoomPanel`: empty initial history now renders a visible skeleton and the room no longer draws a blue progress line. `chat_room_panel_test.dart` taps chat B from `ChatListBody` while chat A is open inside `ThreeColumnShell`, verifies navigation remains mounted and the conversation stays visible without a progress line, and fails on the pre-fix code. Local app-stack visual confirmation remains an optional follow-up.
- [ ] **Локальные черновики** — Hive/SQLite, один на chat; multi-device sync отказано в спеке.
- [ ] **OG link preview** — unfurl на клиенте; сервер не обязан.
- [ ] **Stickers / GIF / voice-note composer** — full EmojiPicker §3.6b remains open. Emoji insertion and profile session recents work; the picker browses installed sticker metadata and uses BE-085's public GIF search/trending API with opaque pagination and retry. No live provider is activated: unconfigured search remains unavailable. Sticker asset previews, media recents and first-class immediate sends still require authorized catalog import/readiness and destination-message File reference acquisition. Until that path is implemented, media send remains disabled even when a search response contains `file_id`; sticker tiles show unavailable-image placeholders with associated emoji metadata. Current Messaging validates destination `chat_id` on ordinary files, while catalog assets are reusable. Do not substitute text/ordinary attachments or weaken that ACL. File processing alone does not establish Messaging admission; E2E catalog identity/encrypted-file behavior needs its separate contract. Voice-note capture remains separate. See [backend.md](backend.md), [messaging wire](../microservices/messaging-service.md#stickers-and-gif-cross-service-wire-contract) and [product-roadmap.md](product-roadmap.md).
- [x] **Message requests inbox UI** — virtual «Запросы» folder in rail/drawer (visible when pending > 0, unread badge); removed middle-column segmented toggle (§1.3 tombstone); accept/decline on list rows; `notificationTypeMessageRequest` settings toggle — **Batch 22b** (backend bucketing Batch 21a).
- [x] **Кастомные папки чатов** — All/DMs/Groups + custom; REST folders + rail/drawer UI shipped (parallel/client); **pin/reorder UI** in list ctx + custom-folder drag reorder shipped (Batch 21b); edit-folders management UI shipped (**Batch 25b**).
- [x] **In-chat search: next/prev highlight** — `in_chat_search.dart` `_step`
  wraps active selection in both directions and renders the active-hit and
  snippet highlight keys; `in_chat_search_test.dart` covers next/previous
  selection and snippet highlighting. Contract: [search.md](../features/search.md).
- [x] **Favorites / contacts UI** — Social panel Contacts + Favorites tabs; `VoiceFriendsClient` list/add/set favorite; star toggle on friends/contacts (`social_panel.dart`, `friends_client.dart`) — **Batch 23b**. **QR add friend UI** — my-code + paste profile link (`qr_add_friend_sheet.dart`) — **Batch 26a**. Phone-book sync is explicitly post-alpha/G3 mobile scope; A1/G1 clients hide it and the backend rejects direct calls (`phone_contact_sync_unavailable`).
- [ ] **QR add-friend — live camera scanner** — Batch 26a ships paste field; l10n implies camera scan. Add `mobile_scanner` (or similar) or narrow copy to paste-only.
- [x] **Blocked accounts UI** — Social panel Blocked tab; `VoiceFriendsClient` `listBlocked`/`unblockAccount`; gateway `GET/DELETE /api/v1/friends/blocks` — **Batch 24a**.
- [x] **Outgoing friend request declined label** — `PendingFriendRequest.status` from API; Requests tab shows «Declined» vs «Request pending» for outgoing rows — **Batch 24b**.
- [ ] **Windows: автоопределение игры** — process list → `game_title` + toggle; 0 кода. Post-MVP рядом с П.17.
- [ ] **E2E fingerprint + key-change banner** — [encryption.md](../features/encryption.md); П.13.
- [ ] **Shorebird OTA или явный defer** — [updates.md](../features/updates.md).
- [ ] **Voice: local MP3 recording; ducking via `setVolume`; in-voice overlay без кражи фокуса** — [voice-chat.md](../features/voice-chat.md).
- [ ] **[A2 Voice] Enable canonical room lifecycle after backend bindings land** — Flutter now retains the complete server-owned UUID pair `voice_room_id` / `space_id` during active-session decode and join response mapping. Missing, malformed or non-room bindings expose no new Space ID; legacy room fields remain compatible. The optional Space returned by canonical join must match its request path. This persisted locator does not grant current access. Separate `joinSpaceVoiceRoom` / `leaveSpaceVoiceRoom` client methods follow the frozen Space-scoped routes and operation ID, but are default-off and not used by the controller. Existing legacy entry and `hangUp`/failed-connect `EndCall` cleanup remain unchanged. Remaining: backend routes and operation ledger, explicit canonical session selection, room-leave cleanup with truthful failure/retry states, background/reconnect and real-media vertical proof. Never infer Space from room ID or enable the client seam from tests alone. Sources: [Voice Phase-0](../microservices/voice-service.md#phase-0-space-room-media-roster-и-lifecycle-target-не-реализовано); `src/frontend/lib/backend/voice_client.dart`; `src/frontend/lib/state/call_providers.dart`.
- [x] **[Voice] Reconnect restores active media projection** — every accepted Realtime `hello` reconciles `GetActiveCall`; no active session clears stale LiveKit/screen UI, active session replaces it, and REST failure retains state for retry. `GetVoiceStates` is authoritative for screen sharers while LiveKit tracks gate renderability.
- [ ] **Screen share: desktop source picker + system audio (Windows); явный simulcast 720/360/180** — [screen-share.md](../features/screen-share.md).
- [x] **Settings language sync** — сохранение `profile.locale`, приоритет
  текущего профиля и временный System описаны в [i18n.md](../features/i18n.md).
  `profile_locale_settings_test.dart` проверяет save/retry, profile/session
  switch, reload, изменение с другого устройства и OS fallback;
  `voice_app_i18n_test.dart` и `api_error_messages_test.dart` проверяют безопасные
  локализованные API errors. Live cross-device/backend acceptance отдельно.


**Промпт-якорь:** `Growth/A11y from docs/todo/client.md Common Batch 5`.



### Guest accounts UX



Baseline (2026-06) закрыт; хвосты UX/E2E ниже. Спека: [auth-and-contacts.md](../features/auth-and-contacts.md).

- [x] **Convert-guest: док auth-service.md** — `ConvertGuest` §: `password` = новый пароль regular-аккаунта (JWT гостя достаточен), не transport-пароль.
- [x] **Convert-guest live в compose-e2e** — `TestComposeConvertGuest_live` in `.github/ci/e2e-features.yml` smoke_gateway (CI tier 2).
- [x] **Convert-guest: negative Auth integration tests** — duplicate email, password <8, non-guest token, missing email/phone in `ConvertGuestIntegrationTest`.
- [x] **Convert-guest: NATS `user.guest_converted`** — `TestComposeConvertGuestNATS_live` shipped (P1.1).
- [ ] **Guest save-account reminder: current-version acceptance** — Auth хранит `guest_reminder_last_shown_at` и атомарно резервирует показ не чаще одного раза за любые 24 часа на guest account; `SharedPreferences` служит локальным fast path. Канон включает последующие входы с того же или другого устройства ([auth-and-contacts.md](../features/auth-and-contacts.md#гостевая-учётка)). Реализация и concurrency/cadence tests уже существуют; остаётся приёмка текущей версии по `CL-026`, а не добавление второго server timestamp endpoint.


**Промпт-якорь:** `Guest accounts UX from docs/todo/client.md Common Batch 6`.


### Multi-profile

- [x] **[Multi-Profile] Create flow missing avatar** — `CreateProfileSheet` optional avatar pick + presigned upload after profile switch (`parallel/client-qa-rail-profiles`).
- [ ] **[Profile Edit v3] Remaining §15 contracts and capability controls** — `UIV3-PANEL-PROFILE-EDIT` must remain open for username mutation/clean-name entitlement purchase, banner upload, additional phone binding and primary reassignment. The existing Flutter `UpdateProfile` mapper supports display name, bio, avatar, accent and locale; it has no username, banner or primary mutation input, and avatar presigning does not establish a banner-upload contract. Keep the handle read-only and do not infer phone/account type from a public profile. Deleting an active profile requires switching away; primary profiles are always blocked, while an inactive non-primary profile can be deleted directly. This follows the existing `ManageProfilesSheet` guards. Finish the documented supported flows and H/V visual/runtime proof when the corresponding API/capability contracts are available; HTML controls alone do not authorize fabricated endpoints or premium enablement. Primary API follow-up is the next item.
- [ ] **[Multi-Profile] Change primary profile API/UI missing** — `is_primary` set at bootstrap only; no way to reassign which profile phone search returns.
- [x] **[Multi-Profile] Accent color not choosable on create** — `CreateProfileSheet` accent swatches from token catalog; sends `accent_color` on create (**Batch 17**).
- [x] **[Multi-Profile] Server-authoritative accent picker** — settings picker displays the active profile's `profiles.accent_color` and reflects a new choice only after `UpdateProfile` succeeds. Legacy local override/index is used only when the server profile has no accent and is cleared only after successful migration/update; late responses cannot replace the newly active profile's selection (PR #623).
- [ ] **[Multi-Profile] Guest + multi-profile product rule undocumented** — no `CreateProfile` tier/guest guard; settings create action visible for guests; clarify in `multi-profile.md` or gate in UI/API.


## Low

### Stories (post-MVP, client)

- [ ] **Story editor v2** — stickers, doodle, filters, clip trim (§Редактор / §Клип).
- [ ] **Full per-story `PrivacyAudiencePicker`** — space multiselect on create.
- [ ] **Anonymous view (Premium)** — backend `MarkViewed.anonymous`; client UX отложен.

### Windows & desktop

- [ ] **Windows sign-off** — скилл `voice-project-full-verification`: `compose-config-ci`, `buf-ci`, `flutter-ci` — OK; `backend-test-ci-short` — после `c3598f3` fix [`jetstream_test.go`](../../src/backend/messaging/internal/messageevents/jetstream_test.go) (Flush + EnsureStream) **перепроверить на Windows/Docker**; compose smoke E2E не гонялся. См. [TESTING.md](../TESTING.md) § «Локальные грабли».
### Multi-profile

- [x] **[Multi-Profile] `profile_context_controller` handoff regressions** — `profile_context_controller_test.dart` covers active MM cancellation/recovery clearing and Space retain/exit on profile switch; T-055 `t055_profile_switch_reconnect_inbox_e2e_live_test.dart` verifies the selected profile's real Realtime hello before its inbox REST/history.



**Промпт-якорь:** `Client from docs/todo/client.md` + приоритет/подсекцию.

> Story backend-хвосты — [backend.md](backend.md) § Low § Story.
> Design/Penpot — [design.md](design.md).
