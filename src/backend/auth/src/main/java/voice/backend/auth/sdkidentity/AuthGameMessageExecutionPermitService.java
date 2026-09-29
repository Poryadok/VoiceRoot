package voice.backend.auth.sdkidentity;

import java.sql.Timestamp;
import java.time.Clock;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.support.TransactionTemplate;

/** Auth-owned aggregation of current Auth authority and GIS's one-use message permit. */
public final class AuthGameMessageExecutionPermitService {
  public record Permit(String permitJws) {}
  public record Completion(UUID permitId, UUID operationId, String outcome, String status) {}

  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final AuthUserPrincipalIssuer issuer;
  private final SdkAuthorizationPolicy policies;
  private final SdkProfileEligibility profiles;
  private final SdkGameIntegrationExecutionPermitAuthority gis;
  private final Clock clock;

  public AuthGameMessageExecutionPermitService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
      AuthUserPrincipalIssuer issuer, SdkAuthorizationPolicy policies, SdkProfileEligibility profiles,
      SdkGameIntegrationExecutionPermitAuthority gis, Clock clock) {
    this.jdbc = jdbc;
    this.transactions = transactions;
    this.issuer = issuer;
    this.policies = policies;
    this.profiles = profiles;
    this.gis = gis;
    this.clock = clock;
  }

  public Permit issue(String assertion, UUID operationId, String requestSha256) {
    if (operationId == null || requestSha256 == null || !requestSha256.matches("[0-9a-f]{64}")) throw denied();
    AuthUserPrincipalIssuer.VerifiedDeviceStatus device = issuer.verifyDeviceStatusAssertion(assertion);
    return transactions.execute(status -> {
      // Keep the shared identity/device serialization fence ahead of grant,
      // session and key locks, matching device-authority issuance and revoke.
      lockIdentityAndDevice(device.accountId(), device.deviceId());
      Instant now = clock.instant();
      var authRows = jdbc.queryForList("""
          SELECT request_id,game_binding_status FROM sdk_authorizations
          WHERE game_binding_id=:binding FOR UPDATE
          """, Map.of("binding", device.bindingId()));
      if (authRows.size() != 1) throw denied();
      UUID requestId = (UUID) authRows.getFirst().get("request_id");
      var grantRows = jdbc.queryForList("""
          SELECT * FROM sdk_game_message_grants WHERE authorization_request_id=:request AND binding_id=:binding
          FOR UPDATE
          """, Map.of("request", requestId, "binding", device.bindingId()));
      if (grantRows.size() != 1) throw denied();
      Map<String, Object> grant = grantRows.getFirst();
      UUID grantId = (UUID) grant.get("grant_id");
      var prior = jdbc.queryForList("""
          SELECT grant_id,request_sha256,assertion_jti,permit_jws FROM sdk_game_message_execution_permits
          WHERE operation_id=:operation FOR UPDATE
          """, Map.of("operation", operationId));
      if (!prior.isEmpty()) {
        Map<String, Object> saved = prior.getFirst();
        if (!grantId.equals(saved.get("grant_id")) || !requestSha256.equals(saved.get("request_sha256"))
            || !device.jti().equals(saved.get("assertion_jti"))) throw conflict();
        return new Permit((String) saved.get("permit_jws"));
      }
      if (!"active".equals(authRows.getFirst().get("game_binding_status"))
          || !"active".equals(grant.get("status"))) throw denied();
      requireAuthority(grant, device, assertion, now);
      SdkGameIntegrationExecutionPermitAuthority.Permit gisPermit = gis.issue(device.bindingId(), operationId, assertion);
      Instant signedAt = clock.instant();
      if (!gisPermit.bindingId().equals(device.bindingId()) || !gisPermit.applicationId().equals(device.applicationId())
          || !gisPermit.environmentId().equals(device.environmentId()) || !gisPermit.assertionJti().equals(device.jti())
          || !gisPermit.operationId().equals(operationId) || !gisPermit.expiresAt().isAfter(signedAt.plusMillis(500))
          || gisPermit.expiresAt().isAfter(signedAt.plusMillis(3750)) || gisPermit.expiresAt().toEpochMilli() > device.expiresAtMs()) {
        throw denied();
      }
      // Re-read every network-backed policy/profile authority after GIS admits; DB rows remain locked.
      requireAuthority(grant, device, assertion, signedAt);
      // Timestamp the assertion immediately after the final authority check so
      // its lifetime is measured from the signing boundary, not before remote
      // policy/profile calls.
      signedAt = clock.instant();
      if (!gisPermit.expiresAt().isAfter(signedAt.plusMillis(500))
          || gisPermit.expiresAt().isAfter(signedAt.plusMillis(3750))
          || gisPermit.expiresAt().toEpochMilli() > device.expiresAtMs()) throw denied();
      UUID permitJti = UUID.randomUUID();
      long issuedAt = signedAt.toEpochMilli();
      long expiresAt = gisPermit.expiresAt().toEpochMilli();
      var claims = new TreeMap<String, Object>();
      claims.put("version", 1L); claims.put("iss", "auth"); claims.put("aud", "voice.game-message");
      claims.put("jti", permitJti.toString()); claims.put("operation", "message.send"); claims.put("scope", "game.chat.send");
      claims.put("operation_id", operationId.toString()); claims.put("request_sha256", requestSha256);
      claims.put("application_id", device.applicationId().toString()); claims.put("environment_id", device.environmentId().toString());
      claims.put("account_id", device.accountId().toString()); claims.put("actor_id", device.actorId().toString());
      claims.put("binding_id", device.bindingId().toString()); claims.put("profile_id", grant.get("target_profile_id").toString());
      claims.put("device_id", device.deviceId().toString()); claims.put("key_id", device.keyId().toString());
      claims.put("device_generation", device.deviceGeneration()); claims.put("authority_revision", device.authorityRevision());
      claims.put("gis_permit_id", gisPermit.permitId().toString()); claims.put("binding_revision", gisPermit.bindingRevision());
      claims.put("assertion_jti", device.jti().toString()); claims.put("iat_ms", issuedAt);
      claims.put("expires_at_ms", expiresAt); claims.put("exp", Math.floorDiv(expiresAt, 1000));
      String compact = issuer.issueGameMessageExecutionPermit(claims);
      jdbc.update("""
          INSERT INTO sdk_game_message_execution_permits(permit_jti,grant_id,operation_id,request_sha256,assertion_jti,
            gis_permit_id,binding_revision,permit_jws,issued_at_ms,expires_at_ms,created_at)
          VALUES (:jti,:grant,:operation,:hash,:assertion,:gis,:bindingRevision,:token,:issued,:expires,:created)
          """, new MapSqlParameterSource().addValue("jti", permitJti).addValue("grant", grantId)
              .addValue("operation", operationId).addValue("hash", requestSha256).addValue("assertion", device.jti())
              .addValue("gis", gisPermit.permitId()).addValue("bindingRevision", gisPermit.bindingRevision())
              .addValue("token", compact).addValue("issued", issuedAt).addValue("expires", expiresAt)
              .addValue("created", Timestamp.from(signedAt)));
      return new Permit(compact);
    });
  }

  public Completion complete(UUID permitJti, UUID operationId, String outcome) {
    if (permitJti == null || operationId == null || !Set.of("committed", "aborted").contains(outcome)) throw denied();
    return transactions.execute(status -> {
      // Issue and completion both serialize grant -> permit. The initial lookup
      // is deliberately unlocked because grant_id is immutable; after taking
      // the grant lock, re-read and lock the permit row in that same order.
      var ownership = jdbc.queryForList("""
          SELECT grant_id FROM sdk_game_message_execution_permits WHERE permit_jti=:jti
          """, Map.of("jti", permitJti));
      if (ownership.size() != 1) throw denied();
      UUID grantId = (UUID) ownership.getFirst().get("grant_id");
      var grants = jdbc.queryForList("""
          SELECT status FROM sdk_game_message_grants WHERE grant_id=:grant FOR UPDATE
          """, Map.of("grant", grantId));
      if (grants.size() != 1) throw denied();
      var rows = jdbc.queryForList("""
          SELECT * FROM sdk_game_message_execution_permits
          WHERE permit_jti=:jti AND grant_id=:grant FOR UPDATE
          """, Map.of("jti", permitJti, "grant", grantId));
      if (rows.size() != 1) throw denied();
      Map<String, Object> row = rows.getFirst();
      if (!grantId.equals(row.get("grant_id"))) throw denied();
      if (!operationId.equals(row.get("operation_id"))) throw conflict();
      if (row.get("completion_outcome") != null) {
        if (!outcome.equals(row.get("completion_outcome"))) throw conflict();
        return new Completion((UUID) row.get("gis_permit_id"), operationId, outcome, "completed");
      }
      var completed = gis.complete((UUID) row.get("gis_permit_id"), operationId, outcome);
      if (!completed.permitId().equals(row.get("gis_permit_id")) || !completed.operationId().equals(operationId)
          || !outcome.equals(completed.outcome()) || !"completed".equals(completed.status())) throw denied();
      String receipt = "{\"outcome\":\"" + outcome + "\",\"operation_id\":\"" + operationId
          + "\",\"permit_id\":\"" + completed.permitId() + "\",\"status\":\"completed\"}";
      jdbc.update("""
          UPDATE sdk_game_message_execution_permits SET completion_outcome=:outcome,
            completion_receipt=CAST(:receipt AS JSONB),completed_at=:now
          WHERE permit_jti=:jti AND completion_outcome IS NULL
          """, Map.of("outcome", outcome, "receipt", receipt, "now", Timestamp.from(clock.instant()), "jti", permitJti));
      return new Completion(completed.permitId(), operationId, outcome, "completed");
    });
  }

  private void requireAuthority(Map<String, Object> grant, AuthUserPrincipalIssuer.VerifiedDeviceStatus device,
      String assertion, Instant now) {
    if (!device.applicationId().equals(grant.get("application_id"))
        || !device.environmentId().equals(grant.get("environment_id"))
        || !device.accountId().equals(queryUuid("SELECT source_account_id FROM sdk_authorizations WHERE request_id=:id", grant))
        || !device.bindingId().equals(grant.get("binding_id"))) throw denied();
    UUID targetAccount = (UUID) grant.get("target_account_id");
    UUID targetProfile = (UUID) grant.get("target_profile_id");
    Long currentEpoch = jdbc.queryForObject("SELECT session_epoch FROM accounts WHERE id=:id AND status='active' AND type='regular'",
        Map.of("id", targetAccount), Long.class);
    if (currentEpoch == null || currentEpoch.longValue() != ((Number) grant.get("target_epoch")).longValue()) throw denied();
    var receipt = jdbc.queryForList("""
        SELECT a.game_binding_status,a.game_binding_id,a.game_binding_consent_revision,s.consent_revision,
               s.expires_at,a.scopes,a.policy_revision,a.profile_revision
        FROM sdk_authorizations a JOIN sdk_linked_sessions s ON s.request_id=a.request_id
        WHERE a.request_id=:request
        """, Map.of("request", grant.get("authorization_request_id")));
    if (receipt.size() != 1) throw denied();
    Map<String, Object> authorization = receipt.getFirst();
    long consent = ((Number) grant.get("consent_revision")).longValue();
    if (!"active".equals(authorization.get("game_binding_status"))
        || !device.bindingId().equals(authorization.get("game_binding_id"))
        || authorization.get("game_binding_consent_revision") == null
        || ((Number) authorization.get("game_binding_consent_revision")).longValue() != consent
        || ((Number) authorization.get("consent_revision")).longValue() != consent
        || !grant.get("scopes").equals(authorization.get("scopes"))
        || ((Number) grant.get("policy_revision")).longValue() != ((Number) authorization.get("policy_revision")).longValue()
        || ((Number) grant.get("profile_revision")).longValue() != ((Number) authorization.get("profile_revision")).longValue()) throw denied();
    List<String> scopes = Arrays.stream(((String) grant.get("scopes")).split(",", -1)).toList();
    if (scopes.isEmpty() || !scopes.equals(scopes.stream().sorted().distinct().toList())
        || !scopes.contains("game.chat.send")) throw denied();
    SdkAuthorizationPolicy.Policy policy = policies.resolveForExecutionPermit(device.applicationId(), device.environmentId());
    if (policy == null || !device.applicationId().equals(policy.applicationId())
        || !device.environmentId().equals(policy.environmentId())
        || policy.revision() != ((Number) grant.get("policy_revision")).longValue()
        || policy.playerScopes() == null || !policy.playerScopes().contains("game.chat.send")) throw denied();
    SdkProfileEligibility.Profile profile = profiles.inspectForExecutionPermit(targetAccount, targetProfile);
    if (profile == null || !targetAccount.equals(profile.accountId()) || !targetProfile.equals(profile.profileId())
        || profile.deleted() || profile.frozen() || profile.revision() != ((Number) grant.get("profile_revision")).longValue()) throw denied();
    List<Map<String, Object>> deviceRows = jdbc.queryForList("""
        SELECT i.actor_id,i.application_id,i.environment_id,i.ownership_generation,i.status AS identity_status,
               d.authority_revision,d.revoked_at,k.generation,k.key_thumbprint,k.status AS key_status,k.not_before,k.not_after,
               k.not_after AS current_key_not_after,x.expires_at
        FROM sdk_device_authority_issues x
        JOIN sdk_sessions s ON s.token_hash=x.session_token_hash AND s.expires_at>:now
        JOIN sdk_identities i ON i.account_id=s.account_id AND i.status='active'
          AND s.ownership_generation=i.ownership_generation
        JOIN sdk_devices d ON d.device_id=s.device_id AND d.revoked_at IS NULL
        JOIN sdk_device_keys k ON k.device_id=d.device_id AND k.key_id=:key AND k.revoked_at IS NULL
        WHERE x.assertion=:assertion AND x.device_id=:device AND x.key_id=:key
          AND i.application_id=:app AND i.environment_id=:env AND i.actor_id=:actor
          AND i.ownership_generation=:generation AND d.authority_revision=:authorityRevision
          AND k.generation=:generation AND k.key_thumbprint=:thumbprint
          AND k.status IN ('active','overlap') AND k.not_before<=:now AND k.not_after>:now AND x.expires_at>:now
        FOR UPDATE OF s,i,d,k,x
        """, new MapSqlParameterSource().addValue("now", Timestamp.from(now)).addValue("assertion", assertion)
            .addValue("device", device.deviceId()).addValue("key", device.keyId()).addValue("app", device.applicationId())
            .addValue("env", device.environmentId()).addValue("actor", device.actorId())
            .addValue("generation", device.deviceGeneration()).addValue("authorityRevision", device.authorityRevision())
            .addValue("thumbprint", device.keyThumbprint()));
    if (deviceRows.size() != 1) throw denied();
    Timestamp assertionExpires = (Timestamp) deviceRows.getFirst().get("expires_at");
    Timestamp currentKeyExpiry = (Timestamp) deviceRows.getFirst().get("current_key_not_after");
    if (assertionExpires == null || currentKeyExpiry == null
        || assertionExpires.toInstant().toEpochMilli() != device.expiresAtMs()
        || currentKeyExpiry.toInstant().toEpochMilli() != device.notAfterMs()) throw denied();
  }

  private UUID queryUuid(String sql, Map<String, Object> grant) {
    UUID request = (UUID) grant.get("authorization_request_id");
    return jdbc.queryForObject(sql, Map.of("id", request), UUID.class);
  }

  private void lockIdentityAndDevice(UUID accountId, UUID deviceId) {
    jdbc.query("SELECT account_id FROM sdk_identities WHERE account_id=:id FOR UPDATE",
        Map.of("id", accountId), rs -> { });
    Integer found = jdbc.query("SELECT device_id FROM sdk_devices WHERE device_id=:id AND account_id=:account FOR UPDATE",
        Map.of("id", deviceId, "account", accountId), rs -> rs.next() ? 1 : 0);
    if (found == null || found != 1) throw denied();
  }

  private static SdkIdentityDeniedException denied() { return new SdkIdentityDeniedException(); }
  private static SdkAuthorizationConflictException conflict() { return new SdkAuthorizationConflictException(); }
}
