package voice.backend.auth.sdkidentity;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSObject;
import com.nimbusds.jose.JOSEObjectType;
import com.nimbusds.jose.crypto.ECDSAVerifier;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jose.jwk.Curve;
import com.nimbusds.jose.jwk.ECKey;
import com.nimbusds.jwt.SignedJWT;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.sql.Timestamp;
import java.time.Clock;
import java.time.Instant;
import java.util.Base64;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.TreeMap;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.support.TransactionTemplate;

/** Auth-owned SDK bootstrap. Every admission reads current durable device/identity state. */
public class SdkIdentityService {
  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final GoogleOidcProofVerifier verifier;
  private final Map<String, SdkApplication> applications;
  private final SdkAuthorizationPolicy policies;
  private final Clock clock;
  private final SdkDeviceStatusIssuer statusIssuer;
  private final SdkBindingAuthority bindingAuthority;
  private final SecureRandom random = new SecureRandom();

  public record Challenge(UUID challengeId, String nonce, String clientId, Instant expiresAt,
                          String purpose, UUID deviceId, UUID replacesDeviceId) {
    public Challenge(UUID challengeId, String nonce, String clientId, Instant expiresAt) {
      this(challengeId, nonce, clientId, expiresAt, "enroll", null, null);
    }
  }
  public record DeviceKeyResult(UUID deviceId, UUID keyId, long generation, long authorityRevision) {}
  public record Session(UUID accountId, UUID actorId, UUID deviceId, UUID applicationId,
                        UUID environmentId, String gameSubject, String accessToken,
                        Instant expiresAt, String accountType, UUID keyId, long deviceGeneration) {}

