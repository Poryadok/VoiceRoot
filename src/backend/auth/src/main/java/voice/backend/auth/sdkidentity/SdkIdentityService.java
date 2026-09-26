package voice.backend.auth.sdkidentity;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSObject;
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
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.support.TransactionTemplate;

/** Auth-owned SDK bootstrap. Every admission reads current durable device/identity state. */
public class SdkIdentityService {
  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final GoogleOidcProofVerifier verifier;
  private final Map<String, SdkApplication> applications;
  private final Clock clock;
  private final SecureRandom random = new SecureRandom();

  public record Challenge(UUID challengeId, String nonce, String clientId, Instant expiresAt) {}
  public record Session(UUID accountId, UUID actorId, UUID deviceId, UUID applicationId,
                        UUID environmentId, String gameSubject, String accessToken,
                        Instant expiresAt, String accountType) {}

  public SdkIdentityService(NamedParameterJdbcTemplate jdbc, TransactionTemplate transactions,
                           GoogleOidcProofVerifier verifier, Map<String, SdkApplication> applications,
                           Clock clock) {
    this.jdbc = jdbc;
    this.transactions = transactions;
    this.verifier = verifier;
    this.applications = Map.copyOf(applications);
    this.clock = clock;
  }

  public Challenge challenge(UUID applicationId, UUID environmentId, String devicePublicJwk) {
    SdkApplication app = admitted(applicationId, environmentId);
    ECKey key = publicKey(devicePublicJwk);
    UUID id = UUID.randomUUID();
    String nonce = randomToken();
    Instant expires = clock.instant().plusSeconds(300);
    jdbc.update("""
        INSERT INTO sdk_challenges(challenge_id,application_id,environment_id,client_id,nonce,public_jwk,expires_at)
        VALUES (:id,:app,:env,:client,:nonce,:key,:expires)
        """, Map.of("id", id, "app", applicationId, "env", environmentId, "client", app.clientId(),
            "nonce", nonce, "key", key.toJSONString(), "expires", Timestamp.from(expires)));
    return new Challenge(id, nonce, app.clientId(), expires);
  }

  public Session exchange(UUID challengeId, String providerToken, String gameTicket, String deviceProof) {
    if (challengeId == null) throw new SdkIdentityDeniedException();
    // Verify provider/network outside the transaction so slow JWKS cannot hold DB row locks.
    Pending pending = pending(challengeId, false);
    SdkApplication app = admitted(pending.applicationId(), pending.environmentId());
    if (!app.clientId().equals(pending.clientId())) throw new SdkIdentityDeniedException();
    VerifiedProviderSubject subject = verifier.verify(providerToken, pending.clientId(), pending.nonce());
    String gameSubject = gameSubject(gameTicket, app, pending.nonce(), subject);
    possession(pending.publicJwk(), deviceProof,
        "voice-sdk-enroll-v1\n" + challengeId + "\n" + pending.nonce());
    return transactions.execute(transaction -> {
      Pending locked = pending(challengeId, true);
      if (!locked.equals(pending)) throw new SdkIdentityDeniedException();
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
      // Locks may have waited past challenge/proof expiry. Validate again after every lock,
      // immediately before the issuance writes; failure rolls back tentative identity/device rows.
      pending(challengeId, true);
      now = clock.instant();
      fresh(providerToken, now);
      fresh(gameTicket, now);
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
          app.environmentId(), gameSubject, token, expires, "sdk-account");
    });
  }

  public Session session(String token, String proof) {
    return transactions.execute(transaction -> authenticated(token, proof, "session"));
  }

  public void revoke(String token, String proof) {
    transactions.executeWithoutResult(transaction -> {
      Session current = authenticated(token, proof, "revoke");
      jdbc.update("UPDATE sdk_devices SET revoked_at=:now WHERE device_id=:id AND revoked_at IS NULL",
          Map.of("now", Timestamp.from(clock.instant()), "id", current.deviceId()));
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
               s.game_subject,s.expires_at,d.public_jwk
        FROM sdk_sessions s JOIN sdk_identities i ON i.account_id=s.account_id
        JOIN sdk_devices d ON d.device_id=s.device_id AND d.account_id=i.account_id
        WHERE s.token_hash=:hash AND i.status='active' AND d.revoked_at IS NULL
          AND s.ownership_generation=i.ownership_generation AND s.expires_at>:now
        FOR UPDATE OF d,s
        """, Map.of("hash", digest, "now", Timestamp.from(clock.instant())), (rs, row) -> {
          Session session = new Session(rs.getObject("account_id", UUID.class), rs.getObject("actor_id", UUID.class),
              rs.getObject("device_id", UUID.class), rs.getObject("application_id", UUID.class),
              rs.getObject("environment_id", UUID.class), rs.getString("game_subject"), null,
              rs.getTimestamp("expires_at").toInstant(), "sdk-account");
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
              rs.getString("client_id"), rs.getString("nonce"), rs.getString("public_jwk"));
        });
    return rows.stream().findFirst().orElseThrow(SdkIdentityDeniedException::new);
  }

  private SdkApplication admitted(UUID app, UUID env) {
    if (app == null || env == null) throw new SdkIdentityDeniedException();
    SdkApplication admission = applications.get(app + "/" + env);
    if (admission == null || !app.equals(admission.applicationId()) || !env.equals(admission.environmentId())) {
      throw new SdkIdentityDeniedException();
    }
    return admission;
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

  private record Pending(UUID applicationId, UUID environmentId, String clientId, String nonce, String publicJwk) {}
  private record Identity(UUID accountId, UUID actorId, long generation, String status) {}
  private record Device(UUID id, boolean revoked) {}
  private record Authenticated(Session session, String publicJwk) {}
}
