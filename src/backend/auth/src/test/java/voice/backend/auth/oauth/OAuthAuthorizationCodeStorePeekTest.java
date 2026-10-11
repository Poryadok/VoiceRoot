package voice.backend.auth.oauth;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import org.junit.jupiter.api.Test;

class OAuthAuthorizationCodeStorePeekTest {
  private static final Instant NOW = Instant.parse("2030-01-01T00:00:00Z");

  @Test
  void memoryPeekIsRepeatableAndFinalConsumeRemainsOneTime() {
    InMemoryOAuthAuthorizationCodeStore store =
        new InMemoryOAuthAuthorizationCodeStore(Clock.fixed(NOW, ZoneOffset.UTC));
    OAuthAuthorizationCode code = code("memory-valid", NOW.plusSeconds(60));
    store.save(code, java.time.Duration.ofMinutes(1));

    assertThat(store.peek(code.code())).contains(code);
    assertThat(store.peek(code.code())).contains(code);
    assertThat(store.consume(code.code())).contains(code);
    assertThat(store.consume(code.code())).isEmpty();
  }

  @Test
  void memoryPeekAndConsumeRejectExpiredOrMissingCodes() {
    InMemoryOAuthAuthorizationCodeStore store =
        new InMemoryOAuthAuthorizationCodeStore(Clock.fixed(NOW, ZoneOffset.UTC));
    OAuthAuthorizationCode expired = code("memory-expired", NOW);
    store.save(expired, java.time.Duration.ofMinutes(1));

    assertThat(store.peek(expired.code())).isEmpty();
    assertThat(store.consume(expired.code())).isEmpty();
    assertThat(store.peek("missing")).isEmpty();
  }

  @Test
  void codecPreservesOriginEpochAndTreatsLegacyPayloadAsUnbound() {
    OAuthAuthorizationCodeCodec codec = new OAuthAuthorizationCodeCodec();
    OAuthAuthorizationCode bound = new OAuthAuthorizationCode(
        "bound", "account", "profile", "client", "https://voice.app/callback",
        "challenge", "S256", NOW.plusSeconds(60), 12L);

    assertThat(codec.decode(codec.encode(bound))).isEqualTo(bound);

    String legacyPayload = String.join("\u001f", bound.code(), bound.accountId(),
        bound.profileId(), bound.clientId(), bound.redirectUri(), bound.codeChallenge(),
        bound.codeChallengeMethod(), bound.expiresAt().toString());
    assertThat(codec.decode(legacyPayload).originSessionEpoch()).isZero();
  }

  static OAuthAuthorizationCode code(String value, Instant expiresAt) {
    return new OAuthAuthorizationCode(
        value, "account", "profile", "client", "https://voice.app/callback", "challenge", "S256", expiresAt);
  }
}