  public SdkIdentityService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
                           GoogleOidcProofVerifier verifier, Map<String, SdkApplication> applications,
                           SdkAuthorizationPolicy policies, Clock clock) {
    this(jdbc, transactions, verifier, applications, policies, clock, null, null);
  }

  public SdkIdentityService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
                           GoogleOidcProofVerifier verifier, Map<String, SdkApplication> applications,
                           SdkAuthorizationPolicy policies, Clock clock,
                           SdkDeviceStatusIssuer statusIssuer, SdkBindingAuthority bindingAuthority) {
    this.jdbc = jdbc;
    this.transactions = transactions;
    this.verifier = verifier;
    this.applications = Map.copyOf(applications);
    this.policies = policies;
    this.clock = clock;
    this.statusIssuer = statusIssuer;
    this.bindingAuthority = bindingAuthority;
  }

  /** Issues an immutable, short-lived assertion only from current Auth and T16 binding state. */
  public String deviceAuthority(String sessionToken, String proof, byte[] requestBytes) {
    if (statusIssuer == null || bindingAuthority == null || sessionToken == null
        || !sessionToken.matches("[A-Za-z0-9_-]{43}") || proof == null || requestBytes == null
        || requestBytes.length == 0 || requestBytes.length > 4096
        || !Arrays.equals(requestBytes, proof.getBytes(StandardCharsets.US_ASCII))) {
      throw new SdkIdentityDeniedException();
    }
    AuthorityProof parsed = parseAuthorityProof(proof, requestBytes);
    return transactions.execute(status -> {
      jdbc.getJdbcTemplate().queryForObject("SELECT pg_advisory_xact_lock(?)", Object.class,
          parsed.requestId().getMostSignificantBits() ^ parsed.requestId().getLeastSignificantBits());
      String sessionHash = hash(sessionToken);
      var receipts = jdbc.query("SELECT request_bytes,session_token_hash,assertion FROM sdk_device_authority_issues WHERE request_id=:id",
          Map.of("id", parsed.requestId()), (rs, row) -> new AuthorityReceipt(rs.getBytes("request_bytes"),
              rs.getString("session_token_hash"), rs.getString("assertion")));
      if (!receipts.isEmpty()) {
        AuthorityReceipt receipt = receipts.getFirst();
        if (!Arrays.equals(receipt.requestBytes(), requestBytes)) throw new SdkDeviceKeyConflictException();
        if (!receipt.sessionTokenHash().equals(sessionHash)) throw new SdkIdentityDeniedException();
        return receipt.assertion();
      }
      UUID accountId = jdbc.query("SELECT account_id FROM sdk_sessions WHERE token_hash=:hash",
          Map.of("hash", sessionHash), (rs, row) -> rs.getObject("account_id", UUID.class))
          .stream().findFirst().orElseThrow(SdkIdentityDeniedException::new);
      lockIdentityAndDevice(accountId, parsed.deviceId());
      Instant now = clock.instant();
      var rows = jdbc.query("""
          SELECT s.account_id,i.actor_id,s.device_id,i.application_id,i.environment_id,
                 s.expires_at,d.authority_revision,k.key_id,k.public_jwk,k.key_thumbprint,
                 k.generation,k.not_before,k.not_after,k.status
          FROM sdk_sessions s JOIN sdk_identities i ON i.account_id=s.account_id
          JOIN sdk_devices d ON d.device_id=s.device_id AND d.account_id=i.account_id
          JOIN sdk_device_keys k ON k.device_id=d.device_id
          WHERE s.token_hash=:hash AND s.device_id=:device AND i.application_id=:app
            AND i.environment_id=:env AND k.key_id=:key AND i.status='active'
            AND d.revoked_at IS NULL AND k.status IN ('active','overlap') AND k.revoked_at IS NULL
            AND k.not_before<=:now AND k.not_after>:now AND s.expires_at>:now
            AND s.ownership_generation=i.ownership_generation
          FOR UPDATE OF s,k
          """, Map.of("hash", sessionHash, "device", parsed.deviceId(),
              "app", parsed.applicationId(), "env", parsed.environmentId(), "key", parsed.keyId(),
              "now", Timestamp.from(now)), (rs, row) -> new AuthorityDevice(
                  rs.getObject("account_id", UUID.class), rs.getObject("actor_id", UUID.class),
                  rs.getObject("device_id", UUID.class), rs.getObject("application_id", UUID.class),
                  rs.getObject("environment_id", UUID.class), rs.getTimestamp("expires_at").toInstant(),
                  rs.getLong("authority_revision"), rs.getObject("key_id", UUID.class),
                  rs.getString("public_jwk"), rs.getString("key_thumbprint"), rs.getLong("generation"),
                  rs.getTimestamp("not_before").toInstant(), rs.getTimestamp("not_after").toInstant(),
                  rs.getString("status")));
      if (rows.isEmpty()) throw new SdkIdentityDeniedException();
      AuthorityDevice device = rows.getFirst();
      if (!device.expiresAt().isAfter(now) || device.notBefore().isAfter(now)
          || !parsed.applicationId().equals(device.applicationId())
          || !parsed.environmentId().equals(device.environmentId())
          || !parsed.deviceId().equals(device.deviceId())) throw new SdkIdentityDeniedException();
      admitted(device.applicationId(), device.environmentId());
      ECKey publicKey = publicKey(device.publicJwk());
      if (!device.keyId().equals(parsed.keyId()) || !thumbprint(publicKey).equals(device.thumbprint())) {
        throw new SdkIdentityDeniedException();
      }
      verifyAuthoritySignature(parsed.jws(), publicKey);
      var binding = bindingAuthority.currentBinding(device.applicationId(), device.environmentId(), device.accountId(), device.deviceId())
          .orElseThrow(SdkIdentityDeniedException::new);
      if (binding.actorId() == null || binding.bindingId() == null
          || !binding.actorId().equals(device.actorId())) throw new SdkIdentityDeniedException();
      long issuedAt = clock.instant().toEpochMilli();
      long keyDeadline = device.notAfter().toEpochMilli();
      long expiresAt = Math.min(issuedAt + 4000, keyDeadline);
      if (expiresAt <= issuedAt) throw new SdkIdentityDeniedException();
      var claims = new TreeMap<String, Object>();
      claims.put("version", 1L);
      claims.put("iss", "auth");
      claims.put("aud", "voice.game-message");
      claims.put("jti", UUID.randomUUID().toString());
      claims.put("application_id", device.applicationId().toString());
      claims.put("environment_id", device.environmentId().toString());
      claims.put("account_id", device.accountId().toString());
      claims.put("actor_id", binding.actorId().toString());
      claims.put("binding_id", binding.bindingId().toString());
      claims.put("device_id", device.deviceId().toString());
      claims.put("key_id", device.keyId().toString());
      claims.put("public_jwk", publicKey.toPublicJWK().toJSONObject());
      claims.put("key_thumbprint", device.thumbprint());
      claims.put("device_generation", device.generation());
      claims.put("authority_revision", device.authorityRevision());
      claims.put("status", "active");
      claims.put("not_after", keyDeadline);
      claims.put("iat", issuedAt);
      claims.put("exp", expiresAt);
      String assertion = statusIssuer.issueDeviceStatus(claims);
      jdbc.update("""
          INSERT INTO sdk_device_authority_issues(request_id,request_bytes,session_token_hash,assertion,device_id,key_id,issued_at,expires_at)
          VALUES (:id,:bytes,:sessionHash,:assertion,:device,:key,:issued,:expires)
          """, Map.of("id", parsed.requestId(), "bytes", requestBytes, "sessionHash", sessionHash,
              "assertion", assertion,
              "device", device.deviceId(), "key", device.keyId(),
              "issued", Timestamp.from(Instant.ofEpochMilli(issuedAt)),
              "expires", Timestamp.from(Instant.ofEpochMilli(expiresAt))));
      return assertion;
    });
  }

  private AuthorityProof parseAuthorityProof(String proof, byte[] rawBytes) {
    try {
      JWSObject jws = JWSObject.parse(proof);
      var header = jws.getHeader();
      if (!JWSAlgorithm.ES256.equals(header.getAlgorithm())
          || !Set.of("alg", "kid", "typ").equals(header.toJSONObject().keySet())
          || !"voice.game-device-authority-request+jws".equals(header.getType().toString())) {
        throw new SdkIdentityDeniedException();
      }
      UUID keyId = canonicalUuid(header.getKeyID());
      String payload = new String(jws.getPayload().toBytes(), StandardCharsets.UTF_8);
      Map<String, Object> claims = com.nimbusds.jose.util.JSONObjectUtils.parse(payload);
      if (!claims.keySet().equals(Set.of("version", "audience", "request_id", "application_id",
          "environment_id", "device_id", "issued_at"))
          || !Long.valueOf(1).equals(longClaim(claims.get("version")))
          || !"voice.game-message".equals(claims.get("audience"))) throw new SdkIdentityDeniedException();
      var canonical = new TreeMap<String, Object>();
      canonical.put("version", 1L);
      canonical.put("audience", "voice.game-message");
      UUID requestId = canonicalUuid((String) claims.get("request_id"));
      UUID applicationId = canonicalUuid((String) claims.get("application_id"));
      UUID environmentId = canonicalUuid((String) claims.get("environment_id"));
      UUID deviceId = canonicalUuid((String) claims.get("device_id"));
      long issuedAt = ((Number) claims.get("issued_at")).longValue();
      if (!java.math.BigDecimal.valueOf(issuedAt).equals(new java.math.BigDecimal(claims.get("issued_at").toString()))) {
        throw new SdkIdentityDeniedException();
      }
      canonical.put("request_id", requestId.toString());
      canonical.put("application_id", applicationId.toString());
      canonical.put("environment_id", environmentId.toString());
      canonical.put("device_id", deviceId.toString());
      canonical.put("issued_at", issuedAt);
      if (!canonicalJson(canonical).equals(payload) || Math.abs(clock.instant().toEpochMilli() - issuedAt) > 30_000) {
        throw new SdkIdentityDeniedException();
      }
      return new AuthorityProof(jws, keyId, requestId, applicationId, environmentId, deviceId);
    } catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }

  private static void verifyAuthoritySignature(JWSObject jws, ECKey key) {
    try { if (!jws.verify(new ECDSAVerifier(key))) throw new SdkIdentityDeniedException(); }
    catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }

  private static Long longClaim(Object value) {
    try { return value instanceof Number n ? new java.math.BigDecimal(n.toString()).longValueExact() : null; }
    catch (ArithmeticException invalid) { return null; }
  }

  private static UUID canonicalUuid(String value) {
    UUID parsed = UUID.fromString(value);
    if (!parsed.toString().equals(value)) throw new IllegalArgumentException("noncanonical uuid");
    return parsed;
  }

  private record AuthorityProof(JWSObject jws, UUID keyId, UUID requestId, UUID applicationId,
      UUID environmentId, UUID deviceId) {}
  private record AuthorityDevice(UUID accountId, UUID actorId, UUID deviceId, UUID applicationId,
      UUID environmentId, Instant expiresAt, long authorityRevision, UUID keyId, String publicJwk,
      String thumbprint, long generation, Instant notBefore, Instant notAfter, String keyStatus) {}
  private record AuthorityReceipt(byte[] requestBytes, String sessionTokenHash, String assertion) {}

  public Challenge challenge(UUID applicationId, UUID environmentId, String devicePublicJwk) {
    SdkApplication app = admitted(applicationId, environmentId);
    ECKey key = publicKey(devicePublicJwk);
    UUID id = UUID.randomUUID();
    String nonce = randomToken();
    Instant expires = clock.instant().plusSeconds(300);
    jdbc.update("""
        INSERT INTO sdk_challenges(challenge_id,application_id,environment_id,client_id,nonce,public_jwk,expires_at,purpose)
        VALUES (:id,:app,:env,:client,:nonce,:key,:expires,'enroll')
        """, Map.of("id", id, "app", applicationId, "env", environmentId, "client", app.clientId(),
            "nonce", nonce, "key", key.toJSONString(), "expires", Timestamp.from(expires)));
    return new Challenge(id, nonce, app.clientId(), expires);
  }

  /** Starts an Auth-owned key lifecycle proof. Rotation is bound to a live SDK session; recovery to one lost device. */
  public Challenge deviceKeyChallenge(String purpose, UUID applicationId, UUID environmentId,
      String sessionToken, UUID replacesDeviceId, String newPublicJwk) {
    if (!"rotate".equals(purpose) && !"recover".equals(purpose)) throw new SdkIdentityDeniedException();
    SdkApplication app = admitted(applicationId, environmentId);
    ECKey key = publicKey(newPublicJwk);
    UUID deviceId = null;
    UUID accountId = null;
    if ("rotate".equals(purpose)) {
      if (replacesDeviceId != null || sessionToken == null || !sessionToken.matches("[A-Za-z0-9_-]{43}")) {
        throw new SdkIdentityDeniedException();
      }
      var sessions = jdbc.query("""
          SELECT s.account_id,s.device_id FROM sdk_sessions s
          JOIN sdk_identities i ON i.account_id=s.account_id
          JOIN sdk_devices d ON d.device_id=s.device_id AND d.account_id=s.account_id
          JOIN sdk_device_keys k ON k.device_id=d.device_id AND k.status='active'
          WHERE s.token_hash=:hash AND i.application_id=:app AND i.environment_id=:env
            AND s.expires_at>:now AND i.status='active' AND d.revoked_at IS NULL
            AND k.not_before<=:now AND k.not_after>:now
          """, Map.of("hash", hash(sessionToken), "app", applicationId, "env", environmentId,
              "now", Timestamp.from(clock.instant())), (rs, row) -> new UUID[] {
                rs.getObject("account_id", UUID.class), rs.getObject("device_id", UUID.class) });
      if (sessions.size() != 1) throw new SdkIdentityDeniedException();
      accountId = sessions.getFirst()[0];
      deviceId = sessions.getFirst()[1];
    } else {
      if (sessionToken != null || replacesDeviceId == null) throw new SdkIdentityDeniedException();
      Long matches = jdbc.queryForObject("""
          SELECT count(*) FROM sdk_devices d JOIN sdk_identities i ON i.account_id=d.account_id
          WHERE d.device_id=:device AND i.application_id=:app AND i.environment_id=:env
            AND i.status='active' AND d.revoked_at IS NULL
          """, Map.of("device", replacesDeviceId, "app", applicationId, "env", environmentId), Long.class);
      if (matches != 1) throw new SdkIdentityDeniedException();
    }
    UUID id = UUID.randomUUID();
    String nonce = randomToken();
    Instant expires = clock.instant().plusSeconds(300);
    var values = new org.springframework.jdbc.core.namedparam.MapSqlParameterSource()
        .addValue("id", id).addValue("app", applicationId).addValue("env", environmentId)
        .addValue("client", app.clientId()).addValue("nonce", nonce).addValue("key", key.toJSONString())
        .addValue("expires", Timestamp.from(expires)).addValue("purpose", purpose)
        .addValue("device", deviceId).addValue("account", accountId)
        .addValue("replaces", replacesDeviceId);
    jdbc.update("""
        INSERT INTO sdk_challenges(challenge_id,application_id,environment_id,client_id,nonce,public_jwk,
          expires_at,purpose,device_id,account_id,replaces_device_id)
        VALUES (:id,:app,:env,:client,:nonce,:key,:expires,:purpose,:device,:account,:replaces)
        """, values);
    return new Challenge(id, nonce, app.clientId(), expires, purpose, deviceId, replacesDeviceId);
  }

  /** Rotates a current device key and stores an immutable byte-identical idempotency receipt. */
  public DeviceKeyResult rotate(UUID challengeId, String providerToken, String currentKeyProof,
      String newKeyProof, UUID requestId, byte[] requestBytes) {
    requireLifecycleRequest(requestId, requestBytes);
    DeviceKeyResult replay = existingOperation(requestId, "rotate", requestBytes);
    if (replay != null) return replay;
    Pending pending = pending(challengeId, false);
    if (!"rotate".equals(pending.purpose()) || pending.deviceId() == null || pending.accountId() == null) {
      throw new SdkIdentityDeniedException();
    }
    admitted(pending.applicationId(), pending.environmentId());
    VerifiedProviderSubject subject = verifier.verify(providerToken, pending.clientId(), pending.nonce());
    requireSameIdentity(subject, pending.applicationId(), pending.environmentId(), pending.accountId());
    var keyRows = deviceKey(pending.deviceId(), "active", false);
    if (keyRows == null) throw new SdkIdentityDeniedException();
    ECKey oldKey = publicKey(keyRows.publicJwk());
    ECKey newKey = publicKey(pending.publicJwk());
    String newThumbprint = thumbprint(newKey);
    verifyLifecycleProof(currentKeyProof, oldKey, keyRows.keyId(), lifecycleClaims("rotate_current",
        requestId, pending, pending.deviceId(), keyRows.keyId(), thumbprint(oldKey), newThumbprint), clock.instant());
    verifyLifecycleProof(newKeyProof, newKey, null, lifecycleClaims("rotate_new", requestId,
        pending, pending.deviceId(), null, newThumbprint, null), clock.instant());

    return transactions.execute(transaction -> {
      lockOperation(requestId);
      DeviceKeyResult concurrentReplay = existingOperation(requestId, "rotate", requestBytes);
      if (concurrentReplay != null) return concurrentReplay;
      Pending locked = pending(challengeId, true);
      if (!locked.equals(pending)) throw new SdkIdentityDeniedException();
      lockIdentityAndDevice(pending.accountId(), pending.deviceId());
      Instant now = clock.instant();
      admitted(pending.applicationId(), pending.environmentId());
      verifier.verify(providerToken, pending.clientId(), pending.nonce());
      requireSameIdentity(subject, pending.applicationId(), pending.environmentId(), pending.accountId());
      DeviceKeyRow current = deviceKey(pending.deviceId(), "active", true);
      if (current == null || !current.keyId().equals(keyRows.keyId())
          || !current.notAfter().isAfter(now) || current.notBefore().isAfter(now)) {
        throw new SdkIdentityDeniedException();
      }
      verifyLifecycleProof(currentKeyProof, publicKey(current.publicJwk()), current.keyId(), lifecycleClaims(
          "rotate_current", requestId, pending, pending.deviceId(), current.keyId(),
          current.thumbprint(), newThumbprint), now);
      verifyLifecycleProof(newKeyProof, newKey, null, lifecycleClaims("rotate_new", requestId,
          pending, pending.deviceId(), null, newThumbprint, null), now);
      if (jdbc.update("""
          UPDATE sdk_challenges SET consumed_at=:now
          WHERE challenge_id=:id AND consumed_at IS NULL AND expires_at>:now
          """, Map.of("now", Timestamp.from(now), "id", challengeId)) != 1) {
        throw new SdkIdentityDeniedException();
      }
      Instant overlapUntil = now.plusSeconds(600);
      jdbc.update("""
          UPDATE sdk_device_keys SET status='overlap',not_after=:until
          WHERE key_id=:id AND status='active' AND not_after>:now
          """, Map.of("until", Timestamp.from(overlapUntil), "id", current.keyId(), "now", Timestamp.from(now)));
      long generation = current.generation() + 1;
      UUID newKeyId = UUID.randomUUID();
      Instant newNotAfter = now.plus(java.time.Duration.ofDays(90));
      jdbc.update("""
          INSERT INTO sdk_device_keys(key_id,device_id,application_id,environment_id,public_jwk,
            key_thumbprint,generation,not_before,not_after,status)
          VALUES (:key,:device,:app,:env,:jwk,:thumb,:generation,:now,:notAfter,'active')
          """, Map.of("key", newKeyId, "device", pending.deviceId(), "app", pending.applicationId(),
              "env", pending.environmentId(), "jwk", pending.publicJwk(), "thumb", newThumbprint,
              "generation", generation, "now", Timestamp.from(now), "notAfter", Timestamp.from(newNotAfter)));
      jdbc.update("""
          UPDATE sdk_devices SET public_jwk=:jwk,thumbprint=:thumb,
            authority_revision=authority_revision+1 WHERE device_id=:device AND revoked_at IS NULL
          """, Map.of("jwk", pending.publicJwk(), "thumb", newThumbprint, "device", pending.deviceId()));
      long authorityRevision = jdbc.queryForObject("SELECT authority_revision FROM sdk_devices WHERE device_id=:id",
          Map.of("id", pending.deviceId()), Long.class);
      DeviceKeyResult result = new DeviceKeyResult(pending.deviceId(), newKeyId, generation, authorityRevision);
      storeOperation(requestId, "rotate", requestBytes, pending.accountId(), result, now);
      return result;
    });
  }

  /** Recovers exactly one named device using independent identity proof and a new-key signature. */
  public DeviceKeyResult recover(UUID challengeId, String providerToken, String newKeyProof,
      UUID requestId, byte[] requestBytes) {
    requireLifecycleRequest(requestId, requestBytes);
    DeviceKeyResult replay = existingOperation(requestId, "recover", requestBytes);
    if (replay != null) return replay;
    Pending pending = pending(challengeId, false);
    if (!"recover".equals(pending.purpose()) || pending.replacesDeviceId() == null) {
      throw new SdkIdentityDeniedException();
    }
    admitted(pending.applicationId(), pending.environmentId());
    VerifiedProviderSubject subject = verifier.verify(providerToken, pending.clientId(), pending.nonce());
    UUID accountId = accountForDevice(pending.replacesDeviceId(), pending.applicationId(), pending.environmentId());
    requireSameIdentity(subject, pending.applicationId(), pending.environmentId(), accountId);
    ECKey newKey = publicKey(pending.publicJwk());
    String newThumbprint = thumbprint(newKey);
    verifyLifecycleProof(newKeyProof, newKey, null, lifecycleClaims("recover_new", requestId,
        pending, null, null, newThumbprint, null), clock.instant());

    return transactions.execute(transaction -> {
      lockOperation(requestId);
      DeviceKeyResult concurrentReplay = existingOperation(requestId, "recover", requestBytes);
      if (concurrentReplay != null) return concurrentReplay;
      Pending locked = pending(challengeId, true);
      if (!locked.equals(pending)) throw new SdkIdentityDeniedException();
      lockIdentityAndDevice(accountId, pending.replacesDeviceId());
      Instant now = clock.instant();
      admitted(pending.applicationId(), pending.environmentId());
      verifier.verify(providerToken, pending.clientId(), pending.nonce());
      requireSameIdentity(subject, pending.applicationId(), pending.environmentId(), accountId);
      verifyLifecycleProof(newKeyProof, newKey, null, lifecycleClaims("recover_new", requestId,
          pending, null, null, newThumbprint, null), now);
      Long activeDevice = jdbc.queryForObject("""
          SELECT count(*) FROM sdk_devices WHERE account_id=:account AND revoked_at IS NULL
          """, Map.of("account", accountId), Long.class);
      if (activeDevice == null || activeDevice < 1 || activeDevice > 10) throw new SdkIdentityDeniedException();
      if (jdbc.update("""
          UPDATE sdk_challenges SET consumed_at=:now
          WHERE challenge_id=:id AND consumed_at IS NULL AND expires_at>:now
          """, Map.of("now", Timestamp.from(now), "id", challengeId)) != 1) {
        throw new SdkIdentityDeniedException();
      }
      jdbc.update("""
          UPDATE sdk_device_keys SET status='revoked',revoked_at=:now,not_after=:now
          WHERE device_id=:device AND status IN ('active','overlap')
          """, Map.of("now", Timestamp.from(now), "device", pending.replacesDeviceId()));
      jdbc.update("""
          UPDATE sdk_devices SET revoked_at=:now,authority_revision=authority_revision+1
          WHERE device_id=:device AND account_id=:account AND revoked_at IS NULL
          """, Map.of("now", Timestamp.from(now), "device", pending.replacesDeviceId(), "account", accountId));
      jdbc.update("DELETE FROM sdk_sessions WHERE device_id=:device", Map.of("device", pending.replacesDeviceId()));
      UUID newDeviceId = UUID.randomUUID();
      jdbc.update("""
          INSERT INTO sdk_devices(device_id,account_id,thumbprint,public_jwk)
          VALUES (:device,:account,:thumb,:jwk)
          """, Map.of("device", newDeviceId, "account", accountId, "thumb", newThumbprint,
              "jwk", pending.publicJwk()));
      UUID newKeyId = UUID.randomUUID();
      Instant notAfter = now.plus(java.time.Duration.ofDays(90));
      jdbc.update("""
          INSERT INTO sdk_device_keys(key_id,device_id,application_id,environment_id,public_jwk,
            key_thumbprint,generation,not_before,not_after,status)
          VALUES (:key,:device,:app,:env,:jwk,:thumb,1,:now,:notAfter,'active')
          """, Map.of("key", newKeyId, "device", newDeviceId, "app", pending.applicationId(),
              "env", pending.environmentId(), "jwk", pending.publicJwk(), "thumb", newThumbprint,
              "now", Timestamp.from(now), "notAfter", Timestamp.from(notAfter)));
      DeviceKeyResult result = new DeviceKeyResult(newDeviceId, newKeyId, 1, 1);
      storeOperation(requestId, "recover", requestBytes, accountId, result, now);
      return result;
    });
  }

  /** Revokes one named device immediately after fresh independent identity and current-key proof. */
  public DeviceKeyResult revokeDevice(UUID deviceId, String providerToken, String currentKeyProof,
      UUID requestId, byte[] requestBytes) {
    requireLifecycleRequest(requestId, requestBytes);
    DeviceKeyResult replay = existingOperation(requestId, "revoke", requestBytes);
    if (replay != null) return replay;
    DeviceOwner owner = deviceOwner(deviceId);
    SdkApplication app = admitted(owner.applicationId(), owner.environmentId());
    VerifiedProviderSubject subject = verifier.verifyFresh(providerToken, app.clientId());
    requireSameIdentity(subject, owner.applicationId(), owner.environmentId(), owner.accountId());
    DeviceKeyRow current = deviceKey(deviceId, "active", false);
    if (current == null || !current.notAfter().isAfter(clock.instant())) throw new SdkIdentityDeniedException();
    var expected = new TreeMap<String, Object>();
    expected.put("version", 1L);
    expected.put("purpose", "revoke");
    expected.put("audience", "voice.auth.device-key");
    expected.put("request_id", requestId.toString());
    expected.put("application_id", owner.applicationId().toString());
    expected.put("environment_id", owner.environmentId().toString());
    expected.put("device_id", deviceId.toString());
    expected.put("key_id", current.keyId().toString());
    expected.put("key_thumbprint", current.thumbprint());
    verifyLifecycleProof(currentKeyProof, publicKey(current.publicJwk()), current.keyId(), expected, clock.instant());
    if (!Objects.equals(subject.issuer(), owner.issuer()) || !Objects.equals(subject.subject(), owner.subject())) {
      throw new SdkIdentityDeniedException();
    }

    return transactions.execute(transaction -> {
      lockOperation(requestId);
      DeviceKeyResult concurrentReplay = existingOperation(requestId, "revoke", requestBytes);
      if (concurrentReplay != null) return concurrentReplay;
      lockIdentityAndDevice(owner.accountId(), deviceId);
      Instant now = clock.instant();
      admitted(owner.applicationId(), owner.environmentId());
      VerifiedProviderSubject currentSubject = verifier.verifyFresh(providerToken, app.clientId());
      if (!Objects.equals(currentSubject.issuer(), owner.issuer())
          || !Objects.equals(currentSubject.subject(), owner.subject())) throw new SdkIdentityDeniedException();
      DeviceKeyRow lockedKey = deviceKey(deviceId, "active", true);
      if (lockedKey == null || !lockedKey.keyId().equals(current.keyId())
          || !lockedKey.notAfter().isAfter(now)) throw new SdkIdentityDeniedException();
      verifyLifecycleProof(currentKeyProof, publicKey(lockedKey.publicJwk()), lockedKey.keyId(), expected, now);
      jdbc.update("""
          UPDATE sdk_device_keys SET status='revoked',revoked_at=:now,not_after=:now
          WHERE device_id=:device AND status IN ('active','overlap')
          """, Map.of("now", Timestamp.from(now), "device", deviceId));
      jdbc.update("""
          UPDATE sdk_devices SET revoked_at=:now,authority_revision=authority_revision+1
          WHERE device_id=:device AND revoked_at IS NULL
          """, Map.of("now", Timestamp.from(now), "device", deviceId));
      jdbc.update("DELETE FROM sdk_sessions WHERE device_id=:device", Map.of("device", deviceId));
      long revision = jdbc.queryForObject("SELECT authority_revision FROM sdk_devices WHERE device_id=:id",
          Map.of("id", deviceId), Long.class);
      DeviceKeyResult result = new DeviceKeyResult(deviceId, lockedKey.keyId(), lockedKey.generation(), revision);
      storeOperation(requestId, "revoke", requestBytes, owner.accountId(), result, now);
      return result;
    });
  }

  public Session exchange(UUID challengeId, String providerToken, String gameTicket, String deviceProof) {
    if (challengeId == null) throw new SdkIdentityDeniedException();
    // Verify provider/network outside the transaction so slow JWKS cannot hold DB row locks.
    Pending pending = pending(challengeId, false);
    if (!"enroll".equals(pending.purpose())) throw new SdkIdentityDeniedException();
    SdkApplication app = admitted(pending.applicationId(), pending.environmentId());
    if (!app.clientId().equals(pending.clientId())) throw new SdkIdentityDeniedException();
    VerifiedProviderSubject subject = verifier.verify(providerToken, pending.clientId(), pending.nonce());
    String gameSubject = gameSubject(gameTicket, app, pending.nonce(), subject);
    possession(pending.publicJwk(), deviceProof,
        "voice-sdk-enroll-v1\n" + challengeId + "\n" + pending.nonce());
    return transactions.execute(transaction -> {
      Pending locked = pending(challengeId, true);
      if (!locked.equals(pending) || !"enroll".equals(locked.purpose())) throw new SdkIdentityDeniedException();
      admitted(pending.applicationId(), pending.environmentId());
      Instant now = clock.instant();
      // One durable app/env lock serializes first registration and its admission cap.
      // Hash collisions only add contention; the full scoped keys still constrain all rows.
      jdbc.query("SELECT pg_advisory_xact_lock(hashtextextended(:scope,0))",
          Map.of("scope", "voice-sdk:" + app.applicationId() + "/" + app.environmentId()),
          rs -> { });
      Map<String, Object> identityKey = Map.of("app", app.applicationId(), "env", app.environmentId(),
          "issuer", subject.issuer(), "subject", subject.subject());
      var identities = jdbc.query("""
          SELECT account_id,actor_id,ownership_generation,status FROM sdk_identities
          WHERE application_id=:app AND environment_id=:env AND issuer=:issuer AND provider_subject=:subject
          FOR UPDATE
          """, identityKey, (rs, row) -> new Identity(rs.getObject("account_id", UUID.class),
              rs.getObject("actor_id", UUID.class), rs.getLong("ownership_generation"), rs.getString("status")));
      Identity identity;
      if (identities.isEmpty()) {
        Long count = jdbc.queryForObject("""
            SELECT count(*) FROM sdk_identities WHERE application_id=:app AND environment_id=:env
            """, identityKey, Long.class);
        if (count >= 1000) throw new SdkIdentityDeniedException();
        identity = new Identity(UUID.randomUUID(), UUID.randomUUID(), 1, "active");
        var values = new org.springframework.jdbc.core.namedparam.MapSqlParameterSource(identityKey)
            .addValue("account", identity.accountId()).addValue("actor", identity.actorId())
            .addValue("now", Timestamp.from(now));
        jdbc.update("""
            INSERT INTO sdk_identities(account_id,actor_id,application_id,environment_id,issuer,provider_subject,created_at)
            VALUES (:account,:actor,:app,:env,:issuer,:subject,:now)
            """, values);
      } else {
        identity = identities.getFirst();
      }
      if (!"active".equals(identity.status())) throw new SdkIdentityDeniedException();
      ECKey key = publicKey(pending.publicJwk());
      String thumbprint;
      try { thumbprint = key.computeThumbprint().toString(); }
      catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
      var deviceKey = Map.of("account", identity.accountId(), "thumb", thumbprint);
      var devices = jdbc.query("""
          SELECT device_id,revoked_at FROM sdk_devices WHERE account_id=:account AND thumbprint=:thumb FOR UPDATE
          """, deviceKey, (rs, row) -> new Device(rs.getObject("device_id", UUID.class), rs.getTimestamp("revoked_at") != null));
      UUID deviceId;
      if (devices.isEmpty()) {
        Long count = jdbc.queryForObject("""
            SELECT count(*) FROM sdk_devices WHERE account_id=:account AND revoked_at IS NULL
            """, deviceKey, Long.class);
        if (count >= 10) throw new SdkIdentityDeniedException();
        deviceId = UUID.randomUUID();
        jdbc.update("""
            INSERT INTO sdk_devices(device_id,account_id,thumbprint,public_jwk) VALUES (:id,:account,:thumb,:key)
            """, Map.of("id", deviceId, "account", identity.accountId(), "thumb", thumbprint,
                "key", pending.publicJwk()));
      } else {
        Device device = devices.getFirst();
        if (device.revoked()) throw new SdkIdentityDeniedException();
        deviceId = device.id();
      }
      Instant keyNow = clock.instant();
      var activeKeys = jdbc.query("""
          SELECT key_id,generation FROM sdk_device_keys
          WHERE device_id=:device AND status='active' AND revoked_at IS NULL
            AND not_before<=:now AND not_after>:now
          """, Map.of("device", deviceId, "now", Timestamp.from(keyNow)), (rs, row) ->
          new DeviceKey(rs.getObject("key_id", UUID.class), rs.getLong("generation")));
      DeviceKey signingKey;
      if (activeKeys.isEmpty()) {
        Long priorKeys = jdbc.queryForObject("SELECT count(*) FROM sdk_device_keys WHERE device_id=:device",
            Map.of("device", deviceId), Long.class);
        if (priorKeys != 0) throw new SdkIdentityDeniedException();
        signingKey = new DeviceKey(UUID.randomUUID(), 1);
        jdbc.update("""
            INSERT INTO sdk_device_keys(key_id,device_id,application_id,environment_id,public_jwk,
              key_thumbprint,generation,not_before,not_after,status)
            VALUES (:key,:device,:app,:env,:jwk,:thumb,:generation,:now,:notAfter,'active')
            """, Map.of("key", signingKey.id(), "device", deviceId, "app", app.applicationId(),
                "env", app.environmentId(), "jwk", pending.publicJwk(), "thumb", thumbprint,
                "generation", signingKey.generation(), "now", Timestamp.from(keyNow),
                "notAfter", Timestamp.from(keyNow.plus(java.time.Duration.ofDays(90)))));
      } else {
        if (activeKeys.size() != 1) throw new SdkIdentityDeniedException();
        signingKey = activeKeys.getFirst();
      }
      // Locks may have waited past challenge/proof expiry. Validate again after every lock,
      // immediately before the issuance writes; failure rolls back tentative identity/device rows.
      pending(challengeId, true);
      now = clock.instant();
      fresh(providerToken, now);
      fresh(gameTicket, now);
      admitted(pending.applicationId(), pending.environmentId());
      if (jdbc.update("UPDATE sdk_challenges SET consumed_at=:now WHERE challenge_id=:id AND consumed_at IS NULL AND expires_at>:now",
          Map.of("now", Timestamp.from(now), "id", challengeId)) != 1) throw new SdkIdentityDeniedException();
      String token = randomToken();
      Instant expires = now.plusSeconds(300);
      jdbc.update("""
          INSERT INTO sdk_sessions(token_hash,account_id,device_id,ownership_generation,game_subject,expires_at)
          VALUES (:hash,:account,:device,:generation,:game,:expires)
          """, Map.of("hash", hash(token), "account", identity.accountId(), "device", deviceId,
              "generation", identity.generation(), "game", gameSubject, "expires", Timestamp.from(expires)));
      return new Session(identity.accountId(), identity.actorId(), deviceId, app.applicationId(),
          app.environmentId(), gameSubject, token, expires, "sdk-account", signingKey.id(), signingKey.generation());
    });
  }

  public Session session(String token, String proof) {
    return transactions.execute(transaction -> authenticated(token, proof, "session"));
  }

  public void revoke(String token, String proof) {
    transactions.executeWithoutResult(transaction -> {
      Session current = authenticated(token, proof, "revoke");
      Instant now = clock.instant();
      jdbc.update("""
          UPDATE sdk_device_keys SET status='revoked',revoked_at=:now,not_after=:now
          WHERE device_id=:id AND status IN ('active','overlap')
          """, Map.of("now", Timestamp.from(now), "id", current.deviceId()));
      jdbc.update("UPDATE sdk_devices SET revoked_at=:now WHERE device_id=:id AND revoked_at IS NULL",
          Map.of("now", Timestamp.from(now), "id", current.deviceId()));
      jdbc.update("UPDATE sdk_devices SET authority_revision=authority_revision+1 WHERE device_id=:id",
          Map.of("id", current.deviceId()));
      jdbc.update("DELETE FROM sdk_sessions WHERE device_id=:id", Map.of("id", current.deviceId()));
    });
  }

  private Session authenticated(String token, String proof, String purpose) {
    return authenticated(token, proof, purpose, "");
  }

  /** Package-scoped consent admission participates in the caller's JDBC transaction. */
  Session authorize(String token, String proof, String requestHash) {
    return authenticated(token, proof, "authorize", "\n" + requestHash);
  }

  Session prepareConversion(String token, String proof, UUID key, UUID binding) {
    SdkConversionProofs.preparePayload("new", token, key, binding);
    return authenticated(token, proof, "conversion-new", "\n" + key + "\n" + binding);
  }

  void requireAdmitted(UUID applicationId, UUID environmentId) {
    admitted(applicationId, environmentId);
  }

  private Session authenticated(String token, String proof, String purpose, String suffix) {
    if (token == null || !token.matches("[A-Za-z0-9_-]{43}")) throw new SdkIdentityDeniedException();
    String digest = hash(token);
    // Lock identity then device in the same order as enrollment; lifecycle changes cannot race admission.
    UUID account = jdbc.query("SELECT account_id FROM sdk_sessions WHERE token_hash=:hash",
        Map.of("hash", digest), (rs, row) -> rs.getObject("account_id", UUID.class))
        .stream().findFirst().orElseThrow(SdkIdentityDeniedException::new);
    jdbc.query("SELECT account_id FROM sdk_identities WHERE account_id=:id FOR UPDATE",
        Map.of("id", account), rs -> { });
    var rows = jdbc.query("""
        SELECT s.account_id,i.actor_id,s.device_id,i.application_id,i.environment_id,
               s.game_subject,s.expires_at,d.public_jwk,k.key_id,k.generation
        FROM sdk_sessions s JOIN sdk_identities i ON i.account_id=s.account_id
        JOIN sdk_devices d ON d.device_id=s.device_id AND d.account_id=i.account_id
        JOIN sdk_device_keys k ON k.device_id=d.device_id AND k.status='active' AND k.revoked_at IS NULL
        WHERE s.token_hash=:hash AND i.status='active' AND d.revoked_at IS NULL
          AND k.not_before<=:now AND k.not_after>:now
          AND s.ownership_generation=i.ownership_generation AND s.expires_at>:now
        FOR UPDATE OF d,s
        """, Map.of("hash", digest, "now", Timestamp.from(clock.instant())), (rs, row) -> {
          Session session = new Session(rs.getObject("account_id", UUID.class), rs.getObject("actor_id", UUID.class),
              rs.getObject("device_id", UUID.class), rs.getObject("application_id", UUID.class),
              rs.getObject("environment_id", UUID.class), rs.getString("game_subject"), null,
              rs.getTimestamp("expires_at").toInstant(), "sdk-account",
              rs.getObject("key_id", UUID.class), rs.getLong("generation"));
          return new Authenticated(session, rs.getString("public_jwk"));
        });
    if (rows.isEmpty()) throw new SdkIdentityDeniedException();
    Authenticated found = rows.getFirst();
    if (!found.session().expiresAt().isAfter(clock.instant())) throw new SdkIdentityDeniedException();
    admitted(found.session().applicationId(), found.session().environmentId());
    possession(found.publicJwk(), proof, "voice-sdk-" + purpose + "-v1\n" + digest + suffix);
    return found.session();
  }

  private Pending pending(UUID id, boolean lock) {
    var rows = jdbc.query("SELECT * FROM sdk_challenges WHERE challenge_id=:id" + (lock ? " FOR UPDATE" : ""),
        Map.of("id", id), (rs, row) -> {
          if (rs.getTimestamp("consumed_at") != null
              || !rs.getTimestamp("expires_at").toInstant().isAfter(clock.instant())) throw new SdkIdentityDeniedException();
          return new Pending(rs.getObject("application_id", UUID.class), rs.getObject("environment_id", UUID.class),
              rs.getString("client_id"), rs.getString("nonce"), rs.getString("public_jwk"),
              rs.getString("purpose"), rs.getObject("device_id", UUID.class),
              rs.getObject("account_id", UUID.class), rs.getObject("replaces_device_id", UUID.class),
              rs.getObject("challenge_id", UUID.class));
        });
    return rows.stream().findFirst().orElseThrow(SdkIdentityDeniedException::new);
  }

  private SdkApplication admitted(UUID app, UUID env) {
    if (app == null || env == null) throw new SdkIdentityDeniedException();
    SdkApplication admission = applications.get(app + "/" + env);
    if (admission == null || !app.equals(admission.applicationId()) || !env.equals(admission.environmentId())) {
      throw new SdkIdentityDeniedException();
    }
    try {
      SdkAuthorizationPolicy.Policy policy = policies.resolve(app, env);
      if (policy == null || !app.equals(policy.applicationId()) || !env.equals(policy.environmentId())
          || policy.revision() <= 0 || policy.displayName() == null || policy.displayName().isBlank()
          || policy.providers() == null || !policy.providers().contains("google")) {
        throw new SdkIdentityDeniedException();
      }
    } catch (SdkIdentityDeniedException denied) {
      throw denied;
    } catch (RuntimeException unavailable) {
      throw new SdkIdentityDeniedException();
    }
    return admission;
  }

  private void requireSameIdentity(VerifiedProviderSubject subject, UUID app, UUID env, UUID accountId) {
    Long matches = jdbc.queryForObject("""
        SELECT count(*) FROM sdk_identities
        WHERE account_id=:account AND application_id=:app AND environment_id=:env
          AND issuer=:issuer AND provider_subject=:subject AND status='active'
        """, Map.of("account", accountId, "app", app, "env", env,
            "issuer", subject.issuer(), "subject", subject.subject()), Long.class);
    if (matches == null || matches != 1) throw new SdkIdentityDeniedException();
  }

  private UUID accountForDevice(UUID deviceId, UUID app, UUID env) {
    var rows = jdbc.query("""
        SELECT d.account_id FROM sdk_devices d JOIN sdk_identities i ON i.account_id=d.account_id
        WHERE d.device_id=:device AND i.application_id=:app AND i.environment_id=:env
          AND i.status='active' AND d.revoked_at IS NULL
        """, Map.of("device", deviceId, "app", app, "env", env),
        (rs, row) -> rs.getObject("account_id", UUID.class));
    return rows.stream().findFirst().orElseThrow(SdkIdentityDeniedException::new);
  }

  private DeviceOwner deviceOwner(UUID deviceId) {
    var rows = jdbc.query("""
        SELECT i.account_id,i.application_id,i.environment_id,i.issuer,i.provider_subject
        FROM sdk_devices d JOIN sdk_identities i ON i.account_id=d.account_id
        WHERE d.device_id=:device AND d.revoked_at IS NULL AND i.status='active'
        """, Map.of("device", deviceId), (rs, row) -> new DeviceOwner(
            rs.getObject("account_id", UUID.class), rs.getObject("application_id", UUID.class),
            rs.getObject("environment_id", UUID.class), rs.getString("issuer"),
            rs.getString("provider_subject")));
    return rows.stream().findFirst().orElseThrow(SdkIdentityDeniedException::new);
  }

  private DeviceKeyRow deviceKey(UUID deviceId, String status, boolean lock) {
    String sql = """
        SELECT key_id,public_jwk,key_thumbprint,generation,not_before,not_after,status,revoked_at
        FROM sdk_device_keys WHERE device_id=:device AND status=:status AND revoked_at IS NULL
        """ + (lock ? " FOR UPDATE" : "");
    var rows = jdbc.query(sql, Map.of("device", deviceId, "status", status), (rs, row) ->
        new DeviceKeyRow(rs.getObject("key_id", UUID.class), rs.getString("public_jwk"),
            rs.getString("key_thumbprint"), rs.getLong("generation"), rs.getTimestamp("not_before").toInstant(),
            rs.getTimestamp("not_after").toInstant()));
    if (rows.size() > 1) throw new SdkIdentityDeniedException();
    return rows.isEmpty() ? null : rows.getFirst();
  }

  private void lockIdentityAndDevice(UUID accountId, UUID deviceId) {
    jdbc.query("SELECT account_id FROM sdk_identities WHERE account_id=:id FOR UPDATE",
        Map.of("id", accountId), rs -> { });
    Integer found = jdbc.query("SELECT device_id FROM sdk_devices WHERE device_id=:id AND account_id=:account FOR UPDATE",
        Map.of("id", deviceId, "account", accountId), rs -> rs.next() ? 1 : 0);
    if (found == null || found != 1) throw new SdkIdentityDeniedException();
  }

  private void requireLifecycleRequest(UUID requestId, byte[] requestBytes) {
    if (requestId == null || requestBytes == null || requestBytes.length == 0 || requestBytes.length > 65536) {
      throw new SdkIdentityDeniedException();
    }
  }

  private void lockOperation(UUID requestId) {
    jdbc.query("SELECT pg_advisory_xact_lock(hashtextextended(:scope,0))",
        Map.of("scope", "voice-sdk-device-key-operation:" + requestId), rs -> { });
  }

  private DeviceKeyResult existingOperation(UUID requestId, String operation, byte[] requestBytes) {
    var rows = jdbc.query("""
        SELECT operation,request_bytes,device_id,key_id,generation,authority_revision
        FROM sdk_device_key_operations WHERE request_id=:id
        """, Map.of("id", requestId), (rs, row) -> new OperationReceipt(rs.getString("operation"),
            rs.getBytes("request_bytes"), new DeviceKeyResult(rs.getObject("device_id", UUID.class),
                rs.getObject("key_id", UUID.class), rs.getLong("generation"), rs.getLong("authority_revision"))));
    if (rows.isEmpty()) return null;
    OperationReceipt receipt = rows.getFirst();
    if (!operation.equals(receipt.operation()) || !Arrays.equals(requestBytes, receipt.requestBytes())) {
      throw new SdkDeviceKeyConflictException();
    }
    return receipt.result();
  }

  private void storeOperation(UUID requestId, String operation, byte[] requestBytes, UUID accountId,
      DeviceKeyResult result, Instant now) {
    jdbc.update("""
        INSERT INTO sdk_device_key_operations(request_id,operation,request_bytes,account_id,device_id,
          key_id,generation,authority_revision,created_at)
        VALUES (:request,:operation,:bytes,:account,:device,:key,:generation,:revision,:now)
        """, Map.of("request", requestId, "operation", operation, "bytes", requestBytes,
            "account", accountId, "device", result.deviceId(), "key", result.keyId(),
            "generation", result.generation(), "revision", result.authorityRevision(),
            "now", Timestamp.from(now)));
  }

  private Map<String, Object> lifecycleClaims(String purpose, UUID requestId, Pending challenge,
      UUID deviceId, UUID keyId, String keyThumbprint, String newKeyThumbprint) {
    var claims = new TreeMap<String, Object>();
    claims.put("version", 1L);
    claims.put("purpose", purpose);
    claims.put("audience", "voice.auth.device-key");
    claims.put("request_id", requestId.toString());
    claims.put("challenge_id", challengeId(challenge).toString());
    claims.put("nonce", challenge.nonce());
    claims.put("application_id", challenge.applicationId().toString());
    claims.put("environment_id", challenge.environmentId().toString());
    if ("recover_new".equals(purpose)) {
      claims.put("replaces_device_id", challenge.replacesDeviceId().toString());
    } else {
      claims.put("device_id", deviceId.toString());
    }
    if (keyId != null) claims.put("key_id", keyId.toString());
    claims.put("key_thumbprint", keyThumbprint);
    if ("rotate_current".equals(purpose)) claims.put("new_key_thumbprint", newKeyThumbprint);
    return claims;
  }

  private UUID challengeId(Pending pending) {
    // The ID is carried separately in the provider proof and set by the caller through this field.
    return pending.challengeId();
  }

  private static String canonicalJson(Map<String, Object> claims) {
    try { return com.nimbusds.jose.util.JSONObjectUtils.toJSONString(new TreeMap<>(claims)); }
    catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }

  private static void verifyLifecycleProof(String proof, ECKey key, UUID kid,
      Map<String, Object> expectedClaims, Instant now) {
    try {
      if (proof == null || proof.length() > 4096) throw new SdkIdentityDeniedException();
      JWSObject jws = JWSObject.parse(proof);
      var header = jws.getHeader();
      Set<String> expectedHeaders = kid == null ? Set.of("alg", "typ") : Set.of("alg", "kid", "typ");
      if (!JWSAlgorithm.ES256.equals(header.getAlgorithm())
          || !expectedHeaders.equals(header.toJSONObject().keySet())
          || !"voice.game-device-key-proof+jws".equals(header.getType().toString())
          || !Objects.equals(kid == null ? null : kid.toString(), header.getKeyID())) {
        throw new SdkIdentityDeniedException();
      }
      String payload = new String(jws.getPayload().toBytes(), StandardCharsets.UTF_8);
      Map<String, Object> claims = com.nimbusds.jose.util.JSONObjectUtils.parse(payload);
      if (!claims.keySet().equals(expectedWithIssuedAt(expectedClaims).keySet())) {
        throw new SdkIdentityDeniedException();
      }
      Object issue = claims.get("issued_at");
      if (!(issue instanceof Number)) throw new SdkIdentityDeniedException();
      long issuedAt = new java.math.BigDecimal(issue.toString()).longValueExact();
      var complete = new TreeMap<>(expectedClaims);
      complete.put("issued_at", issuedAt);
      if (!canonicalJson(complete).equals(payload)
          || issuedAt < now.toEpochMilli() - 30_000 || issuedAt > now.toEpochMilli() + 30_000
          || !jws.verify(new ECDSAVerifier(key))) throw new SdkIdentityDeniedException();
    } catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }

  private static Map<String, Object> expectedWithIssuedAt(Map<String, Object> expected) {
    var result = new TreeMap<>(expected);
    result.put("issued_at", 0L);
    return result;
  }

  private static String thumbprint(ECKey key) {
    try { return key.computeThumbprint().toString(); }
    catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }

  private String gameSubject(String token, SdkApplication app, String nonce, VerifiedProviderSubject independent) {
    try {
      if (token == null || token.length() > 16384) throw new SdkIdentityDeniedException();
      SignedJWT jwt = SignedJWT.parse(token);
      if (!JWSAlgorithm.RS256.equals(jwt.getHeader().getAlgorithm())
          || jwt.getHeader().getCriticalParams() != null || jwt.getHeader().getJWK() != null
          || jwt.getHeader().getJWKURL() != null || jwt.getHeader().getX509CertURL() != null
          || !jwt.verify(new RSASSAVerifier(app.gameKey()))) throw new SdkIdentityDeniedException();
      var claims = jwt.getJWTClaimsSet();
      Instant now = clock.instant();
      if (!("game:" + app.applicationId() + ":" + app.environmentId()).equals(claims.getIssuer())
          || !List.of("voice:sdk-enroll").equals(claims.getAudience())
          || !nonce.equals(claims.getStringClaim("nonce"))
          || !hash(independent.issuer() + "\n" + independent.subject()).equals(claims.getStringClaim("independent_subject_hash"))
          || claims.getSubject() == null || claims.getSubject().isBlank() || claims.getSubject().length() > 255
          || claims.getIssueTime() == null || claims.getExpirationTime() == null
          || claims.getIssueTime().toInstant().isBefore(now.minusSeconds(300))
          || claims.getIssueTime().toInstant().isAfter(now.plusSeconds(30))
          || !claims.getExpirationTime().toInstant().isAfter(now)) throw new SdkIdentityDeniedException();
      return claims.getSubject();
    } catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }

  private static ECKey publicKey(String encoded) {
    try {
      if (encoded == null || encoded.length() > 2048) throw new SdkIdentityDeniedException();
      if (!Set.of("kty", "crv", "x", "y").equals(
          com.nimbusds.jose.util.JSONObjectUtils.parse(encoded).keySet())) {
        throw new SdkIdentityDeniedException();
      }
      ECKey key = ECKey.parse(encoded);
      if (key.isPrivate() || !Curve.P_256.equals(key.getCurve())
          || (key.getAlgorithm() != null && !JWSAlgorithm.ES256.equals(key.getAlgorithm()))) {
        throw new SdkIdentityDeniedException();
      }
      key.toECPublicKey();
      return key;
    } catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }

  /** Signature and immutable bindings were already checked outside the transaction. */
  private static void fresh(String token, Instant now) {
    try {
      var claims = SignedJWT.parse(token).getJWTClaimsSet();
      if (claims.getIssueTime() == null || claims.getExpirationTime() == null
          || claims.getIssueTime().toInstant().isBefore(now.minusSeconds(300))
          || claims.getIssueTime().toInstant().isAfter(now.plusSeconds(30))
          || !claims.getExpirationTime().toInstant().isAfter(now)) throw new SdkIdentityDeniedException();
    } catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }

  static void possession(String encodedKey, String proof, String payload) {
    try {
      if (proof == null || proof.length() > 4096) throw new SdkIdentityDeniedException();
      JWSObject jws = JWSObject.parse(proof);
      if (!JWSAlgorithm.ES256.equals(jws.getHeader().getAlgorithm())
          || jws.getHeader().getCriticalParams() != null || jws.getHeader().getJWK() != null
          || jws.getHeader().getJWKURL() != null || jws.getHeader().getX509CertURL() != null
          || !MessageDigest.isEqual(payload.getBytes(StandardCharsets.UTF_8), jws.getPayload().toBytes())
          || !jws.verify(new ECDSAVerifier(publicKey(encodedKey)))) throw new SdkIdentityDeniedException();
    } catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (Exception invalid) { throw new SdkIdentityDeniedException(); }
  }

  private String randomToken() {
    byte[] bytes = new byte[32];
    random.nextBytes(bytes);
    return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
  }

  static String hash(String input) {
    try {
      return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(input.getBytes(StandardCharsets.UTF_8)));
    } catch (java.security.NoSuchAlgorithmException impossible) { throw new IllegalStateException(impossible); }
  }

  private record Pending(UUID applicationId, UUID environmentId, String clientId, String nonce, String publicJwk,
                         String purpose, UUID deviceId, UUID accountId, UUID replacesDeviceId, UUID challengeId) {}
  private record DeviceKeyRow(UUID keyId, String publicJwk, String thumbprint, long generation,
                              Instant notBefore, Instant notAfter) {}
  private record DeviceOwner(UUID accountId, UUID applicationId, UUID environmentId,
                             String issuer, String subject) {}
  private record OperationReceipt(String operation, byte[] requestBytes, DeviceKeyResult result) {}
  private record Identity(UUID accountId, UUID actorId, long generation, String status) {}
  private record Device(UUID id, boolean revoked) {}
  private record DeviceKey(UUID id, long generation) {}
  private record Authenticated(Session session, String publicJwk) {}
}
