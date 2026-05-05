package voice.backend.auth.security;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import java.nio.charset.StandardCharsets;
import java.security.KeyFactory;
import java.security.NoSuchAlgorithmException;
import java.security.interfaces.RSAPrivateCrtKey;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.InvalidKeySpecException;
import java.security.spec.PKCS8EncodedKeySpec;
import java.security.spec.RSAPublicKeySpec;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Base64;
import java.util.Date;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.TokenClaims;

class JwtServiceTest {
  private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-05-01T10:00:00Z"), ZoneOffset.UTC);

  @Test
  void issuesGatewayCompatibleClaimsWithFifteenMinuteTtlAndUniqueJti() {
    JwtService jwt = JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), CLOCK);

    String first = jwt.issue("account-1", "profile-1", List.of("user"), "free");
    String second = jwt.issue("account-1", "profile-1", List.of("user"), "free");

    TokenClaims claims = jwt.validate(first);

    assertThat(first).isNotEqualTo(second);
    assertThat(claims.userId()).isEqualTo("account-1");
    assertThat(claims.profileId()).isEqualTo("profile-1");
    assertThat(claims.roles()).containsExactly("user");
    assertThat(claims.subscriptionTier()).isEqualTo("free");
    assertThat(claims.expiresAt()).isEqualTo(Instant.parse("2026-05-01T10:15:00Z"));
    assertThat(claims.jti()).isNotBlank();
    assertThat(jwt.jwksJson()).contains("\"kid\":\"test-key\"");
  }

  @Test
  void stablePkcs8PemProducesJwksWithConfiguredKid() throws Exception {
    String pem;
    try (var in = JwtServiceTest.class.getClassLoader().getResourceAsStream("jwt-test-private.pem")) {
      assertThat(in).isNotNull();
      pem = new String(in.readAllBytes(), StandardCharsets.UTF_8);
    }
    JwtService jwt =
        JwtService.fromPkcs8PrivateKeyPem(
            "voice-auth", "voice-client", "file-key", Duration.ofMinutes(15), CLOCK, pem);
    String token = jwt.issue("account-1", "profile-1", List.of("user"), "free");
    TokenClaims claims = jwt.validate(token);
    assertThat(claims.userId()).isEqualTo("account-1");
    assertThat(jwt.jwksJson()).contains("\"kid\":\"file-key\"");
  }

  @Test
  void issueRejectsBlankProfileId() {
    JwtService jwt = JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), CLOCK);
    assertThatThrownBy(() -> jwt.issue("account-1", "  ", List.of("user"), "free"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("profile_id");
    assertThatThrownBy(() -> jwt.issue("account-1", null, List.of("user"), "free"))
        .isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void validateRejectsAccessTokenWithoutProfileIdClaim() throws Exception {
    String pem;
    try (var in = JwtServiceTest.class.getClassLoader().getResourceAsStream("jwt-test-private.pem")) {
      assertThat(in).isNotNull();
      pem = new String(in.readAllBytes(), StandardCharsets.UTF_8);
    }
    JwtService jwt =
        JwtService.fromPkcs8PrivateKeyPem(
            "voice-auth", "voice-client", "file-key", Duration.ofMinutes(15), CLOCK, pem);
    RSAPrivateCrtKey privateKey = parsePkcs8RsaPrivateCrtKey(pem);
    KeyFactory keyFactory = KeyFactory.getInstance("RSA");
    RSAPublicKey publicKey =
        (RSAPublicKey) keyFactory.generatePublic(new RSAPublicKeySpec(privateKey.getModulus(), privateKey.getPublicExponent()));
    RSAKey rsaJwk =
        new RSAKey.Builder(publicKey)
            .privateKey(privateKey)
            .keyUse(KeyUse.SIGNATURE)
            .keyID("file-key")
            .algorithm(JWSAlgorithm.RS256)
            .build();
    Instant now = Instant.now(CLOCK);
    JWTClaimsSet claims =
        new JWTClaimsSet.Builder()
            .issuer("voice-auth")
            .audience("voice-client")
            .subject("account-1")
            .claim("user_id", "account-1")
            .claim("roles", List.of("user"))
            .claim("subscription_tier", "free")
            .jwtID(UUID.randomUUID().toString())
            .issueTime(Date.from(now))
            .expirationTime(Date.from(now.plus(Duration.ofMinutes(15))))
            .build();
    SignedJWT signedJwt =
        new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).keyID("file-key").build(), claims);
    signedJwt.sign(new RSASSASigner(rsaJwk.toPrivateKey()));
    String token = signedJwt.serialize();

    assertThatThrownBy(() -> jwt.validate(token)).isInstanceOf(AuthException.class).hasMessage("invalid_token");
  }

  @Test
  void rejectsInvalidSignatureAndExpiredToken() {
    JwtService jwt = JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), CLOCK);
    String token = jwt.issue("account-1", "profile-1", List.of("user"), "free");

    assertThatThrownBy(() -> jwt.validate(token + "x"))
        .isInstanceOf(AuthException.class)
        .hasMessage("invalid_token");

    JwtService later = jwt.withClock(Clock.fixed(Instant.parse("2026-05-01T10:16:00Z"), ZoneOffset.UTC));
    assertThatThrownBy(() -> later.validate(token))
        .isInstanceOf(AuthException.class)
        .hasMessage("token_expired");
  }

  private static RSAPrivateCrtKey parsePkcs8RsaPrivateCrtKey(String pem)
      throws InvalidKeySpecException, NoSuchAlgorithmException {
    String normalized =
        pem.replace("-----BEGIN PRIVATE KEY-----", "")
            .replace("-----END PRIVATE KEY-----", "")
            .replaceAll("\\s", "");
    byte[] pkcs8 = Base64.getDecoder().decode(normalized);
    PKCS8EncodedKeySpec spec = new PKCS8EncodedKeySpec(pkcs8);
    KeyFactory keyFactory = KeyFactory.getInstance("RSA");
    var key = keyFactory.generatePrivate(spec);
    if (!(key instanceof RSAPrivateCrtKey rsaPrivateCrtKey)) {
      throw new InvalidKeySpecException("expected RSA private key with CRT parameters");
    }
    return rsaPrivateCrtKey;
  }
}
