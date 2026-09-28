package voice.backend.auth.sdkidentity;

import java.net.URI;
import java.security.SecureRandom;
import java.sql.Timestamp;
import java.time.Clock;
import java.time.Instant;
import java.util.Base64;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.TokenClaims;

/** Durable browser consent. Linked bootstrap credentials do not activate game or media authority. */
public class SdkAuthorizationService {
  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final SdkIdentityService identity;
  private final AuthService auth;
  private final SdkAuthorizationPolicy policies;
  private final SdkProfileEligibility profiles;
  private final TokenBlacklist blacklist;
  private final Clock clock;
  private final SdkBindingChallengeAuthority bindingChallenges;
  private final AuthUserPrincipalIssuer principalIssuer;
  private final AuthGameBindingSubjectDigest subjectDigest;
  private final AuthGameBindingApprovalCodeVault approvalCodeVault;
  private final SecureRandom random = new SecureRandom();

  public record AuthorizationRequest(UUID requestId, long policyRevision, Instant expiresAt,
                                     String displayName, Set<String> scopes, UUID gameBindingChallengeId, String gameBindingNonce) {
    public AuthorizationRequest(UUID requestId, long policyRevision, Instant expiresAt, String displayName, Set<String> scopes) {
      this(requestId, policyRevision, expiresAt, displayName, scopes, null, null);
    }
  }
  public record Approval(String code, URI redirectUri, Instant expiresAt, UUID gameBindingChallengeId, String gameBindingNonce) {
    public Approval(String code, URI redirectUri, Instant expiresAt) { this(code, redirectUri, expiresAt, null, null); }
  }
  public record ConsentView(UUID requestId, UUID applicationId, UUID environmentId, String displayName,
                            Set<String> scopes, String gameSubject, long policyRevision, Instant expiresAt) {}
  public record LinkedSession(UUID sourceAccountId, UUID accountId, UUID profileId, UUID deviceId,
                              UUID applicationId, UUID environmentId, Set<String> scopes,
                              long consentRevision, long policyRevision, String accessToken, Instant expiresAt) {}

