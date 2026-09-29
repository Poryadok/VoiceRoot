package voice.backend.auth.sdkidentity;

import com.nimbusds.jose.JOSEException;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.JOSEObjectType;
import com.nimbusds.jose.JWSObject;
import com.nimbusds.jose.Payload;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.jwk.JWK;
import com.nimbusds.jose.jwk.JWKSet;
import com.nimbusds.jose.jwk.ECKey;
import com.nimbusds.jose.jwk.Curve;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jose.util.JSONObjectUtils;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.interfaces.RSAPrivateCrtKey;
import java.security.interfaces.RSAPrivateKey;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.PKCS8EncodedKeySpec;
import java.security.spec.RSAPublicKeySpec;
import java.time.Clock;
import java.time.Instant;
import java.util.Base64;
import java.util.Comparator;
import java.util.Date;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.UUID;

/** Dedicated RS256 workload credentials for request-bound Auth-to-User calls. */
public final class AuthUserPrincipalIssuer implements SdkDeviceStatusIssuer {
  private static final String ELIGIBILITY_RPC = "/voice.user.v1.UserService/GetSdkProfileEligibility";
  private static final String ISSUER = "auth";
  private static final String SUBJECT = "service:auth";
  private static final String AUDIENCE = "user";
  private static final int MAX_LIFETIME_SECONDS = 30;
  private final List<RSAKey> keys;
  private final RSAKey active;
  private final Clock clock;

  public record GameBindingHandoff(UUID authorizationRequestId, UUID operationId, UUID challengeId,
      String challengeNonce, UUID applicationId, UUID environmentId, String redirectUriSha256,
      String pkceChallenge, UUID deviceKeyId, String deviceKeyThumbprint, String provider,
      String providerSubjectDigest, UUID sourceAccountId, UUID sourceActorId, UUID sourceDeviceId,
      long deviceGeneration, UUID targetAccountId, UUID targetProfileId, long profileRevision,
      long consentRevision, long policyRevision, List<String> scopes, Instant expiresAt) {}

  public record VerifiedGameBindingHandoff(UUID authorizationRequestId, UUID operationId, UUID challengeId,
      String challengeNonce, UUID applicationId, UUID environmentId, String redirectUriSha256,
      String pkceChallenge, UUID deviceKeyId, String deviceKeyThumbprint, String provider,
      String providerSubjectDigest, UUID sourceAccountId, UUID sourceActorId, UUID sourceDeviceId,
      long deviceGeneration, UUID targetAccountId, UUID targetProfileId, long profileRevision,
      long consentRevision, long policyRevision, List<String> scopes, UUID assertionJti,
      Instant issuedAt, Instant expiresAt) {}

  public record VerifiedDeviceStatus(UUID jti, UUID applicationId, UUID environmentId, UUID accountId, UUID actorId,
      UUID bindingId, UUID deviceId, UUID keyId, String publicJwk, String keyThumbprint, long deviceGeneration,
      long authorityRevision, long issuedAtMs, long expiresAtMs, long notAfterMs) {}

  private static final Set<String> HANDOFF_CLAIMS = Set.of("iss", "sub", "aud", "iat", "nbf", "exp", "jti",
      "version", "authorization_request_id", "operation_id", "challenge_id", "challenge_nonce",
      "application_id", "environment_id", "redirect_uri_sha256", "pkce_challenge", "device_key_id",
      "device_key_thumbprint", "provider", "provider_subject_digest", "source_account_id", "source_actor_id",
      "source_device_id", "device_generation", "target_account_id", "target_profile_id", "profile_revision",
      "consent_revision", "policy_revision", "scopes");
  private static final String GAME_BINDING_HANDOFF_TYP = "voice.game-binding-handoff+jwt";
  private static final String GAME_MESSAGE_EXECUTION_PERMIT_TYP = "voice.game-message-execution-permit+jwt";
  private static final Set<String> GAME_MESSAGE_EXECUTION_PERMIT_CLAIMS = Set.of("iss", "aud", "jti", "version",
      "operation", "scope", "operation_id", "request_sha256", "application_id", "environment_id", "account_id",
      "actor_id", "binding_id", "profile_id", "device_id", "key_id", "device_generation", "authority_revision",
      "gis_permit_id", "binding_revision", "assertion_jti", "iat_ms", "expires_at_ms", "exp");
  private static final String GAME_BINDING_SUBJECT_DIGEST = "hmac-sha256-v1:[A-Za-z0-9_-]{1,32}:[0-9a-f]{64}";

