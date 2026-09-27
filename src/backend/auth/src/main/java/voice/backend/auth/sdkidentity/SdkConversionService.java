package voice.backend.auth.sdkidentity;

import java.sql.Timestamp;
import java.time.Clock;
import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.TokenClaims;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

/** Durable preparation only. Binding ownership cannot advance without its owner's receipts. */
public class SdkConversionService {
  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final SdkIdentityService identity;
  private final SdkAuthorizationService authorization;
  private final AuthService auth;
  private final SdkProfileEligibility profiles;
  private final PrimaryProfileProvisioner primaries;
  private final TokenBlacklist blacklist;
  private final Clock clock;

  public record Operation(UUID operationId, String mode, String state, long revision,
                          UUID registrationIntentId, Instant registrationIntentExpiresAt,
                          UUID registeredAccountId, UUID targetAccountId, UUID targetProfileId) {}

  public SdkConversionService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
      SdkIdentityService identity, SdkAuthorizationService authorization, AuthService auth,
      SdkProfileEligibility profiles, PrimaryProfileProvisioner primaries, TokenBlacklist blacklist, Clock clock) {
    this.jdbc = jdbc;
    this.transactions = transactions;
    this.identity = identity;
    this.authorization = authorization;
    this.auth = auth;
    this.profiles = profiles;
    this.primaries = primaries;
    this.blacklist = blacklist;
    this.clock = clock;
  }

  public Operation prepareNew(String token, String proof, UUID key, UUID binding) {
    SdkConversionProofs.preparePayload("new", token, key, binding);
    return transactions.execute(transaction -> {
      var source = identity.prepareConversion(token, proof, key, binding);
      return prepare("new", key, binding, source.accountId(), source.deviceId(), source.applicationId(),
          source.environmentId(), source.expiresAt(), null, null, null);
    });
  }

  public Operation prepareExisting(String token, String proof, UUID key, UUID binding) {
    SdkConversionProofs.preparePayload("existing", token, key, binding);
    return transactions.execute(transaction -> {
      var linked = authorization.prepareConversion(token, proof, key, binding);
      var approvals = jdbc.queryForList("""
          SELECT a.target_epoch,a.approval_jti,a.approval_expires_at,a.profile_revision
          FROM sdk_linked_sessions s JOIN sdk_authorizations a ON a.request_id=s.request_id
          WHERE s.token_hash=:hash
          """, Map.of("hash", SdkIdentityService.hash(token)));
      if (approvals.isEmpty()) throw denied();
      return prepare("existing", key, binding, linked.sourceAccountId(), linked.deviceId(), linked.applicationId(),
          linked.environmentId(), linked.expiresAt(), linked.accountId(), linked.profileId(), new Row(approvals.getFirst()));
    });
  }

  private Operation prepare(String mode, UUID key, UUID binding, UUID source, UUID device, UUID app, UUID env,
      Instant proofExpires, UUID target, UUID selectedProfile, Row approval) {
    String digest = SdkConversionProofs.requestHash(mode, key, binding, target, selectedProfile);
    Row metadata = metadata(source, device);
    var existing = jdbc.queryForList(select() + " WHERE o.source_account_id=:source AND o.mode=:mode"
        + " AND o.idempotency_key=:key FOR UPDATE OF o", Map.of("source", source, "mode", mode, "key", key));
    fresh(proofExpires);
    if (!existing.isEmpty()) {
      Row row = new Row(existing.getFirst());
      if (!digest.equals(row.text("request_hash"))) throw new SdkAuthorizationConflictException();
      if (!device.equals(row.id("device_id")) || metadata.number("ownership_generation") != row.number("source_generation")) {
        throw denied();
      }
      return operation(row);
    }
    UUID operationId = UUID.randomUUID();
    var values = new MapSqlParameterSource().addValue("id", operationId).addValue("mode", mode)
        .addValue("source", source).addValue("device", device).addValue("jwk", metadata.text("public_jwk"))
        .addValue("app", app).addValue("env", env).addValue("generation", metadata.number("ownership_generation"))
        .addValue("binding", binding).addValue("key", key).addValue("hash", digest)
        .addValue("target", target).addValue("profile", selectedProfile).addValue("now", Timestamp.from(clock.instant()))
        .addValue("epoch", approval == null ? null : approval.number("target_epoch"))
        .addValue("jti", approval == null ? null : approval.text("approval_jti"))
        .addValue("approvalExpires", approval == null ? null : Timestamp.from(approval.time("approval_expires_at")))
        .addValue("profileRevision", approval == null ? null : approval.number("profile_revision"));
    jdbc.update("""
        INSERT INTO sdk_conversion_operations(operation_id,mode,source_account_id,device_id,public_jwk,
          application_id,environment_id,source_generation,binding_id,idempotency_key,request_hash,
          target_account_id,target_profile_id,target_epoch,approval_jti,approval_expires_at,profile_revision,created_at)
        VALUES (:id,:mode,:source,:device,:jwk,:app,:env,:generation,:binding,:key,:hash,
          :target,:profile,:epoch,:jti,:approvalExpires,:profileRevision,:now)
        """, values);
    if ("new".equals(mode)) {
      jdbc.update("""
          INSERT INTO sdk_registration_intents(intent_id,operation_id,expires_at) VALUES (:intent,:operation,:expires)
          """, Map.of("intent", UUID.randomUUID(), "operation", operationId,
              "expires", Timestamp.from(clock.instant().plusSeconds(900))));
    }
    return operation(read(operationId, false));
  }

  public Operation status(UUID operationId, long issuedAt, String deviceProof) {
    String payload = SdkConversionProofs.statusPayload(operationId, issuedAt, clock);
    Row row = read(operationId, false);
    SdkIdentityService.possession(row.text("public_jwk"), deviceProof, payload);
    // A revoked/retired source key authorizes this purpose-limited recovery read, never new access.
    SdkConversionProofs.statusPayload(operationId, issuedAt, clock);
    return operation(row);
  }

  public Operation attachNewTarget(UUID operationId, String voiceBearer) {
    TokenClaims claims = voice(voiceBearer);
    UUID account;
    try { account = UUID.fromString(claims.userId()); }
    catch (RuntimeException invalid) { throw denied(); }
    return transactions.execute(transaction -> {
      Row row = locked(operationId);
      if (!"new".equals(row.text("mode")) || !"prepared".equals(row.text("state"))
          || !account.equals(row.id("registered_account_id"))) throw denied();
      currentSource(row);
      currentTarget(account, claims);
      UUID primary;
      try { primary = UUID.fromString(primaries.ensurePrimaryProfile(account, "", false)); }
      catch (RuntimeException unavailable) { throw denied(); }
      SdkProfileEligibility.Profile profile;
      try {
        profile = profiles.inspect(account, primary);
        if (profile == null || !account.equals(profile.accountId()) || !primary.equals(profile.profileId())
            || profile.revision() <= 0 || profile.deleted() || profile.frozen()) throw denied();
      } catch (RuntimeException unavailable) { throw denied(); }
      fresh(claims.expiresAt());
      if (row.id("target_account_id") != null) {
        if (!account.equals(row.id("target_account_id")) || !primary.equals(row.id("target_profile_id"))) throw denied();
        if (claims.jti().equals(row.text("approval_jti")) && claims.sessionEpoch() == row.number("target_epoch")
            && profile.revision() == row.number("profile_revision")) return operation(row);
      }
      jdbc.update("""
          UPDATE sdk_conversion_operations SET target_account_id=:account,target_profile_id=:profile,
            target_epoch=:epoch,approval_jti=:jti,approval_expires_at=:expires,profile_revision=:profileRevision,
            revision=revision+1 WHERE operation_id=:operation
          """, Map.of("account", account, "profile", primary, "epoch", claims.sessionEpoch(), "jti", claims.jti(),
              "expires", Timestamp.from(claims.expiresAt()), "profileRevision", profile.revision(), "operation", operationId));
      return operation(read(operationId, false));
    });
  }

  private Row locked(UUID operationId) {
    Row observed = read(operationId, false);
    jdbc.query("SELECT account_id FROM sdk_identities WHERE account_id=:id FOR UPDATE",
        Map.of("id", observed.id("source_account_id")), rs -> { });
    return read(operationId, true);
  }

  private Row metadata(UUID source, UUID device) {
    var rows = jdbc.queryForList("""
        SELECT i.ownership_generation,d.public_jwk FROM sdk_identities i
        JOIN sdk_devices d ON d.account_id=i.account_id
        WHERE i.account_id=:source AND d.device_id=:device AND i.status='active' AND d.revoked_at IS NULL
        FOR UPDATE OF d
        """, Map.of("source", source, "device", device));
    if (rows.isEmpty()) throw denied();
    return new Row(rows.getFirst());
  }

  private void currentSource(Row operation) {
    identity.requireAdmitted(operation.id("application_id"), operation.id("environment_id"));
    Row source = metadata(operation.id("source_account_id"), operation.id("device_id"));
    if (source.number("ownership_generation") != operation.number("source_generation")) throw denied();
  }

  private void currentTarget(UUID account, TokenClaims claims) {
    var rows = jdbc.queryForList("""
        SELECT type,status,session_epoch,regular_email_verification_pending FROM accounts WHERE id=:id FOR UPDATE
        """, Map.of("id", account));
    if (rows.isEmpty()) throw denied();
    Row row = new Row(rows.getFirst());
    if (!"regular".equals(row.text("type")) || !"active".equals(row.text("status"))
        || Boolean.TRUE.equals(row.values().get("regular_email_verification_pending"))
        || row.number("session_epoch") != claims.sessionEpoch()) throw denied();
    try {
      var epoch = auth.prepareOAuthAccessToken(account.toString());
      if (epoch == null || !account.equals(epoch.accountId()) || epoch.sessionEpoch() != claims.sessionEpoch()
          || blacklist.isRevoked(claims.jti())) throw denied();
    } catch (RuntimeException unavailable) { throw denied(); }
    fresh(claims.expiresAt());
  }

  private TokenClaims voice(String bearer) {
    try {
      TokenClaims claims = auth.validate(bearer);
      if (claims == null || claims.userId() == null || claims.jti() == null || claims.jti().isBlank()
          || claims.sessionEpoch() <= 0 || !"regular".equals(claims.normalizedAccountType())) throw denied();
      return claims;
    } catch (RuntimeException invalid) { throw denied(); }
  }

  private Row read(UUID operationId, boolean lock) {
    if (operationId == null) throw denied();
    var rows = jdbc.queryForList(select() + " WHERE o.operation_id=:id" + (lock ? " FOR UPDATE OF o" : ""),
        Map.of("id", operationId));
    if (rows.isEmpty()) throw denied();
    return new Row(rows.getFirst());
  }

  private static String select() {
    return """
        SELECT o.*,r.intent_id AS registration_intent_id,r.expires_at AS registration_intent_expires_at
        FROM sdk_conversion_operations o LEFT JOIN sdk_registration_intents r ON r.operation_id=o.operation_id
        """;
  }

  private static Operation operation(Row row) {
    return new Operation(row.id("operation_id"), row.text("mode"), row.text("state"), row.number("revision"),
        row.id("registration_intent_id"), row.time("registration_intent_expires_at"), row.id("registered_account_id"),
        row.id("target_account_id"), row.id("target_profile_id"));
  }

  private void fresh(Instant expires) { if (expires == null || !expires.isAfter(clock.instant())) throw denied(); }
  private static SdkIdentityDeniedException denied() { return new SdkIdentityDeniedException(); }

  private record Row(Map<String, Object> values) {
    UUID id(String key) { return (UUID) values.get(key); }
    String text(String key) { return (String) values.get(key); }
    long number(String key) { return ((Number) values.get(key)).longValue(); }
    Instant time(String key) { return values.get(key) == null ? null : ((Timestamp) values.get(key)).toInstant(); }
  }
}
