package voice.backend.auth.security;

import com.nimbusds.jose.JOSEException;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jose.jwk.JWKSet;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import java.security.KeyFactory;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.NoSuchAlgorithmException;
import java.security.interfaces.RSAPrivateCrtKey;
import java.security.interfaces.RSAPrivateKey;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.InvalidKeySpecException;
import java.security.spec.PKCS8EncodedKeySpec;
import java.security.spec.RSAPublicKeySpec;
import java.util.Base64;
import java.text.ParseException;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Date;
import java.util.List;
import java.util.UUID;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.TokenClaims;

public class JwtService {
  private final String issuer;
  private final String audience;
  private final String keyId;
  private final Duration accessTtl;
  private final Clock clock;
  private final RSAKey jwk;

  private JwtService(String issuer, String audience, String keyId, Duration accessTtl, Clock clock, RSAKey jwk) {
    this.issuer = issuer;
    this.audience = audience;
    this.keyId = keyId;
    this.accessTtl = accessTtl;
    this.clock = clock;
    this.jwk = jwk;
  }

  public static JwtService forTests(String issuer, String audience, String keyId, Duration accessTtl, Clock clock) {
    try {
      KeyPairGenerator generator = KeyPairGenerator.getInstance("RSA");
      generator.initialize(2048);
      KeyPair keyPair = generator.generateKeyPair();
      RSAKey jwk = new RSAKey.Builder((RSAPublicKey) keyPair.getPublic())
          .privateKey((RSAPrivateKey) keyPair.getPrivate())
          .keyUse(KeyUse.SIGNATURE)
          .keyID(keyId)
          .algorithm(JWSAlgorithm.RS256)
          .build();
      return new JwtService(issuer, audience, keyId, accessTtl, clock, jwk);
    } catch (NoSuchAlgorithmException ex) {
      throw new IllegalStateException("RSA is not available", ex);
    }
  }

  /**
   * Stable signing keys for non-test runtimes: PKCS#8 PEM ({@code -----BEGIN PRIVATE KEY-----}).
   * Public key is derived from the CRT private key material.
   */
  public static JwtService fromPkcs8PrivateKeyPem(
      String issuer, String audience, String keyId, Duration accessTtl, Clock clock, String pkcs8Pem) {
    try {
      RSAPrivateCrtKey privateKey = parsePkcs8RsaPrivateCrtKey(pkcs8Pem);
      KeyFactory keyFactory = KeyFactory.getInstance("RSA");
      RSAPublicKey publicKey =
          (RSAPublicKey) keyFactory.generatePublic(new RSAPublicKeySpec(privateKey.getModulus(), privateKey.getPublicExponent()));
      RSAKey jwk = new RSAKey.Builder(publicKey)
          .privateKey(privateKey)
          .keyUse(KeyUse.SIGNATURE)
          .keyID(keyId)
          .algorithm(JWSAlgorithm.RS256)
          .build();
      return new JwtService(issuer, audience, keyId, accessTtl, clock, jwk);
    } catch (NoSuchAlgorithmException | InvalidKeySpecException ex) {
      throw new IllegalArgumentException("invalid PKCS#8 RSA private key PEM", ex);
    }
  }

  private static RSAPrivateCrtKey parsePkcs8RsaPrivateCrtKey(String pem) throws InvalidKeySpecException, NoSuchAlgorithmException {
    if (pem == null || pem.isBlank()) {
      throw new InvalidKeySpecException("empty PEM");
    }
    String normalized =
        pem.replace("-----BEGIN PRIVATE KEY-----", "")
            .replace("-----END PRIVATE KEY-----", "")
            .replaceAll("\\s", "");
    byte[] pkcs8 = Base64.getDecoder().decode(normalized);
    PKCS8EncodedKeySpec spec = new PKCS8EncodedKeySpec(pkcs8);
    KeyFactory keyFactory = KeyFactory.getInstance("RSA");
    var key = keyFactory.generatePrivate(spec);
    if (!(key instanceof RSAPrivateCrtKey rsaPrivateCrtKey)) {
      throw new InvalidKeySpecException("expected RSA private key with CRT parameters (use openssl pkcs8 -topk8 -nocrypt)");
    }
    return rsaPrivateCrtKey;
  }

  public JwtService withClock(Clock newClock) {
    return new JwtService(issuer, audience, keyId, accessTtl, newClock, jwk);
  }

  public String issue(String accountId, String profileId, List<String> roles, String subscriptionTier) {
    if (profileId == null || profileId.isBlank()) {
      throw new IllegalArgumentException("profile_id required in access JWT");
    }
    try {
      Instant now = Instant.now(clock);
      JWTClaimsSet claims = new JWTClaimsSet.Builder()
          .issuer(issuer)
          .audience(audience)
          .subject(accountId)
          .claim("user_id", accountId)
          .claim("profile_id", profileId)
          .claim("roles", roles)
          .claim("subscription_tier", subscriptionTier)
          .jwtID(UUID.randomUUID().toString())
          .issueTime(Date.from(now))
          .expirationTime(Date.from(now.plus(accessTtl)))
          .build();
      SignedJWT jwt = new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).keyID(keyId).build(), claims);
      jwt.sign(new RSASSASigner(jwk.toPrivateKey()));
      return jwt.serialize();
    } catch (JOSEException ex) {
      throw new IllegalStateException("sign jwt", ex);
    }
  }

  public TokenClaims validate(String token) {
    try {
      SignedJWT jwt = SignedJWT.parse(token);
      if (!jwt.verify(new RSASSAVerifier(jwk.toRSAPublicKey()))) {
        throw new AuthException("invalid_token");
      }
      JWTClaimsSet claims = jwt.getJWTClaimsSet();
      if (!issuer.equals(claims.getIssuer()) || !claims.getAudience().contains(audience)) {
        throw new AuthException("invalid_token");
      }
      Instant expiresAt = claims.getExpirationTime().toInstant();
      if (!expiresAt.isAfter(Instant.now(clock))) {
        throw new AuthException("token_expired");
      }
      String userId = claims.getStringClaim("user_id");
      if (userId == null || userId.isBlank()) {
        userId = claims.getSubject();
      }
      String profileId = claims.getStringClaim("profile_id");
      if (profileId == null || profileId.isBlank()) {
        throw new AuthException("invalid_token");
      }
      return new TokenClaims(
          userId,
          profileId,
          claims.getStringListClaim("roles"),
          claims.getStringClaim("subscription_tier"),
          expiresAt,
          claims.getJWTID());
    } catch (AuthException ex) {
      throw ex;
    } catch (ParseException | JOSEException ex) {
      throw new AuthException("invalid_token");
    }
  }

  public String jwksJson() {
    return new JWKSet(jwk.toPublicJWK()).toString();
  }

  public Duration ttl(TokenClaims claims) {
    Duration ttl = Duration.between(Instant.now(clock), claims.expiresAt());
    return ttl.isNegative() ? Duration.ZERO : ttl;
  }

  public Duration accessTtl() {
    return accessTtl;
  }
}
