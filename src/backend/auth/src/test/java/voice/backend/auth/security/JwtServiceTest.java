package voice.backend.auth.security;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.charset.StandardCharsets;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
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
}