  private AuthUserPrincipalIssuer(List<RSAKey> keys, RSAKey active, Clock clock) {
    this.keys = keys;
    this.active = active;
    this.clock = clock;
  }

  public static AuthUserPrincipalIssuer load(Path directory, String activeKid, Clock clock) {
    try {
      if (directory == null || activeKid == null || activeKid.isBlank() || clock == null
          || !Files.isDirectory(directory)) throw new IllegalArgumentException("invalid Auth principal key configuration");
      List<Path> files;
      try (var listed = Files.list(directory)) {
        files = listed.filter(path -> path.getFileName().toString().endsWith(".pem"))
            .sorted(Comparator.comparing(path -> path.getFileName().toString())).toList();
      }
      if (files.size() != 2 || !files.stream().map(path -> path.getFileName().toString())
          .collect(java.util.stream.Collectors.toSet()).equals(java.util.Set.of("current.pem", "next.pem"))) {
        throw new IllegalArgumentException("Auth principal keyset must contain current.pem and next.pem");
      }
      List<RSAKey> loaded = files.stream().map(AuthUserPrincipalIssuer::readKey).toList();
      if (loaded.get(0).getModulus().equals(loaded.get(1).getModulus())) {
        throw new IllegalArgumentException("Auth principal keys must be distinct");
      }
      RSAKey selected = loaded.stream().filter(key -> activeKid.equals(key.getKeyID())).findFirst()
          .orElseThrow(() -> new IllegalArgumentException("active Auth principal kid is not in keyset"));
      return new AuthUserPrincipalIssuer(loaded, selected, clock);
    } catch (IOException invalid) {
      throw new IllegalArgumentException("unable to load Auth principal keyset", invalid);
    }
  }