  public SdkAuthorizationService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
      SdkIdentityService identity, AuthService auth, SdkAuthorizationPolicy policies,
      SdkProfileEligibility profiles, TokenBlacklist blacklist, Clock clock) {
    this(jdbc, transactions, identity, auth, policies, profiles, blacklist, clock, null);
  }

  public SdkAuthorizationService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
      SdkIdentityService identity, AuthService auth, SdkAuthorizationPolicy policies,
      SdkProfileEligibility profiles, TokenBlacklist blacklist, Clock clock,
      SdkBindingChallengeAuthority bindingChallenges) {
    this(jdbc, transactions, identity, auth, policies, profiles, blacklist, clock, bindingChallenges, null, null);
  }

  public SdkAuthorizationService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
      SdkIdentityService identity, AuthService auth, SdkAuthorizationPolicy policies,
      SdkProfileEligibility profiles, TokenBlacklist blacklist, Clock clock,
      SdkBindingChallengeAuthority bindingChallenges, AuthUserPrincipalIssuer principalIssuer,
      AuthGameBindingSubjectDigest subjectDigest) {
    this(jdbc, transactions, identity, auth, policies, profiles, blacklist, clock, bindingChallenges,
        principalIssuer, subjectDigest, null);
  }

  public SdkAuthorizationService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
      SdkIdentityService identity, AuthService auth, SdkAuthorizationPolicy policies,
      SdkProfileEligibility profiles, TokenBlacklist blacklist, Clock clock,
      SdkBindingChallengeAuthority bindingChallenges, AuthUserPrincipalIssuer principalIssuer,
      AuthGameBindingSubjectDigest subjectDigest, AuthGameBindingApprovalCodeVault approvalCodeVault) {
    this.jdbc = jdbc;
    this.transactions = transactions;
    this.identity = identity;
    this.auth = auth;
    this.policies = policies;
    this.profiles = profiles;
    this.blacklist = blacklist;
    this.clock = clock;
    this.bindingChallenges = bindingChallenges;
    this.principalIssuer = principalIssuer;
    this.subjectDigest = subjectDigest;
    this.approvalCodeVault = approvalCodeVault;
  }

  public AuthorizationRequest start(String sdkToken, String deviceProof, UUID idempotencyKey,
      String redirect, String codeChallenge, String state, Set<String> scopes) {
    return start(sdkToken, deviceProof, idempotencyKey, redirect, codeChallenge, state, scopes, null);
  }

  public AuthorizationRequest start(String sdkToken, String deviceProof, UUID idempotencyKey,
      String redirect, String codeChallenge, String state, Set<String> scopes, UUID bindingChallengeId) {
    return start(sdkToken, deviceProof, idempotencyKey, redirect, codeChallenge, state, scopes,
        bindingChallengeId, bindingChallengeId != null);
  }

  public AuthorizationRequest start(String sdkToken, String deviceProof, UUID idempotencyKey,
      String redirect, String codeChallenge, String state, Set<String> scopes, boolean gameBindingIntent) {
    return start(sdkToken, deviceProof, idempotencyKey, redirect, codeChallenge, state, scopes, null, gameBindingIntent);
  }

  private AuthorizationRequest start(String sdkToken, String deviceProof, UUID idempotencyKey,
      String redirect, String codeChallenge, String state, Set<String> scopes, UUID bindingChallengeId,
      boolean gameBindingIntent) {
    SdkBindingChallengeAuthority.Challenge bindingChallenge = bindingChallengeId == null ? null :
        bindingChallenges == null ? deniedChallenge() : bindingChallenges.resolveBindingChallenge(bindingChallengeId);
    String requestHash = SdkAuthorizationProtocol.requestHash(idempotencyKey, redirect, codeChallenge, state, scopes,
        bindingChallengeId != null ? bindingChallengeId : (gameBindingIntent ? idempotencyKey : null));
    Set<String> requested = Set.copyOf(scopes);
    return transactions.execute(transaction -> {
      var source = identity.authorize(sdkToken, deviceProof, requestHash);
      var policy = policy(source.applicationId(), source.environmentId(), redirect, requested);
      if (bindingChallenge != null) checkBindingChallenge(bindingChallengeId, bindingChallenge, source.applicationId(),
          source.environmentId(), source.deviceId(), redirect, codeChallenge);
      var existing = jdbc.queryForList("""
          SELECT * FROM sdk_authorizations WHERE source_account_id=:source AND idempotency_key=:key FOR UPDATE
          """, Map.of("source", source.accountId(), "key", idempotencyKey));
      if (!existing.isEmpty()) {
        Row row = new Row(existing.getFirst());
        if (!requestHash.equals(row.text("request_hash"))) throw new SdkAuthorizationConflictException();
        if (policy.revision() != row.number("policy_revision")
            || !source.deviceId().equals(row.id("device_id"))
            || !java.util.Objects.equals(bindingChallengeId, row.id("game_binding_challenge_id"))
            || gameBindingIntent != Boolean.TRUE.equals(row.values().get("game_binding_intent"))) throw denied();
        fresh(row.time("expires_at"));
        return request(row);
      }
      fresh(source.expiresAt());
      UUID requestId = UUID.randomUUID();
      Instant expires = earlier(source.expiresAt(), clock.instant().plusSeconds(300));
      Long bindingConsentRevision = gameBindingIntent ? jdbc.queryForObject(
          "SELECT nextval(pg_get_serial_sequence('sdk_linked_sessions','consent_revision'))", Map.of(), Long.class) : null;
      Long generation = jdbc.queryForObject("SELECT ownership_generation FROM sdk_identities WHERE account_id=:id",
          Map.of("id", source.accountId()), Long.class);
      var values = new MapSqlParameterSource().addValue("id", requestId).addValue("source", source.accountId())
          .addValue("device", source.deviceId()).addValue("session", SdkIdentityService.hash(sdkToken))
          .addValue("generation", generation).addValue("app", source.applicationId()).addValue("env", source.environmentId())
          .addValue("key", idempotencyKey).addValue("hash", requestHash).addValue("redirect", redirect)
          .addValue("challenge", codeChallenge).addValue("state", state).addValue("scopes", canonicalScopes(requested))
          .addValue("revision", policy.revision()).addValue("name", policy.displayName()).addValue("expires", Timestamp.from(expires));
      values.addValue("bindingChallenge", bindingChallenge == null ? null : bindingChallenge.challengeId())
          .addValue("bindingNonce", bindingChallenge == null ? null : bindingChallenge.nonce())
          .addValue("bindingOperation", bindingChallenge != null ? bindingChallenge.operationId() : (gameBindingIntent ? idempotencyKey : null))
          .addValue("bindingConsent", bindingConsentRevision)
          .addValue("bindingIntent", gameBindingIntent);
      jdbc.update("""
          INSERT INTO sdk_authorizations(request_id,source_account_id,device_id,source_session_hash,source_generation,
            application_id,environment_id,idempotency_key,request_hash,redirect_uri,code_challenge,client_state,
            scopes,policy_revision,display_name,expires_at,game_binding_challenge_id,game_binding_operation_id,
            game_binding_intent,game_binding_consent_revision,game_binding_challenge_nonce)
          VALUES (:id,:source,:device,:session,:generation,:app,:env,:key,:hash,:redirect,:challenge,:state,
            :scopes,:revision,:name,:expires,:bindingChallenge,:bindingOperation,:bindingIntent,:bindingConsent,:bindingNonce)
          """, values);
      return new AuthorizationRequest(requestId, policy.revision(), expires, policy.displayName(), requested, null, null);
    });
  }

  private void checkBindingChallenge(UUID expectedChallengeId, SdkBindingChallengeAuthority.Challenge challenge, UUID app, UUID env,
      UUID deviceId, String redirect, String codeChallenge) {
    if (challenge == null || !expectedChallengeId.equals(challenge.challengeId())
        || !app.equals(challenge.applicationId()) || !env.equals(challenge.environmentId())
        || !deviceId.equals(challenge.deviceKeyId()) || !"google".equals(challenge.provider())
        || !challenge.pkceChallenge().equals(codeChallenge)
        || !challenge.redirectUriSha256().equals(SdkIdentityService.hash(redirect))
        || challenge.operationId() == null || !"pending".equals(challenge.status())
        || challenge.expiresAt() == null || !challenge.expiresAt().isAfter(clock.instant())) throw denied();
    var thumbprints = jdbc.queryForList("SELECT thumbprint FROM sdk_devices WHERE device_id=:id AND revoked_at IS NULL",
        Map.of("id", deviceId));
    if (thumbprints.isEmpty() || !challenge.deviceKeyThumbprint().equals(thumbprints.getFirst().get("thumbprint"))) throw denied();
  }

  private static SdkBindingChallengeAuthority.Challenge deniedChallenge() { throw denied(); }

  public Approval approve(UUID requestId, String voiceBearer, UUID selectedProfile, long expectedPolicyRevision) {
    if (selectedProfile == null) throw denied();
    TokenClaims claims = voice(voiceBearer);
    UUID target;
    try { target = UUID.fromString(claims.userId()); }
    catch (RuntimeException invalid) { throw denied(); }
    return transactions.execute(transaction -> {
      Row row = locked(requestId);
      source(row, true);
      policy(row);
      if (expectedPolicyRevision != row.number("policy_revision")) throw denied();
      target(target, claims.sessionEpoch(), claims.jti());
      fresh(claims.expiresAt());
      fresh(row.time("expires_at"));
      if (row.text("code_hash") != null) {
        if (!Boolean.TRUE.equals(row.values().get("game_binding_intent")) || approvalCodeVault == null
            || !target.equals(row.id("target_account_id")) || !selectedProfile.equals(row.id("target_profile_id"))
            || claims.sessionEpoch() != row.number("target_epoch") || !claims.jti().equals(row.text("approval_jti"))
            || row.time("consumed_at") != null
            || row.id("game_binding_challenge_id") == null
            || row.bytes("game_binding_approval_code_nonce") == null
            || row.bytes("game_binding_approval_code_ciphertext") == null) throw new SdkAuthorizationConflictException();
        var currentProfile = profile(target, selectedProfile);
        if (currentProfile.revision() != row.number("profile_revision")) throw denied();
        fresh(row.time("code_expires_at"));
        approved(row);
        String savedCode = approvalCodeVault.open(requestId, row.bytes("game_binding_approval_code_nonce"),
            row.bytes("game_binding_approval_code_ciphertext"));
        if (!SdkIdentityService.hash(savedCode).equals(row.text("code_hash"))) throw denied();
        String savedRedirect = row.text("redirect_uri");
        return new Approval(savedCode, URI.create(savedRedirect + (savedRedirect.contains("?") ? "&" : "?")
            + "code=" + savedCode + "&state=" + row.text("client_state")), row.time("code_expires_at"),
            row.id("game_binding_challenge_id"), row.text("game_binding_challenge_nonce"));
      }
      var profile = profile(target, selectedProfile);
      String code = randomToken();
      Instant expires = earlier(earlier(clock.instant().plusSeconds(60), row.time("expires_at")), claims.expiresAt());
      UUID bindingChallengeId = null;
      String bindingNonce = null;
      if (Boolean.TRUE.equals(row.values().get("game_binding_intent")) && row.id("game_binding_challenge_id") == null) {
        if (bindingChallenges == null || row.id("game_binding_operation_id") == null
            || row.number("game_binding_consent_revision") <= 0) throw denied();
        var sourceIdentity = jdbc.queryForList("""
            SELECT actor_id FROM sdk_identities WHERE account_id=:id AND ownership_generation=:generation AND status='active'
            """, Map.of("id", row.id("source_account_id"), "generation", row.number("source_generation")));
        var deviceValues = jdbc.queryForList("SELECT thumbprint FROM sdk_devices WHERE device_id=:id AND account_id=:account AND revoked_at IS NULL",
            Map.of("id", row.id("device_id"), "account", row.id("source_account_id")));
        if (sourceIdentity.isEmpty() || deviceValues.isEmpty()) throw denied();
        String thumbprint = (String) deviceValues.getFirst().get("thumbprint");
        var create = new SdkBindingChallengeAuthority.CreateRequest(row.id("application_id"), row.id("environment_id"),
            "google", SdkIdentityService.hash(row.text("redirect_uri")), row.text("code_challenge"), row.id("device_id"),
            thumbprint, row.id("game_binding_operation_id"), row.time("expires_at"), row.id("source_account_id"),
            (UUID) sourceIdentity.getFirst().get("actor_id"), row.id("device_id"), row.number("source_generation"),
            target, selectedProfile, profile.revision(), row.number("game_binding_consent_revision"),
            row.number("policy_revision"), row.scopes().stream().sorted().toList());
        SdkBindingChallengeAuthority.Challenge challenge = bindingChallenges.createBindingChallenge(create);
        if (challenge == null || challenge.challengeId() == null || challenge.nonce() == null
            || !challenge.operationId().equals(create.operationId()) || !challenge.applicationId().equals(create.applicationId())
            || !challenge.environmentId().equals(create.environmentId()) || !challenge.sourceAccountId().equals(create.sourceAccountId())
            || !challenge.sourceActorId().equals(create.sourceActorId()) || !challenge.sourceDeviceId().equals(create.sourceDeviceId())
            || challenge.sourceGeneration() != create.sourceGeneration() || !challenge.targetAccountId().equals(create.targetAccountId())
            || !challenge.targetProfileId().equals(create.targetProfileId()) || challenge.profileRevision() != create.profileRevision()
            || challenge.consentRevision() != create.consentRevision() || challenge.policyRevision() != create.policyRevision()
            || !challenge.scopes().equals(create.scopes())) throw denied();
        bindingChallengeId = challenge.challengeId();
        bindingNonce = challenge.nonce();
        if (jdbc.update("""
            UPDATE sdk_authorizations SET game_binding_challenge_id=:challenge,game_binding_challenge_nonce=:nonce
            WHERE request_id=:id AND game_binding_operation_id=:operation AND game_binding_challenge_id IS NULL
            """, Map.of("challenge", bindingChallengeId, "nonce", bindingNonce, "id", requestId,
                "operation", create.operationId())) != 1) throw denied();
      }
      AuthGameBindingApprovalCodeVault.Sealed sealedCode = Boolean.TRUE.equals(row.values().get("game_binding_intent"))
          ? requireApprovalCodeVault().seal(requestId, code) : null;
      jdbc.update("""
          UPDATE sdk_authorizations SET code_hash=:code,code_expires_at=:expires,target_account_id=:target,
            target_profile_id=:profile,target_epoch=:epoch,profile_revision=:revision,approval_jti=:jti,
            approval_expires_at=:approvalExpires,game_binding_approval_code_nonce=:bindingCodeNonce,
            game_binding_approval_code_ciphertext=:bindingCodeCiphertext WHERE request_id=:id AND code_hash IS NULL
          """, new MapSqlParameterSource().addValue("code", SdkIdentityService.hash(code))
              .addValue("expires", Timestamp.from(expires)).addValue("target", target).addValue("profile", selectedProfile)
              .addValue("epoch", claims.sessionEpoch()).addValue("revision", profile.revision()).addValue("jti", claims.jti())
              .addValue("approvalExpires", Timestamp.from(claims.expiresAt()))
              .addValue("bindingCodeNonce", sealedCode == null ? null : sealedCode.nonce())
              .addValue("bindingCodeCiphertext", sealedCode == null ? null : sealedCode.ciphertext())
              .addValue("id", requestId));
      String redirect = row.text("redirect_uri");
      // Code and state are base64url; existing query bytes remain untouched.
      return new Approval(code, URI.create(redirect + (redirect.contains("?") ? "&" : "?")
          + "code=" + code + "&state=" + row.text("client_state")), expires, bindingChallengeId, bindingNonce);
    });
  }

  public ConsentView inspect(UUID requestId, String voiceBearer) {
    TokenClaims claims = voice(voiceBearer);
    UUID account;
    try { account = UUID.fromString(claims.userId()); }
    catch (RuntimeException invalid) { throw denied(); }
    return transactions.execute(transaction -> {
      Row row = locked(requestId);
      source(row, true);
      policy(row);
      target(account, claims.sessionEpoch(), claims.jti());
      fresh(claims.expiresAt());
      fresh(row.time("expires_at"));
      String game = jdbc.queryForObject("SELECT game_subject FROM sdk_sessions WHERE token_hash=:hash",
          Map.of("hash", row.text("source_session_hash")), String.class);
      return new ConsentView(requestId, row.id("application_id"), row.id("environment_id"), row.text("display_name"),
          row.scopes(), game, row.number("policy_revision"), row.time("expires_at"));
    });
  }

  private TokenClaims voice(String bearer) {
    try {
      TokenClaims claims = auth.validate(bearer);
      if (claims == null || claims.userId() == null || claims.jti() == null || claims.jti().isBlank()
          || !"regular".equals(claims.normalizedAccountType())) throw denied();
      return claims;
    } catch (RuntimeException invalid) { throw denied(); }
  }

  public LinkedSession exchange(UUID requestId, String code, String redirect, String verifier, String deviceProof) {
    token(code);
    if (verifier == null || verifier.length() > 128) throw denied();
    return transactions.execute(transaction -> {
      Row row = locked(requestId);
      if (row.id("game_binding_challenge_id") != null) throw denied();
      String key = source(row, true);
      if (row.time("consumed_at") != null || !SdkIdentityService.hash(code).equals(row.text("code_hash"))
          || !row.text("redirect_uri").equals(redirect)
          || !SdkAuthorizationProtocol.verifyPkce(verifier, row.text("code_challenge"))) throw denied();
      SdkIdentityService.possession(key, deviceProof, "voice-sdk-code-v1\n" + requestId + "\n"
          + SdkIdentityService.hash(code) + "\n" + SdkIdentityService.hash(verifier));
      approved(row);
      fresh(row.time("expires_at"));
      fresh(row.time("code_expires_at"));
      Instant now = clock.instant();
      if (jdbc.update("""
          UPDATE sdk_authorizations SET consumed_at=:now WHERE request_id=:id AND consumed_at IS NULL
            AND expires_at>:now AND code_expires_at>:now
          """, Map.of("now", Timestamp.from(now), "id", requestId)) != 1) throw denied();
      String linkedToken = randomToken();
      Instant expires = earlier(now.plusSeconds(300), row.time("approval_expires_at"));
      Long revision = jdbc.queryForObject("""
          INSERT INTO sdk_linked_sessions(token_hash,request_id,expires_at) VALUES (:hash,:id,:expires)
          RETURNING consent_revision
          """, Map.of("hash", SdkIdentityService.hash(linkedToken), "id", requestId,
              "expires", Timestamp.from(expires)), Long.class);
      return linked(row, revision, linkedToken, expires);
    });
  }

  /** Consume a linked-profile code only for the private GIS binding exchange. */
  public String exchangeGameBinding(UUID challengeId, UUID operationId, String code, String verifier, String deviceProof) {
    token(code);
    if (challengeId == null || operationId == null || verifier == null || verifier.length() > 128
        || bindingChallenges == null || principalIssuer == null || subjectDigest == null) throw denied();
    SdkBindingChallengeAuthority.Challenge challenge = bindingChallenges.resolveBindingChallenge(challengeId);
    return transactions.execute(transaction -> {
      var requests = jdbc.queryForList("""
          SELECT request_id FROM sdk_authorizations WHERE game_binding_challenge_id=:challenge
            AND game_binding_operation_id=:operation
          """, Map.of("challenge", challengeId, "operation", operationId));
      if (requests.isEmpty()) throw denied();
      UUID requestId = (UUID) requests.getFirst().get("request_id");
      Row row = locked(requestId);
      String deviceKey = source(row, true);
      if (!operationId.equals(challenge.operationId()) || !challengeId.equals(row.id("game_binding_challenge_id"))
          || !challenge.nonce().equals(row.text("game_binding_challenge_nonce"))
          || !operationId.equals(row.id("game_binding_operation_id"))
          || !SdkIdentityService.hash(code).equals(row.text("code_hash"))
          || !SdkAuthorizationProtocol.verifyPkce(verifier, row.text("code_challenge"))
          || !SdkIdentityService.hash(row.text("redirect_uri")).equals(challenge.redirectUriSha256())) throw denied();
      checkBindingChallenge(challengeId, challenge, row.id("application_id"), row.id("environment_id"),
          row.id("device_id"), row.text("redirect_uri"), row.text("code_challenge"));
      checkBindingChallengeConsent(challenge, row);
      SdkIdentityService.possession(deviceKey, deviceProof, "voice-sdk-code-v1\n" + requestId + "\n"
          + SdkIdentityService.hash(code) + "\n" + SdkIdentityService.hash(verifier));
      var saved = jdbc.queryForList("""
          SELECT code_sha256,verifier_sha256,request_sha256,device_id,device_proof_sha256,handoff_jws
          FROM sdk_game_binding_handoff_issuances WHERE authorization_request_id=:request FOR UPDATE
          """, Map.of("request", requestId));
      if (!saved.isEmpty()) {
        Map<String, Object> prior = saved.getFirst();
        if (!SdkIdentityService.hash(code).equals(prior.get("code_sha256"))
            || !SdkIdentityService.hash(verifier).equals(prior.get("verifier_sha256"))
            || !bindingExchangeHash(challengeId, operationId, code, verifier).equals(prior.get("request_sha256"))
            || !SdkIdentityService.hash(deviceProof).equals(prior.get("device_proof_sha256"))
            || !row.id("device_id").equals(prior.get("device_id"))) throw new SdkAuthorizationConflictException();
        approved(row);
        return (String) prior.get("handoff_jws");
      }
      if (row.time("consumed_at") != null) throw denied();
      approved(row);
      fresh(row.time("expires_at"));
      fresh(row.time("code_expires_at"));
      Instant now = clock.instant();
      Instant expires = earlier(earlier(now.plusSeconds(30), challenge.expiresAt()), row.time("code_expires_at"));
      if (!expires.isAfter(now)) throw denied();
      if (jdbc.update("""
          UPDATE sdk_authorizations SET consumed_at=:now WHERE request_id=:id AND consumed_at IS NULL
            AND expires_at>:now AND code_expires_at>:now
          """, Map.of("now", Timestamp.from(now), "id", requestId)) != 1) throw denied();
      var subjectRows = jdbc.queryForList("SELECT issuer,provider_subject FROM sdk_identities WHERE account_id=:id",
          Map.of("id", row.id("source_account_id")));
      if (subjectRows.isEmpty() || !"google".equals(challenge.provider())) throw denied();
      String issuer = (String) subjectRows.getFirst().get("issuer");
      String sourceSubject = (String) subjectRows.getFirst().get("provider_subject");
      if (!Set.of("https://accounts.google.com", "accounts.google.com").contains(issuer)) throw denied();
      String digest = subjectDigest.digest(challenge.provider(), issuer,
          row.id("application_id"), row.id("environment_id"), sourceSubject);
      UUID actorId = jdbc.queryForObject("SELECT actor_id FROM sdk_identities WHERE account_id=:id",
          Map.of("id", row.id("source_account_id")), UUID.class);
      Long consentRevision = jdbc.queryForObject("""
          INSERT INTO sdk_linked_sessions(token_hash,request_id,expires_at)
          VALUES (:hash,:id,:expires) RETURNING consent_revision
          """, Map.of("hash", SdkIdentityService.hash(randomToken()), "id", requestId,
              "expires", Timestamp.from(expires)), Long.class);
      String handoff = principalIssuer.issueGameBindingHandoff(new AuthUserPrincipalIssuer.GameBindingHandoff(
          requestId, operationId, challengeId, challenge.nonce(), row.id("application_id"), row.id("environment_id"),
          SdkIdentityService.hash(row.text("redirect_uri")), row.text("code_challenge"), row.id("device_id"),
          challenge.deviceKeyThumbprint(), challenge.provider(), digest, row.id("source_account_id"),
          actorId, row.id("device_id"), row.number("source_generation"),
          row.id("target_account_id"), row.id("target_profile_id"), row.number("profile_revision"), consentRevision,
          row.number("policy_revision"), row.scopes().stream().sorted().toList(), expires));
      jdbc.update("""
          INSERT INTO sdk_game_binding_handoff_issuances(authorization_request_id,challenge_id,operation_id,
            code_sha256,verifier_sha256,request_sha256,device_id,device_proof_sha256,assertion_jti,handoff_jws,consent_revision,expires_at)
          VALUES (:request,:challenge,:operation,:code,:verifier,:requestHash,:device,:deviceProofHash,:jti,:jws,:consent,:expires)
          """, new MapSqlParameterSource().addValue("request", requestId).addValue("challenge", challengeId)
              .addValue("operation", operationId).addValue("code", SdkIdentityService.hash(code))
              .addValue("verifier", SdkIdentityService.hash(verifier)).addValue("requestHash", bindingExchangeHash(challengeId,
                  operationId, code, verifier))
              .addValue("device", row.id("device_id")).addValue("deviceProofHash", SdkIdentityService.hash(deviceProof))
              .addValue("jti", principalIssuer.verifyGameBindingHandoff(handoff).assertionJti())
              .addValue("jws", handoff).addValue("consent", consentRevision).addValue("expires", Timestamp.from(expires)));
      return handoff;
    });
  }

  private static String bindingExchangeHash(UUID challengeId, UUID operationId, String code, String verifier) {
    return SdkIdentityService.hash("voice-game-binding-exchange-v1\n" + challengeId + "\n" + operationId
        + "\n" + SdkIdentityService.hash(code) + "\n" + SdkIdentityService.hash(verifier));
  }

  private void checkBindingChallengeConsent(SdkBindingChallengeAuthority.Challenge challenge, Row row) {
    if (challenge.sourceAccountId() == null) return; // Only legacy in-process fixtures lack the Auth-bound tuple.
    UUID actor = jdbc.queryForObject("SELECT actor_id FROM sdk_identities WHERE account_id=:id",
        Map.of("id", row.id("source_account_id")), UUID.class);
    if (!row.id("source_account_id").equals(challenge.sourceAccountId()) || !actor.equals(challenge.sourceActorId())
        || !row.id("device_id").equals(challenge.sourceDeviceId()) || row.number("source_generation") != challenge.sourceGeneration()
        || !row.id("target_account_id").equals(challenge.targetAccountId())
        || !row.id("target_profile_id").equals(challenge.targetProfileId())
        || row.number("profile_revision") != challenge.profileRevision()
        || row.number("game_binding_consent_revision") != challenge.consentRevision()
        || row.number("policy_revision") != challenge.policyRevision()
        || !row.scopes().stream().sorted().toList().equals(challenge.scopes())) throw denied();
  }

  public LinkedSession linkedSession(String token, String proof) {
    token(token);
    return linkedSession(token, proof, "voice-sdk-linked-v1\n" + SdkIdentityService.hash(token));
  }

  LinkedSession prepareConversion(String token, String proof, UUID key, UUID binding) {
    return linkedSession(token, proof, SdkConversionProofs.preparePayload("existing", token, key, binding));
  }

  private LinkedSession linkedSession(String token, String proof, String payload) {
    return transactions.execute(transaction -> {
      var sessions = jdbc.queryForList("SELECT * FROM sdk_linked_sessions WHERE token_hash=:hash",
          Map.of("hash", SdkIdentityService.hash(token)));
      if (sessions.isEmpty()) throw denied();
      Row session = new Row(sessions.getFirst());
      Row row = locked(session.id("request_id"));
      String key = source(row, false);
      SdkIdentityService.possession(key, proof, payload);
      approved(row);
      fresh(session.time("expires_at"));
      return linked(row, session.number("consent_revision"), null, session.time("expires_at"));
    });
  }

  private Row locked(UUID requestId) {
    if (requestId == null) throw denied();
    var source = jdbc.queryForList("SELECT source_account_id FROM sdk_authorizations WHERE request_id=:id", Map.of("id", requestId));
    if (source.isEmpty()) throw denied();
    // Every operation locks SDK identity first, matching enrollment and revoke.
    jdbc.query("SELECT account_id FROM sdk_identities WHERE account_id=:id FOR UPDATE",
        Map.of("id", source.getFirst().get("source_account_id")), rs -> { });
    var rows = jdbc.queryForList("SELECT * FROM sdk_authorizations WHERE request_id=:id FOR UPDATE", Map.of("id", requestId));
    if (rows.isEmpty()) throw denied();
    return new Row(rows.getFirst());
  }

  private String source(Row row, boolean requireSession) {
    identity.requireAdmitted(row.id("application_id"), row.id("environment_id"));
    var values = new MapSqlParameterSource().addValue("source", row.id("source_account_id"))
        .addValue("device", row.id("device_id")).addValue("generation", row.number("source_generation"))
        .addValue("app", row.id("application_id")).addValue("env", row.id("environment_id"));
    var devices = jdbc.queryForList("""
        SELECT d.public_jwk FROM sdk_identities i JOIN sdk_devices d ON d.account_id=i.account_id
        WHERE i.account_id=:source AND i.status='active' AND i.ownership_generation=:generation
          AND i.application_id=:app AND i.environment_id=:env AND d.device_id=:device AND d.revoked_at IS NULL
        FOR UPDATE OF d
        """, values);
    if (devices.isEmpty()) throw denied();
    if (requireSession) {
      var sessions = jdbc.queryForList("""
          SELECT expires_at FROM sdk_sessions WHERE token_hash=:hash AND account_id=:source
            AND device_id=:device AND ownership_generation=:generation FOR UPDATE
          """, values.addValue("hash", row.text("source_session_hash")));
      if (sessions.isEmpty()) throw denied();
      fresh(new Row(sessions.getFirst()).time("expires_at"));
    }
    return (String) devices.getFirst().get("public_jwk");
  }

  private void approved(Row row) {
    if (row.id("target_account_id") == null) throw denied();
    policy(row);
    target(row.id("target_account_id"), row.number("target_epoch"), row.text("approval_jti"));
    var profile = profile(row.id("target_account_id"), row.id("target_profile_id"));
    if (profile.revision() != row.number("profile_revision")) throw denied();
    fresh(row.time("approval_expires_at"));
  }

  private void target(UUID account, long epoch, String jti) {
    var accounts = jdbc.queryForList("SELECT type,status,session_epoch,regular_email_verification_pending FROM accounts WHERE id=:id FOR UPDATE", Map.of("id", account));
    if (accounts.isEmpty()) throw denied();
    Row row = new Row(accounts.getFirst());
    if (!"regular".equals(row.text("type")) || !"active".equals(row.text("status"))
        || Boolean.TRUE.equals(row.values().get("regular_email_verification_pending"))
        || row.number("session_epoch") != epoch || epoch <= 0 || jti == null || jti.isBlank()) throw denied();
    try {
      var prepared = auth.prepareOAuthAccessToken(account.toString());
      if (prepared == null || !account.equals(prepared.accountId()) || prepared.sessionEpoch() != epoch
          || blacklist.isRevoked(jti)) throw denied();
    } catch (RuntimeException unavailable) { throw denied(); }
  }

  private SdkProfileEligibility.Profile profile(UUID account, UUID selected) {
    try {
      var profile = profiles.inspect(account, selected);
      if (profile == null || !account.equals(profile.accountId()) || !selected.equals(profile.profileId())
          || profile.revision() <= 0 || profile.deleted() || profile.frozen()) throw denied();
      return profile;
    } catch (RuntimeException unavailable) { throw denied(); }
  }

  private SdkAuthorizationPolicy.Policy policy(UUID app, UUID env, String redirect, Set<String> scopes) {
    try {
      var policy = policies.resolve(app, env);
      SdkAuthorizationProtocol.requirePolicy(policy, app, env, redirect, scopes);
      return policy;
    } catch (RuntimeException unavailable) { throw denied(); }
  }

  private void policy(Row row) {
    if (policy(row.id("application_id"), row.id("environment_id"), row.text("redirect_uri"), row.scopes())
        .revision() != row.number("policy_revision")) throw denied();
  }

  private AuthorizationRequest request(Row row) {
    return new AuthorizationRequest(row.id("request_id"), row.number("policy_revision"), row.time("expires_at"),
        row.text("display_name"), row.scopes(), row.id("game_binding_challenge_id"), row.text("game_binding_challenge_nonce"));
  }

  private LinkedSession linked(Row row, long revision, String token, Instant expires) {
    return new LinkedSession(row.id("source_account_id"), row.id("target_account_id"), row.id("target_profile_id"),
        row.id("device_id"), row.id("application_id"), row.id("environment_id"), row.scopes(), revision,
        row.number("policy_revision"), token, expires);
  }

  private void fresh(Instant expires) { if (expires == null || !expires.isAfter(clock.instant())) throw denied(); }
  private static void token(String value) { if (value == null || !value.matches("[A-Za-z0-9_-]{43}")) throw denied(); }
  private AuthGameBindingApprovalCodeVault requireApprovalCodeVault() {
    if (approvalCodeVault == null) throw denied();
    return approvalCodeVault;
  }
  private static Instant earlier(Instant first, Instant second) { return first.isBefore(second) ? first : second; }
  private static String canonicalScopes(Set<String> scopes) { return String.join(",", scopes.stream().sorted().toList()); }
  private static SdkIdentityDeniedException denied() { return new SdkIdentityDeniedException(); }
  private String randomToken() {
    byte[] bytes = new byte[32];
    random.nextBytes(bytes);
    return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
  }

  private record Row(Map<String, Object> values) {
    UUID id(String key) { return (UUID) values.get(key); }
    String text(String key) { return (String) values.get(key); }
    byte[] bytes(String key) { return (byte[]) values.get(key); }
    long number(String key) { return ((Number) values.get(key)).longValue(); }
    Instant time(String key) { return values.get(key) == null ? null : ((Timestamp) values.get(key)).toInstant(); }
    Set<String> scopes() { return Set.of(text("scopes").split(",")); }
  }
}
