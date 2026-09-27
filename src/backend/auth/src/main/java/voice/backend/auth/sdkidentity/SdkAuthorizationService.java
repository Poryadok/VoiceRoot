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
  private final SecureRandom random = new SecureRandom();

  public record AuthorizationRequest(UUID requestId, long policyRevision, Instant expiresAt,
                                     String displayName, Set<String> scopes) {}
  public record Approval(String code, URI redirectUri, Instant expiresAt) {}
  public record ConsentView(UUID requestId, UUID applicationId, UUID environmentId, String displayName,
                            Set<String> scopes, String gameSubject, long policyRevision, Instant expiresAt) {}
  public record LinkedSession(UUID sourceAccountId, UUID accountId, UUID profileId, UUID deviceId,
                              UUID applicationId, UUID environmentId, Set<String> scopes,
                              long consentRevision, long policyRevision, String accessToken, Instant expiresAt) {}

  public SdkAuthorizationService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
      SdkIdentityService identity, AuthService auth, SdkAuthorizationPolicy policies,
      SdkProfileEligibility profiles, TokenBlacklist blacklist, Clock clock) {
    this.jdbc = jdbc;
    this.transactions = transactions;
    this.identity = identity;
    this.auth = auth;
    this.policies = policies;
    this.profiles = profiles;
    this.blacklist = blacklist;
    this.clock = clock;
  }

  public AuthorizationRequest start(String sdkToken, String deviceProof, UUID idempotencyKey,
      String redirect, String codeChallenge, String state, Set<String> scopes) {
    String requestHash = SdkAuthorizationProtocol.requestHash(idempotencyKey, redirect, codeChallenge, state, scopes);
    Set<String> requested = Set.copyOf(scopes);
    return transactions.execute(transaction -> {
      var source = identity.authorize(sdkToken, deviceProof, requestHash);
      var policy = policy(source.applicationId(), source.environmentId(), redirect, requested);
      var existing = jdbc.queryForList("""
          SELECT * FROM sdk_authorizations WHERE source_account_id=:source AND idempotency_key=:key FOR UPDATE
          """, Map.of("source", source.accountId(), "key", idempotencyKey));
      if (!existing.isEmpty()) {
        Row row = new Row(existing.getFirst());
        if (!requestHash.equals(row.text("request_hash"))) throw new SdkAuthorizationConflictException();
        if (policy.revision() != row.number("policy_revision")
            || !source.deviceId().equals(row.id("device_id"))) throw denied();
        fresh(row.time("expires_at"));
        return request(row);
      }
      fresh(source.expiresAt());
      UUID requestId = UUID.randomUUID();
      Instant expires = earlier(source.expiresAt(), clock.instant().plusSeconds(300));
      Long generation = jdbc.queryForObject("SELECT ownership_generation FROM sdk_identities WHERE account_id=:id",
          Map.of("id", source.accountId()), Long.class);
      var values = new MapSqlParameterSource().addValue("id", requestId).addValue("source", source.accountId())
          .addValue("device", source.deviceId()).addValue("session", SdkIdentityService.hash(sdkToken))
          .addValue("generation", generation).addValue("app", source.applicationId()).addValue("env", source.environmentId())
          .addValue("key", idempotencyKey).addValue("hash", requestHash).addValue("redirect", redirect)
          .addValue("challenge", codeChallenge).addValue("state", state).addValue("scopes", canonicalScopes(requested))
          .addValue("revision", policy.revision()).addValue("name", policy.displayName()).addValue("expires", Timestamp.from(expires));
      jdbc.update("""
          INSERT INTO sdk_authorizations(request_id,source_account_id,device_id,source_session_hash,source_generation,
            application_id,environment_id,idempotency_key,request_hash,redirect_uri,code_challenge,client_state,
            scopes,policy_revision,display_name,expires_at)
          VALUES (:id,:source,:device,:session,:generation,:app,:env,:key,:hash,:redirect,:challenge,:state,
            :scopes,:revision,:name,:expires)
          """, values);
      return new AuthorizationRequest(requestId, policy.revision(), expires, policy.displayName(), requested);
    });
  }

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
      if (expectedPolicyRevision != row.number("policy_revision") || row.text("code_hash") != null) throw denied();
      target(target, claims.sessionEpoch(), claims.jti());
      var profile = profile(target, selectedProfile);
      fresh(claims.expiresAt());
      fresh(row.time("expires_at"));
      String code = randomToken();
      Instant expires = earlier(earlier(clock.instant().plusSeconds(60), row.time("expires_at")), claims.expiresAt());
      jdbc.update("""
          UPDATE sdk_authorizations SET code_hash=:code,code_expires_at=:expires,target_account_id=:target,
            target_profile_id=:profile,target_epoch=:epoch,profile_revision=:revision,approval_jti=:jti,
            approval_expires_at=:approvalExpires WHERE request_id=:id AND code_hash IS NULL
          """, new MapSqlParameterSource().addValue("code", SdkIdentityService.hash(code))
              .addValue("expires", Timestamp.from(expires)).addValue("target", target).addValue("profile", selectedProfile)
              .addValue("epoch", claims.sessionEpoch()).addValue("revision", profile.revision()).addValue("jti", claims.jti())
              .addValue("approvalExpires", Timestamp.from(claims.expiresAt())).addValue("id", requestId));
      String redirect = row.text("redirect_uri");
      // Code and state are base64url; existing query bytes remain untouched.
      return new Approval(code, URI.create(redirect + (redirect.contains("?") ? "&" : "?")
          + "code=" + code + "&state=" + row.text("client_state")), expires);
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
        row.text("display_name"), row.scopes());
  }

  private LinkedSession linked(Row row, long revision, String token, Instant expires) {
    return new LinkedSession(row.id("source_account_id"), row.id("target_account_id"), row.id("target_profile_id"),
        row.id("device_id"), row.id("application_id"), row.id("environment_id"), row.scopes(), revision,
        row.number("policy_revision"), token, expires);
  }

  private void fresh(Instant expires) { if (expires == null || !expires.isAfter(clock.instant())) throw denied(); }
  private static void token(String value) { if (value == null || !value.matches("[A-Za-z0-9_-]{43}")) throw denied(); }
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
    long number(String key) { return ((Number) values.get(key)).longValue(); }
    Instant time(String key) { return values.get(key) == null ? null : ((Timestamp) values.get(key)).toInstant(); }
    Set<String> scopes() { return Set.of(text("scopes").split(",")); }
  }
}
