package voice.backend.auth.sdkidentity;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.sql.Timestamp;
import java.time.Clock;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.security.TokenBlacklist;

/** Durable online claim and completion ledger for GIS binding exchange. */
public class AuthGameBindingHandoffService {
  private record HandoffVerification(AuthUserPrincipalIssuer.VerifiedGameBindingHandoff handoff, boolean fresh) {}
  public record ClaimReceipt(UUID claimId, UUID operationId, UUID assertionJti, Instant expiresAt) {}
  public record CompletionReceipt(UUID claimId, UUID operationId, String outcome, UUID bindingId, String status) {}
  public record RevocationReceipt(UUID operationId, String status, long authorityRevision) {}

  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final AuthUserPrincipalIssuer issuer;
  private final SdkAuthorizationPolicy policies;
  private final SdkProfileEligibility profiles;
  private final SdkIdentityService identity;
  private final TokenBlacklist blacklist;
  private final Clock clock;

  public AuthGameBindingHandoffService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
      AuthUserPrincipalIssuer issuer, SdkAuthorizationPolicy policies, SdkProfileEligibility profiles,
      SdkIdentityService identity, TokenBlacklist blacklist, Clock clock) {
    this.jdbc = jdbc;
    this.transactions = transactions;
    this.issuer = issuer;
    this.policies = policies;
    this.profiles = profiles;
    this.identity = identity;
    this.blacklist = blacklist;
    this.clock = clock;
  }

  public ClaimReceipt claim(String handoffJws, String deviceProof, UUID operationId, String requestSha256) {
    HandoffVerification verification = verifyForClaim(handoffJws);
    final AuthUserPrincipalIssuer.VerifiedGameBindingHandoff handoff = verification.handoff();
    final boolean freshAssertion = verification.fresh();
    if (operationId == null || !operationId.equals(handoff.operationId()) || requestSha256 == null
        || !requestSha256.matches("[0-9a-f]{64}")) throw denied();
    return transactions.execute(status -> {
      // Match the SDK identity -> authorization lock order used by identity revoke and consent exchange.
      jdbc.query("SELECT account_id FROM sdk_identities WHERE account_id=:id FOR UPDATE",
          Map.of("id", handoff.sourceAccountId()), rs -> { });
      List<Map<String, Object>> grants = jdbc.queryForList("""
          SELECT a.*, s.consent_revision, s.expires_at AS linked_expires_at, i.actor_id,
                 i.application_id AS identity_application_id, i.environment_id AS identity_environment_id,
                 i.ownership_generation, i.status AS identity_status,
                 h.challenge_id AS handoff_challenge_id, h.operation_id AS handoff_operation_id,
                 h.assertion_jti AS handoff_assertion_jti, h.device_id AS handoff_device_id,
                 h.device_proof_sha256 AS handoff_device_proof_sha256,
                 d.thumbprint, d.public_jwk, d.revoked_at, target.type AS target_type,
                 target.status AS target_status, target.session_epoch AS target_session_epoch,
                 target.regular_email_verification_pending AS target_email_pending
          FROM sdk_authorizations a
          JOIN sdk_linked_sessions s ON s.request_id=a.request_id
          JOIN sdk_game_binding_handoff_issuances h ON h.authorization_request_id=a.request_id
          JOIN sdk_identities i ON i.account_id=a.source_account_id
          JOIN sdk_devices d ON d.device_id=a.device_id AND d.account_id=a.source_account_id
          JOIN accounts target ON target.id=a.target_account_id
          WHERE a.request_id=:request FOR UPDATE OF a,s,i,d,target
          """, Map.of("request", handoff.authorizationRequestId()));
      if (grants.isEmpty()) throw denied();
      Map<String, Object> row = grants.getFirst();
      List<Map<String, Object>> prior = jdbc.queryForList("""
          SELECT claim_id, operation_id, assertion_jti, request_sha256, authorization_request_id, source_device_id, expires_at
          FROM sdk_game_binding_handoff_claims WHERE operation_id=:operation OR assertion_jti=:jti
          FOR UPDATE
          """, Map.of("operation", operationId, "jti", handoff.assertionJti()));
      if (!prior.isEmpty()) {
        Map<String, Object> existing = prior.getFirst();
        if (!operationId.equals(existing.get("operation_id")) || !handoff.assertionJti().equals(existing.get("assertion_jti"))
            || !handoff.authorizationRequestId().equals(existing.get("authorization_request_id"))
            || !handoff.sourceDeviceId().equals(existing.get("source_device_id"))
            || !requestSha256.equals(existing.get("request_sha256"))) throw conflict();
        checkReplayIdentity(row, handoff);
        requirePersistedDeviceProof(row, handoff, deviceProof);
        return new ClaimReceipt((UUID) existing.get("claim_id"), operationId, handoff.assertionJti(),
            ((Timestamp) existing.get("expires_at")).toInstant());
      }
      if (!freshAssertion) throw denied();
      checkGrant(row, handoff);
      requirePersistedDeviceProof(row, handoff, deviceProof);
      if ("revoking".equals(row.get("game_binding_status")) || "revoked".equals(row.get("game_binding_status"))) throw denied();
      UUID claimId = UUID.randomUUID();
      jdbc.update("""
          INSERT INTO sdk_game_binding_handoff_claims(claim_id,authorization_request_id,challenge_id,assertion_jti,
            operation_id,request_sha256,source_device_id,state,expires_at)
          VALUES (:claim,:request,:challenge,:jti,:operation,:hash,:device,'claimed',:expires)
          """, new MapSqlParameterSource().addValue("claim", claimId).addValue("request", handoff.authorizationRequestId())
              .addValue("challenge", handoff.challengeId()).addValue("jti", handoff.assertionJti())
              .addValue("operation", operationId).addValue("hash", requestSha256).addValue("device", handoff.sourceDeviceId())
              .addValue("expires", Timestamp.from(handoff.expiresAt())));
      return new ClaimReceipt(claimId, operationId, handoff.assertionJti(), handoff.expiresAt());
    });
  }

  private HandoffVerification verifyForClaim(String handoffJws) {
    try { return new HandoffVerification(issuer.verifyGameBindingHandoff(handoffJws), true); }
    catch (IllegalArgumentException expiredOrStale) {
      return new HandoffVerification(issuer.verifyGameBindingHandoffForExactReplay(handoffJws), false);
    }
  }

  public CompletionReceipt complete(UUID claimId, UUID operationId, String outcome, UUID bindingId) {
    if (claimId == null || operationId == null || !("succeeded".equals(outcome) || "failed".equals(outcome))
        || ("succeeded".equals(outcome) != (bindingId != null))) throw denied();
    return transactions.execute(status -> {
      // The claim's authorization owner is immutable. Read it without locks,
      // then take the same identity -> authorization/session -> claim order as
      // claim() and revoke(), and re-read/revalidate after each lock.
      var ownerRows = jdbc.queryForList("""
          SELECT c.authorization_request_id,a.source_account_id
          FROM sdk_game_binding_handoff_claims c
          JOIN sdk_authorizations a ON a.request_id=c.authorization_request_id
          WHERE c.claim_id=:claim
          """, Map.of("claim", claimId));
      if (ownerRows.size() != 1 || ownerRows.getFirst().get("source_account_id") == null) throw denied();
      UUID requestId = (UUID) ownerRows.getFirst().get("authorization_request_id");
      UUID sourceAccountId = (UUID) ownerRows.getFirst().get("source_account_id");
      var identities = jdbc.queryForList("""
          SELECT account_id FROM sdk_identities WHERE account_id=:account FOR UPDATE
          """, Map.of("account", sourceAccountId));
      if (identities.size() != 1) throw denied();
      var authorizationRows = jdbc.queryForList("""
          SELECT request_id FROM sdk_authorizations WHERE request_id=:request FOR UPDATE
          """, Map.of("request", requestId));
      if (authorizationRows.size() != 1) throw denied();
      var sessionRows = jdbc.queryForList("""
          SELECT request_id FROM sdk_linked_sessions WHERE request_id=:request FOR UPDATE
          """, Map.of("request", requestId));
      if (sessionRows.size() != 1) throw denied();
      var rows = jdbc.queryForList("""
          SELECT * FROM sdk_game_binding_handoff_claims WHERE claim_id=:claim FOR UPDATE
          """, Map.of("claim", claimId));
      if (rows.isEmpty()) throw denied();
      Map<String, Object> row = rows.getFirst();
      if (!requestId.equals(row.get("authorization_request_id"))) throw denied();
      if (!operationId.equals(row.get("operation_id"))) throw conflict();
      String state = (String) row.get("state");
      if (!"claimed".equals(state)) {
        boolean same = ("completed".equals(state) && "succeeded".equals(outcome)
            && bindingId.equals(row.get("result_binding_id")))
            || ("failed".equals(state) && "failed".equals(outcome));
        if (!same) throw conflict();
        return new CompletionReceipt(claimId, operationId, outcome, (UUID) row.get("result_binding_id"), "completed");
      }
      if ("succeeded".equals(outcome)) {
        int changed = jdbc.update("""
            UPDATE sdk_authorizations SET game_binding_id=:binding,
              game_binding_status=CASE WHEN game_binding_status='revoking' THEN 'revoking' ELSE 'active' END,
              game_binding_authority_revision=game_binding_authority_revision+1
            WHERE request_id=:request AND game_binding_status IN ('unbound','active','revoking')
              AND (game_binding_id IS NULL OR game_binding_id=:binding)
            """, Map.of("binding", bindingId, "request", requestId));
        if (changed != 1) throw denied();
        var authority = jdbc.queryForList("""
            SELECT a.application_id,a.environment_id,a.target_account_id,a.target_profile_id,a.target_epoch,a.scopes,
                   a.policy_revision,a.profile_revision,a.game_binding_authority_revision,
                   a.game_binding_status,s.consent_revision
            FROM sdk_authorizations a JOIN sdk_linked_sessions s ON s.request_id=a.request_id
            WHERE a.request_id=:request FOR UPDATE OF a,s
            """, Map.of("request", requestId));
        if (authority.size() != 1) throw denied();
        Map<String, Object> grant = authority.getFirst();
        if ("active".equals(grant.get("game_binding_status"))) {
          Instant now = clock.instant();
          jdbc.update("""
              INSERT INTO sdk_game_message_grants(grant_id,authorization_request_id,application_id,environment_id,
                target_account_id,target_profile_id,target_epoch,binding_id,consent_revision,scopes,policy_revision,profile_revision,
                status,authority_revision,created_at,updated_at)
              VALUES (:id,:request,:app,:env,:account,:profile,:epoch,:binding,:consent,:scopes,:policy,:profileRevision,
                'active',1,:now,:now)
              """, new MapSqlParameterSource().addValue("id", UUID.randomUUID()).addValue("request", requestId)
                  .addValue("app", grant.get("application_id")).addValue("env", grant.get("environment_id"))
                  .addValue("account", grant.get("target_account_id")).addValue("profile", grant.get("target_profile_id"))
                  .addValue("epoch", grant.get("target_epoch"))
                  .addValue("binding", bindingId).addValue("consent", grant.get("consent_revision"))
                  .addValue("scopes", grant.get("scopes")).addValue("policy", grant.get("policy_revision"))
                  .addValue("profileRevision", grant.get("profile_revision")).addValue("now", Timestamp.from(now)));
        }
      }
      jdbc.update("""
          UPDATE sdk_game_binding_handoff_claims SET state=:state,result_binding_id=:binding,result_sha256=:hash,
            completed_at=:now WHERE claim_id=:claim AND state='claimed'
          """, new MapSqlParameterSource().addValue("state", "succeeded".equals(outcome) ? "completed" : "failed")
              .addValue("binding", bindingId).addValue("hash", sha256(outcome + ":" + (bindingId == null ? "" : bindingId)))
              .addValue("now", Timestamp.from(clock.instant())).addValue("claim", claimId));
      return new CompletionReceipt(claimId, operationId, outcome, bindingId, "completed");
    });
  }

  /**
   * Stops new handoff claims for an operation and acknowledges only after all
   * already accepted GIS operations have a durable completion receipt. A
   * pending response is retried with the same operation ID after GIS recovery.
   */
  public RevocationReceipt revoke(UUID operationId) {
    if (operationId == null) throw denied();
    // source_account_id is immutable for this authorization. Read it before
    // the transaction so the transaction can acquire the common identity
    // serialization lock before the authorization and grant rows.
    List<Map<String, Object>> owners = jdbc.queryForList("""
        SELECT source_account_id FROM sdk_authorizations WHERE game_binding_operation_id=:operation
        """, Map.of("operation", operationId));
    if (owners.size() != 1 || owners.getFirst().get("source_account_id") == null) throw denied();
    UUID sourceAccountId = (UUID) owners.getFirst().get("source_account_id");
    return transactions.execute(status -> {
      List<Map<String, Object>> identities = jdbc.queryForList("""
          SELECT account_id FROM sdk_identities WHERE account_id=:account FOR UPDATE
          """, Map.of("account", sourceAccountId));
      if (identities.size() != 1) throw denied();
      var rows = jdbc.queryForList("""
          SELECT request_id, game_binding_status, game_binding_authority_revision
          FROM sdk_authorizations WHERE game_binding_operation_id=:operation FOR UPDATE
          """, Map.of("operation", operationId));
      if (rows.isEmpty()) throw denied();
      Map<String, Object> row = rows.getFirst();
      UUID requestId = (UUID) row.get("request_id");
      String bindingStatus = (String) row.get("game_binding_status");
      long revision = ((Number) row.get("game_binding_authority_revision")).longValue();
      if ("revoked".equals(bindingStatus)) return new RevocationReceipt(operationId, "revoked", revision);
      if ("active".equals(bindingStatus) || "unbound".equals(bindingStatus)) {
        int changed = jdbc.update("""
            UPDATE sdk_authorizations SET game_binding_status='revoking',
              game_binding_authority_revision=game_binding_authority_revision+1
            WHERE request_id=:request AND game_binding_status=:status
            """, Map.of("request", requestId, "status", bindingStatus));
        if (changed != 1) throw conflict();
        revision++;
      } else if (!"revoking".equals(bindingStatus)) {
        throw conflict();
      }
      List<Map<String, Object>> gameGrants = jdbc.queryForList("""
          SELECT grant_id,status,authority_revision FROM sdk_game_message_grants
          WHERE authorization_request_id=:request FOR UPDATE
          """, Map.of("request", requestId));
      for (Map<String, Object> gameGrant : gameGrants) {
        UUID grantId = (UUID) gameGrant.get("grant_id");
        String grantStatus = (String) gameGrant.get("status");
        if ("active".equals(grantStatus)) {
          jdbc.update("""
              UPDATE sdk_game_message_grants SET status='revoking',authority_revision=authority_revision+1,updated_at=:now
              WHERE grant_id=:grant AND status='active'
              """, Map.of("now", Timestamp.from(clock.instant()), "grant", grantId));
        } else if (!"revoking".equals(grantStatus) && !"revoked".equals(grantStatus)) throw conflict();
      }
      Long outstanding = jdbc.queryForObject("""
          SELECT count(*) FROM sdk_game_binding_handoff_claims
          WHERE authorization_request_id=:request AND state='claimed'
          """, Map.of("request", requestId), Long.class);
      if (outstanding == null) throw denied();
      if (outstanding > 0) return new RevocationReceipt(operationId, "revoking", revision);
      boolean permitsDraining = false;
      for (Map<String, Object> gameGrant : gameGrants) {
        UUID grantId = (UUID) gameGrant.get("grant_id");
        Long permits = jdbc.queryForObject("""
            SELECT count(*) FROM sdk_game_message_execution_permits
            WHERE grant_id=:grant AND completion_outcome IS NULL AND expires_at_ms > :drainDeadline
            """, Map.of("grant", grantId, "drainDeadline", clock.instant().toEpochMilli() - 500), Long.class);
        if (permits == null) throw denied();
        if (permits > 0) permitsDraining = true;
      }
      if (permitsDraining) return new RevocationReceipt(operationId, "revoking", revision);
      int changed = jdbc.update("""
          UPDATE sdk_authorizations SET game_binding_status='revoked',
            game_binding_authority_revision=game_binding_authority_revision+1
          WHERE request_id=:request AND game_binding_operation_id=:operation AND game_binding_status='revoking'
          """, Map.of("request", requestId, "operation", operationId));
      if (changed != 1) throw conflict();
      jdbc.update("""
          UPDATE sdk_game_message_grants SET status='revoked',authority_revision=authority_revision+1,updated_at=:now
          WHERE authorization_request_id=:request AND status='revoking'
          """, Map.of("now", Timestamp.from(clock.instant()), "request", requestId));
      return new RevocationReceipt(operationId, "revoked", revision + 1);
    });
  }

  private void checkGrant(Map<String, Object> row, AuthUserPrincipalIssuer.VerifiedGameBindingHandoff handoff) {
    boolean valid = handoff.authorizationRequestId().equals(row.get("request_id"))
        && handoff.sourceAccountId().equals(row.get("source_account_id"))
        && handoff.sourceActorId().equals(row.get("actor_id")) && handoff.sourceDeviceId().equals(row.get("device_id"))
        && handoff.applicationId().equals(row.get("application_id")) && handoff.environmentId().equals(row.get("environment_id"))
        && handoff.applicationId().equals(row.get("identity_application_id"))
        && handoff.environmentId().equals(row.get("identity_environment_id"))
        && "active".equals(row.get("identity_status")) && row.get("revoked_at") == null
        && handoff.deviceGeneration() == ((Number) row.get("ownership_generation")).longValue()
        && handoff.targetAccountId().equals(row.get("target_account_id"))
        && handoff.targetProfileId().equals(row.get("target_profile_id"))
        && handoff.profileRevision() == ((Number) row.get("profile_revision")).longValue()
        && handoff.policyRevision() == ((Number) row.get("policy_revision")).longValue()
        && handoff.consentRevision() == ((Number) row.get("consent_revision")).longValue()
        && handoff.challengeId().equals(row.get("handoff_challenge_id"))
        && handoff.operationId().equals(row.get("handoff_operation_id"))
        && handoff.assertionJti().equals(row.get("handoff_assertion_jti"))
        && handoff.sourceDeviceId().equals(row.get("handoff_device_id"))
        && handoff.deviceKeyId().equals(row.get("device_id"))
        && handoff.deviceKeyThumbprint().equals(row.get("thumbprint"))
        && handoff.pkceChallenge().equals(row.get("code_challenge"))
        && handoff.scopes().equals(((String) row.get("scopes")).isBlank() ? List.of() : List.of(((String) row.get("scopes")).split(",")))
        && row.get("consumed_at") != null && row.get("code_hash") != null;
    String redirect = (String) row.get("redirect_uri");
    valid &= handoff.redirectUriSha256().equals(sha256(redirect));
    Timestamp authExpiry = (Timestamp) row.get("approval_expires_at");
    valid &= authExpiry != null && authExpiry.toInstant().isAfter(clock.instant())
        && row.get("linked_expires_at") instanceof Timestamp linkedExpiry && linkedExpiry.toInstant().isAfter(clock.instant())
        && handoff.expiresAt().isAfter(clock.instant());
    valid &= !blacklist.isRevoked((String) row.get("approval_jti"));
    try {
      identity.requireAdmitted(handoff.applicationId(), handoff.environmentId());
      valid &= "regular".equals(row.get("target_type")) && "active".equals(row.get("target_status"))
          && !Boolean.TRUE.equals(row.get("target_email_pending"))
          && ((Number) row.get("target_session_epoch")).longValue() == ((Number) row.get("target_epoch")).longValue();
      var policy = policies.resolve(handoff.applicationId(), handoff.environmentId());
      SdkAuthorizationProtocol.requirePolicy(policy, handoff.applicationId(), handoff.environmentId(), redirect,
          Set.copyOf(handoff.scopes()));
      valid &= policy.revision() == handoff.policyRevision();
      var profile = profiles.inspect(handoff.targetAccountId(), handoff.targetProfileId());
      valid &= profile != null && !profile.deleted() && !profile.frozen()
          && profile.revision() == handoff.profileRevision();
    } catch (RuntimeException unavailable) { valid = false; }
    if (!valid) throw denied();
  }

  private static void checkReplayIdentity(Map<String, Object> row,
      AuthUserPrincipalIssuer.VerifiedGameBindingHandoff handoff) {
    boolean exact = handoff.authorizationRequestId().equals(row.get("request_id"))
        && handoff.sourceAccountId().equals(row.get("source_account_id"))
        && handoff.sourceActorId().equals(row.get("actor_id"))
        && handoff.sourceDeviceId().equals(row.get("device_id"))
        && handoff.applicationId().equals(row.get("application_id"))
        && handoff.environmentId().equals(row.get("environment_id"))
        && handoff.deviceKeyId().equals(row.get("device_id"))
        && handoff.deviceKeyThumbprint().equals(row.get("thumbprint"))
        && handoff.targetAccountId().equals(row.get("target_account_id"))
        && handoff.targetProfileId().equals(row.get("target_profile_id"))
        && handoff.profileRevision() == ((Number) row.get("profile_revision")).longValue()
        && handoff.policyRevision() == ((Number) row.get("policy_revision")).longValue()
        && handoff.consentRevision() == ((Number) row.get("consent_revision")).longValue();
    exact &= handoff.challengeId().equals(row.get("handoff_challenge_id"))
        && handoff.operationId().equals(row.get("handoff_operation_id"))
        && handoff.assertionJti().equals(row.get("handoff_assertion_jti"))
        && handoff.sourceDeviceId().equals(row.get("handoff_device_id"));
    if (!exact) throw conflict();
  }

  private static void requirePersistedDeviceProof(Map<String, Object> row,
      AuthUserPrincipalIssuer.VerifiedGameBindingHandoff handoff, String deviceProof) {
    if (deviceProof == null || deviceProof.isBlank() || !handoff.sourceDeviceId().equals(row.get("handoff_device_id"))
        || !handoff.assertionJti().equals(row.get("handoff_assertion_jti"))
        || !sha256(deviceProof).equals(row.get("handoff_device_proof_sha256"))) throw conflict();
  }

  private static String sha256(String value) {
    try { return java.util.HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8))); }
    catch (Exception impossible) { throw new IllegalStateException(impossible); }
  }

  private static SdkIdentityDeniedException denied() { return new SdkIdentityDeniedException(); }
  private static SdkAuthorizationConflictException conflict() { return new SdkAuthorizationConflictException(); }
}