  public String issue(String rpc, String requestId, String requestHash) {
    if (!ELIGIBILITY_RPC.equals(rpc) || requestId == null || !requestId.matches(
        "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
        || requestHash == null || !requestHash.matches("sha256:[0-9a-f]{64}")) {
      throw new IllegalArgumentException("invalid request binding");
    }
    Instant now = clock.instant();
    JWTClaimsSet claims = new JWTClaimsSet.Builder().issuer(ISSUER).subject(SUBJECT).audience(AUDIENCE)
        .claim("principal_type", "service").claim("rpc", rpc).claim("request_id", requestId)
        .claim("request_hash", requestHash).issueTime(Date.from(now)).notBeforeTime(Date.from(now))
        .expirationTime(Date.from(now.plusSeconds(MAX_LIFETIME_SECONDS))).jwtID(UUID.randomUUID().toString()).build();
    SignedJWT jwt = new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).keyID(active.getKeyID()).build(), claims);
    try {
      jwt.sign(new RSASSASigner(active.toPrivateKey()));
      return jwt.serialize();
    } catch (JOSEException failure) {
      throw new IllegalStateException("unable to sign Auth principal", failure);
    }
  }

  public String issueGameBindingHandoff(GameBindingHandoff handoff) {
    validateHandoff(handoff);
    Instant now = clock.instant();
    Instant expires = handoff.expiresAt().isBefore(now.plusSeconds(MAX_LIFETIME_SECONDS))
        ? handoff.expiresAt() : now.plusSeconds(MAX_LIFETIME_SECONDS);
    if (!expires.isAfter(now)) throw new IllegalArgumentException("game-binding handoff is expired");
    JWTClaimsSet claims = new JWTClaimsSet.Builder().issuer(ISSUER).subject(SUBJECT).audience("voice.game-binding")
        .claim("version", 1).claim("authorization_request_id", handoff.authorizationRequestId().toString())
        .claim("operation_id", handoff.operationId().toString()).claim("challenge_id", handoff.challengeId().toString())
        .claim("challenge_nonce", handoff.challengeNonce()).claim("application_id", handoff.applicationId().toString())
        .claim("environment_id", handoff.environmentId().toString())
        .claim("redirect_uri_sha256", handoff.redirectUriSha256()).claim("pkce_challenge", handoff.pkceChallenge())
        .claim("device_key_id", handoff.deviceKeyId().toString()).claim("device_key_thumbprint", handoff.deviceKeyThumbprint())
        .claim("provider", handoff.provider()).claim("provider_subject_digest", handoff.providerSubjectDigest())
        .claim("source_account_id", handoff.sourceAccountId().toString()).claim("source_actor_id", handoff.sourceActorId().toString())
        .claim("source_device_id", handoff.sourceDeviceId().toString()).claim("device_generation", handoff.deviceGeneration())
        .claim("target_account_id", handoff.targetAccountId().toString()).claim("target_profile_id", handoff.targetProfileId().toString())
        .claim("profile_revision", handoff.profileRevision()).claim("consent_revision", handoff.consentRevision())
        .claim("policy_revision", handoff.policyRevision()).claim("scopes", handoff.scopes())
        .issueTime(Date.from(now)).notBeforeTime(Date.from(now)).expirationTime(Date.from(expires))
        .jwtID(UUID.randomUUID().toString()).build();
    return sign(claims, GAME_BINDING_HANDOFF_TYP);
  }

  public VerifiedGameBindingHandoff verifyGameBindingHandoff(String compact) {
    return verifyGameBindingHandoff(compact, false);
  }

  /** Verify signature and immutable claims for a receipt lookup; freshness is required before a new claim. */
  public VerifiedGameBindingHandoff verifyGameBindingHandoffForExactReplay(String compact) {
    return verifyGameBindingHandoff(compact, true);
  }

  private VerifiedGameBindingHandoff verifyGameBindingHandoff(String compact, boolean existingReceiptLookup) {
    try {
      if (compact == null || compact.isBlank() || compact.length() > 16_384) throw new IllegalArgumentException();
      SignedJWT jwt = SignedJWT.parse(compact);
      if (!JWSAlgorithm.RS256.equals(jwt.getHeader().getAlgorithm())
          || !GAME_BINDING_HANDOFF_TYP.equals(jwt.getHeader().getType().getType())
          || jwt.getHeader().getCriticalParams() != null || jwt.getHeader().getJWK() != null
          || jwt.getHeader().getJWKURL() != null || jwt.getHeader().getX509CertURL() != null) {
        throw new IllegalArgumentException();
      }
      RSAKey key = keys.stream().filter(candidate -> candidate.getKeyID().equals(jwt.getHeader().getKeyID()))
          .findFirst().orElseThrow(IllegalArgumentException::new);
      if (!jwt.verify(new com.nimbusds.jose.crypto.RSASSAVerifier(key.toPublicJWK()))) throw new IllegalArgumentException();
      JWTClaimsSet claims = jwt.getJWTClaimsSet();
      if (!claims.getClaims().keySet().equals(HANDOFF_CLAIMS) || !ISSUER.equals(claims.getIssuer())
          || !SUBJECT.equals(claims.getSubject()) || !claims.getAudience().equals(List.of("voice.game-binding"))
          || !positiveVersion(claims.getClaim("version"))) throw new IllegalArgumentException();
      Instant issuedAt = claims.getIssueTime().toInstant();
      Instant expiresAt = claims.getExpirationTime().toInstant();
      Instant now = clock.instant();
      if (claims.getNotBeforeTime() == null || !claims.getNotBeforeTime().toInstant().equals(issuedAt)
          || issuedAt.isAfter(now.plusSeconds(2))
          || (!existingReceiptLookup && (issuedAt.isBefore(now.minusSeconds(30)) || !expiresAt.isAfter(now)))
          || !expiresAt.isAfter(issuedAt) || expiresAt.isAfter(issuedAt.plusSeconds(MAX_LIFETIME_SECONDS))) {
        throw new IllegalArgumentException();
      }
      UUID assertionJti = canonicalUuid(claims.getJWTID());
      List<String> scopes = claims.getStringListClaim("scopes");
      if (scopes == null || scopes.isEmpty() || !scopes.equals(scopes.stream().distinct().sorted().toList())
          || scopes.stream().anyMatch(scope -> !scope.matches("game\\.[a-z][a-z0-9.]{0,63}"))) {
        throw new IllegalArgumentException();
      }
      VerifiedGameBindingHandoff result = new VerifiedGameBindingHandoff(
          claimUuid(claims, "authorization_request_id"), claimUuid(claims, "operation_id"),
          claimUuid(claims, "challenge_id"), claimText(claims, "challenge_nonce"),
          claimUuid(claims, "application_id"), claimUuid(claims, "environment_id"), claimText(claims, "redirect_uri_sha256"),
          claimText(claims, "pkce_challenge"), claimUuid(claims, "device_key_id"), claimText(claims, "device_key_thumbprint"),
          claimText(claims, "provider"), claimText(claims, "provider_subject_digest"), claimUuid(claims, "source_account_id"),
          claimUuid(claims, "source_actor_id"), claimUuid(claims, "source_device_id"),
          positiveLong(claims, "device_generation"), claimUuid(claims, "target_account_id"), claimUuid(claims, "target_profile_id"),
          positiveLong(claims, "profile_revision"), positiveLong(claims, "consent_revision"),
          positiveLong(claims, "policy_revision"), List.copyOf(scopes), assertionJti, issuedAt, expiresAt);
      validateHandoff(new GameBindingHandoff(result.authorizationRequestId(), result.operationId(), result.challengeId(),
          result.challengeNonce(), result.applicationId(), result.environmentId(), result.redirectUriSha256(),
          result.pkceChallenge(), result.deviceKeyId(), result.deviceKeyThumbprint(), result.provider(),
          result.providerSubjectDigest(), result.sourceAccountId(), result.sourceActorId(), result.sourceDeviceId(),
          result.deviceGeneration(), result.targetAccountId(), result.targetProfileId(), result.profileRevision(),
          result.consentRevision(), result.policyRevision(), result.scopes(), result.expiresAt()));
      return result;
    } catch (Exception invalid) {
      throw new IllegalArgumentException("invalid game-binding handoff", invalid);
    }
  }

  private String sign(JWTClaimsSet claims, String typ) {
    SignedJWT jwt = new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).type(new JOSEObjectType(typ))
        .keyID(active.getKeyID()).build(), claims);
    try {
      jwt.sign(new RSASSASigner(active.toPrivateKey()));
      return jwt.serialize();
    } catch (JOSEException failure) {
      throw new IllegalStateException("unable to sign Auth game-binding handoff", failure);
    }
  }

  /** Signs Auth's short-lived, canonical device authority assertion. */
  @Override public String issueDeviceStatus(Map<String, Object> claims) {
    java.util.Set<String> expected = java.util.Set.of("version", "iss", "aud", "jti", "application_id",
        "environment_id", "account_id", "actor_id", "binding_id", "device_id", "key_id", "public_jwk",
        "key_thumbprint", "device_generation", "authority_revision", "status", "not_after", "iat", "exp");
    if (claims == null || !claims.keySet().equals(expected)
        || !Long.valueOf(1).equals(number(claims.get("version")))
        || !"auth".equals(claims.get("iss")) || !"voice.game-message".equals(claims.get("aud"))
        || !"active".equals(claims.get("status")) || !isUuid(claims.get("jti"))
        || !(claims.get("public_jwk") instanceof Map<?, ?>)
        || !isPositive(number(claims.get("device_generation")))
        || !isPositive(number(claims.get("authority_revision")))) {
      throw new IllegalArgumentException("invalid device status assertion claims");
    }
    for (String field : List.of("application_id", "environment_id", "account_id", "actor_id", "binding_id",
        "device_id", "key_id")) {
      if (!isUuid(claims.get(field))) throw new IllegalArgumentException("invalid device status assertion claims");
    }
    Object thumbprint = claims.get("key_thumbprint");
    if (!(thumbprint instanceof String value) || !value.matches("[A-Za-z0-9_-]{43}")) {
      throw new IllegalArgumentException("invalid device status assertion claims");
    }
    try {
      @SuppressWarnings("unchecked") Map<String, Object> jwk = (Map<String, Object>) claims.get("public_jwk");
      ECKey deviceKey = ECKey.parse(jwk);
      if (!Curve.P_256.equals(deviceKey.getCurve()) || deviceKey.isPrivate()
          || !value.equals(deviceKey.computeThumbprint().toString())
          || !jwk.keySet().equals(java.util.Set.of("kty", "crv", "x", "y"))) {
        throw new IllegalArgumentException("invalid device status assertion key");
      }
    } catch (Exception invalid) {
      if (invalid instanceof IllegalArgumentException argument) throw argument;
      throw new IllegalArgumentException("invalid device status assertion key", invalid);
    }
    Long issuedAt = number(claims.get("iat"));
    Long expiresAt = number(claims.get("exp"));
    Long notAfter = number(claims.get("not_after"));
    long now = clock.instant().toEpochMilli();
    if (issuedAt == null || expiresAt == null || notAfter == null || issuedAt > now + 250
        || issuedAt < now - 250 || expiresAt <= now + 250 || expiresAt <= issuedAt
        || expiresAt - issuedAt > 4000 || expiresAt > notAfter) {
      throw new IllegalArgumentException("invalid device status assertion lifetime");
    }
    JWSObject jwt = new JWSObject(
        new JWSHeader.Builder(JWSAlgorithm.RS256).keyID(active.getKeyID())
            .type(new JOSEObjectType("voice.game-device-status+jwt")).build(),
        new Payload(canonicalJson(claims)));
    try {
      jwt.sign(new RSASSASigner(active.toPrivateKey()));
      return jwt.serialize();
    } catch (JOSEException failure) {
      throw new IllegalStateException("unable to sign Auth device status", failure);
    }
  }

  /** Verifies a raw Auth device-status assertion against the dedicated principal keyset. */
  public VerifiedDeviceStatus verifyDeviceStatusAssertion(String compact) {
    try {
      if (compact == null || compact.isBlank() || compact.length() > 16_384) throw new IllegalArgumentException();
      JWSObject jwt = JWSObject.parse(compact);
      var header = jwt.getHeader();
      if (!JWSAlgorithm.RS256.equals(header.getAlgorithm())
          || !"voice.game-device-status+jwt".equals(header.getType().toString())
          || !header.toJSONObject().keySet().equals(Set.of("alg", "kid", "typ"))) throw new IllegalArgumentException();
      RSAKey key = keys.stream().filter(candidate -> candidate.getKeyID().equals(header.getKeyID()))
          .findFirst().orElseThrow(IllegalArgumentException::new);
      if (!jwt.verify(new com.nimbusds.jose.crypto.RSASSAVerifier(key.toPublicJWK()))) throw new IllegalArgumentException();
      String payload = new String(jwt.getPayload().toBytes(), java.nio.charset.StandardCharsets.UTF_8);
      Map<String, Object> claims = JSONObjectUtils.parse(payload);
      Set<String> expected = Set.of("version", "iss", "aud", "jti", "application_id", "environment_id", "account_id",
          "actor_id", "binding_id", "device_id", "key_id", "public_jwk", "key_thumbprint", "device_generation",
          "authority_revision", "status", "not_after", "iat", "exp");
      if (!claims.keySet().equals(expected) || !canonicalJson(claims).equals(payload)
          || !positiveVersion(claims.get("version")) || !ISSUER.equals(claims.get("iss"))
          || !"voice.game-message".equals(claims.get("aud")) || !"active".equals(claims.get("status"))) {
        throw new IllegalArgumentException();
      }
      UUID jti = canonicalUuid((String) claims.get("jti"));
      UUID app = canonicalUuid((String) claims.get("application_id"));
      UUID env = canonicalUuid((String) claims.get("environment_id"));
      UUID account = canonicalUuid((String) claims.get("account_id"));
      UUID actor = canonicalUuid((String) claims.get("actor_id"));
      UUID binding = canonicalUuid((String) claims.get("binding_id"));
      UUID device = canonicalUuid((String) claims.get("device_id"));
      UUID keyId = canonicalUuid((String) claims.get("key_id"));
      long generation = positiveLongValue(claims.get("device_generation"));
      long revision = positiveLongValue(claims.get("authority_revision"));
      long issued = positiveLongValue(claims.get("iat"));
      long expires = positiveLongValue(claims.get("exp"));
      long notAfter = positiveLongValue(claims.get("not_after"));
      Object thumbprint = claims.get("key_thumbprint");
      if (!(thumbprint instanceof String thumb) || !thumb.matches("[A-Za-z0-9_-]{43}")) throw new IllegalArgumentException();
      if (!(claims.get("public_jwk") instanceof Map<?, ?> rawJwk)) throw new IllegalArgumentException();
      @SuppressWarnings("unchecked") Map<String, Object> jwk = (Map<String, Object>) rawJwk;
      ECKey deviceKey = ECKey.parse(jwk);
      if (!Curve.P_256.equals(deviceKey.getCurve()) || deviceKey.isPrivate()
          || !thumb.equals(deviceKey.computeThumbprint().toString())
          || !jwk.keySet().equals(Set.of("kty", "crv", "x", "y"))) throw new IllegalArgumentException();
      long now = clock.instant().toEpochMilli();
      if (issued > now + 250 || issued < now - 4000 || expires <= now + 250 || expires <= issued
          || expires - issued > 4000 || expires > notAfter || notAfter <= now || generation <= 0 || revision <= 0) {
        throw new IllegalArgumentException();
      }
      return new VerifiedDeviceStatus(jti, app, env, account, actor, binding, device, keyId,
          JSONObjectUtils.toJSONString(jwk), thumb, generation, revision, issued, expires, notAfter);
    } catch (Exception invalid) {
      throw new IllegalArgumentException("invalid Auth device status assertion", invalid);
    }
  }

  /** Signs exactly the frozen Auth-owned game-message execution permit claims. */
  public String issueGameMessageExecutionPermit(Map<String, Object> claims) {
    long now = clock.instant().toEpochMilli();
    if (claims == null || !claims.keySet().equals(GAME_MESSAGE_EXECUTION_PERMIT_CLAIMS)
        || !Long.valueOf(1).equals(number(claims.get("version")))
        || !"auth".equals(claims.get("iss")) || !"voice.game-message".equals(claims.get("aud"))
        || !"message.send".equals(claims.get("operation")) || !"game.chat.send".equals(claims.get("scope"))) {
      throw new IllegalArgumentException("invalid game-message execution permit claims");
    }
    for (String field : List.of("jti", "operation_id", "application_id", "environment_id", "account_id", "actor_id",
        "binding_id", "profile_id", "device_id", "key_id", "gis_permit_id", "assertion_jti")) {
      if (!isUuid(claims.get(field))) throw new IllegalArgumentException("invalid game-message execution permit claims");
    }
    Object digest = claims.get("request_sha256");
    if (!(digest instanceof String value) || !value.matches("[0-9a-f]{64}")
        || !isPositive(number(claims.get("device_generation")))
        || !isPositive(number(claims.get("authority_revision")))
        || !isPositive(number(claims.get("binding_revision")))) {
      throw new IllegalArgumentException("invalid game-message execution permit claims");
    }
    Long issuedAt = number(claims.get("iat_ms"));
    Long expiresAt = number(claims.get("expires_at_ms"));
    Long expiration = number(claims.get("exp"));
    if (issuedAt == null || expiresAt == null || expiration == null || issuedAt > now || issuedAt < now - 250
        || expiresAt <= issuedAt || expiresAt - issuedAt > 3750
        || expiration != Math.floorDiv(expiresAt, 1000)) {
      throw new IllegalArgumentException("invalid game-message execution permit lifetime");
    }
    JWSObject jwt = new JWSObject(
        new JWSHeader.Builder(JWSAlgorithm.RS256).type(new JOSEObjectType(GAME_MESSAGE_EXECUTION_PERMIT_TYP))
            .keyID(active.getKeyID()).build(),
        new Payload(canonicalJson(claims)));
    try {
      jwt.sign(new RSASSASigner(active.toPrivateKey()));
      return jwt.serialize();
    } catch (JOSEException failure) {
      throw new IllegalStateException("unable to sign Auth game-message execution permit", failure);
    }
  }

  private static void validateHandoff(GameBindingHandoff value) {
    if (value == null || value.authorizationRequestId() == null || value.operationId() == null || value.challengeId() == null
        || value.applicationId() == null || value.environmentId() == null || value.deviceKeyId() == null
        || value.sourceAccountId() == null || value.sourceActorId() == null || value.sourceDeviceId() == null
        || value.targetAccountId() == null || value.targetProfileId() == null || value.deviceGeneration() <= 0
        || value.profileRevision() <= 0 || value.consentRevision() <= 0 || value.policyRevision() <= 0
        || value.expiresAt() == null || value.challengeNonce() == null || !value.challengeNonce().matches("[A-Za-z0-9_-]{43}")
        || value.redirectUriSha256() == null || !value.redirectUriSha256().matches("[0-9a-f]{64}")
        || value.pkceChallenge() == null || !value.pkceChallenge().matches("[A-Za-z0-9_-]{43}")
        || value.deviceKeyThumbprint() == null || !value.deviceKeyThumbprint().matches("[A-Za-z0-9_-]{43}")
        || value.provider() == null || !value.provider().matches("[a-z][a-z0-9_-]{0,31}")
        || value.providerSubjectDigest() == null || !value.providerSubjectDigest().matches(GAME_BINDING_SUBJECT_DIGEST)
        || value.scopes() == null || value.scopes().isEmpty()
        || !value.scopes().equals(value.scopes().stream().distinct().sorted().toList())
        || value.scopes().stream().anyMatch(scope -> scope == null || !scope.matches("game\\.[a-z][a-z0-9.]{0,63}"))) {
      throw new IllegalArgumentException("invalid game-binding handoff claims");
    }
  }

  private static UUID claimUuid(JWTClaimsSet claims, String name) throws Exception {
    return canonicalUuid(claims.getStringClaim(name));
  }

  private static UUID canonicalUuid(String value) {
    UUID id = UUID.fromString(value);
    if (id.equals(new UUID(0, 0)) || !id.toString().equals(value)) throw new IllegalArgumentException();
    return id;
  }

  private static String claimText(JWTClaimsSet claims, String name) throws Exception {
    String value = claims.getStringClaim(name);
    if (value == null || value.isBlank()) throw new IllegalArgumentException();
    return value;
  }

  private static long positiveLong(JWTClaimsSet claims, String name) throws Exception {
    Object value = claims.getClaim(name);
    if (!(value instanceof Number number) || number.longValue() <= 0 || number.doubleValue() != number.longValue()) {
      throw new IllegalArgumentException();
    }
    return number.longValue();
  }

  private static boolean positiveVersion(Object value) {
    return value instanceof Number number && number.longValue() == 1 && number.doubleValue() == 1;
  }

  private static long positiveLongValue(Object value) {
    if (!(value instanceof Number number) || number.longValue() <= 0
        || number.doubleValue() != number.longValue()) throw new IllegalArgumentException();
    return number.longValue();
  }

  private static Long number(Object value) {
    return value instanceof Long number ? number : null;
  }

  private static boolean isPositive(Long value) { return value != null && value > 0; }

  private static boolean isUuid(Object value) {
    if (!(value instanceof String text)) return false;
    try { return UUID.fromString(text).toString().equals(text); }
    catch (IllegalArgumentException invalid) { return false; }
  }

  private static String canonicalJson(Object value) {
    if (value instanceof Map<?, ?> map) {
      TreeMap<String, Object> sorted = new TreeMap<>();
      map.forEach((key, item) -> {
        if (!(key instanceof String name)) throw new IllegalArgumentException("invalid JCS object key");
        sorted.put(name, canonicalValue(item));
      });
      return JSONObjectUtils.toJSONString(sorted);
    }
    throw new IllegalArgumentException("device status payload must be an object");
  }

  private static Object canonicalValue(Object value) {
    if (value instanceof Map<?, ?> map) {
      TreeMap<String, Object> sorted = new TreeMap<>();
      map.forEach((key, item) -> {
        if (!(key instanceof String name)) throw new IllegalArgumentException("invalid JCS object key");
        sorted.put(name, canonicalValue(item));
      });
      return sorted;
    }
    if (value instanceof String || value instanceof Long || value instanceof Integer || value instanceof Boolean
        || value == null) return value;
    throw new IllegalArgumentException("invalid JCS value");
  }

  public String jwksJson() {
    try {
      List<JWK> publicKeys = keys.stream().map(key -> (JWK) key.toPublicJWK()).toList();
      return JSONObjectUtils.toJSONString(new JWKSet(publicKeys).toJSONObject());
    } catch (RuntimeException failure) {
      throw new IllegalStateException("unable to publish Auth principal keyset", failure);
    }
  }

  private static RSAKey readKey(Path file) {
    try {
      String filename = file.getFileName().toString();
      String kid = filename.substring(0, filename.length() - ".pem".length());
      if (!kid.matches("[A-Za-z0-9_-]{1,64}")) throw new IllegalArgumentException("invalid Auth principal key id");
      String pem = Files.readString(file).trim();
      if (!pem.startsWith("-----BEGIN PRIVATE KEY-----") || !pem.endsWith("-----END PRIVATE KEY-----")) {
        throw new IllegalArgumentException("Auth principal key must be unencrypted PKCS#8 PEM");
      }
      String encoded = pem.substring("-----BEGIN PRIVATE KEY-----".length(),
          pem.length() - "-----END PRIVATE KEY-----".length()).replaceAll("\\s", "");
      byte[] der = Base64.getDecoder().decode(encoded);
      RSAPrivateKey privateKey = (RSAPrivateKey) KeyFactory.getInstance("RSA")
          .generatePrivate(new PKCS8EncodedKeySpec(der));
      if (!(privateKey instanceof RSAPrivateCrtKey crt) || privateKey.getModulus().bitLength() < 2048) {
        throw new IllegalArgumentException("Auth principal RSA keys must be at least 2048 bits");
      }
      RSAPublicKey publicKey = (RSAPublicKey) KeyFactory.getInstance("RSA")
          .generatePublic(new RSAPublicKeySpec(crt.getModulus(), crt.getPublicExponent()));
      return new RSAKey.Builder(publicKey).privateKey(privateKey)
          .keyUse(KeyUse.SIGNATURE).algorithm(JWSAlgorithm.RS256).keyID(kid).build();
    } catch (Exception invalid) {
      if (invalid instanceof IllegalArgumentException argument) throw argument;
      throw new IllegalArgumentException("invalid Auth principal PKCS#8 RSA key", invalid);
    }
  }
}
