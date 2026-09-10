package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;
import org.springframework.data.redis.core.StringRedisTemplate;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

class AuthPrincipalConfigurationTest {
  static final String URLS = "S2S_JWKS_URLS_JSON";
  static final String VALID = "{\"gateway\":\"https://gateway.invalid/jwks\",\"space\":\"https://space.invalid/jwks\"}";
  final SessionEpochFloorStore epochs = mock(SessionEpochFloorStore.class);
  final StringRedisTemplate redis = mock(StringRedisTemplate.class);

  @Test void absentConfigurationDisablesVerifierWithoutTouchingDependencies() {
    assertFalse(AuthPrincipalConfiguration.fromEnvironment(new MockEnvironment(), epochs, redis).enabled());
    verifyNoInteractions(epochs, redis);
  }

  @Test void completeHttpsConfigurationUsesDefaultsAndDoesNotFetchUntilRequest() {
    // These deliberately unresolvable hosts make eager fetching observable as a failure.
    var environment = new MockEnvironment().withProperty(URLS, VALID);
    assertTrue(AuthPrincipalConfiguration.fromEnvironment(environment, epochs, redis).enabled());
    verifyNoInteractions(epochs, redis);
  }

  @Test void acceptsExplicitStandardDurations() {
    var environment = new MockEnvironment().withProperty(URLS, VALID)
        .withProperty("S2S_JWKS_REFRESH_AFTER", "30s")
        .withProperty("S2S_JWKS_HARD_EXPIRY", "2m")
        .withProperty("S2S_UNKNOWN_KID_COOLDOWN", "5s");
    assertTrue(AuthPrincipalConfiguration.fromEnvironment(environment, epochs, redis).enabled());
  }

  @Test void rejectsBlankMalformedIncompleteAndPlaintextJwksConfiguration() {
    for (String value : List.of("", " ", "not-json", "[]", "{}",
        "{\"gateway\":\"https://gateway.invalid/jwks\"}",
        "{\"space\":\"https://space.invalid/jwks\"}",
        "{\"gateway\":\"http://gateway.invalid/jwks\",\"space\":\"https://space.invalid/jwks\"}",
        "{\"gateway\":\"https://gateway.invalid/jwks\",\"space\":\"\"}")) {
      assertThrows(IllegalArgumentException.class, () -> AuthPrincipalConfiguration.fromEnvironment(
          new MockEnvironment().withProperty(URLS, value), epochs, redis), value);
    }
  }

  @Test void rejectsOrphanedInvalidAndNonpositiveDurations() {
    for (String name : List.of("S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN")) {
      assertThrows(IllegalArgumentException.class, () -> AuthPrincipalConfiguration.fromEnvironment(
          new MockEnvironment().withProperty(name, "5s"), epochs, redis), name);
      for (String value : List.of("", " ", "nonsense", "0s", "-1s")) {
        assertThrows(IllegalArgumentException.class, () -> AuthPrincipalConfiguration.fromEnvironment(
            new MockEnvironment().withProperty(URLS, VALID).withProperty(name, value), epochs, redis), name + "=" + value);
      }
    }
    assertThrows(IllegalArgumentException.class, () -> AuthPrincipalConfiguration.fromEnvironment(
        new MockEnvironment().withProperty(URLS, VALID).withProperty("S2S_JWKS_REFRESH_AFTER", "30s")
            .withProperty("S2S_JWKS_HARD_EXPIRY", "20s"), epochs, redis));
  }

  @Test void rejectsCompatibilitySigningAliasesEvenWhenBlank() {
    for (String alias : List.of("S2S_SIGNING_KEY_PEM", "S2S_SIGNING_KID")) {
      for (String value : List.of("", "legacy-value")) {
        assertThrows(IllegalArgumentException.class, () -> AuthPrincipalConfiguration.fromEnvironment(
            new MockEnvironment().withProperty(alias, value), epochs, redis));
        assertThrows(IllegalArgumentException.class, () -> AuthPrincipalConfiguration.fromEnvironment(
            new MockEnvironment().withProperty(URLS, VALID).withProperty(alias, value), epochs, redis));
      }
    }
  }

  @Test void memoryReplayRequiresOnlyExplicitLocalOrTestProfiles() {
    for (String[] profiles : new String[][] {{}, {"production"}, {"staging"}, {"test", "production"}}) {
      var environment = new MockEnvironment().withProperty(URLS, VALID).withProperty("auth.persistence", "memory");
      environment.setActiveProfiles(profiles);
      assertThrows(IllegalArgumentException.class,
          () -> AuthPrincipalConfiguration.fromEnvironment(environment, epochs, null));
    }
    for (String[] profiles : new String[][] {{"local"}, {"test"}, {"local", "test"}}) {
      var environment = new MockEnvironment().withProperty(URLS, VALID).withProperty("auth.persistence", "memory");
      environment.setActiveProfiles(profiles);
      assertTrue(AuthPrincipalConfiguration.fromEnvironment(environment, epochs, null).enabled());
    }
  }

  @Test void configuredPrincipalRequiresEpochStoreAndJdbcRequiresRedis() {
    var environment = new MockEnvironment().withProperty(URLS, VALID).withProperty("auth.persistence", "jdbc");
    assertThrows(IllegalArgumentException.class,
        () -> AuthPrincipalConfiguration.fromEnvironment(environment, epochs, null));
    assertThrows(IllegalArgumentException.class,
        () -> AuthPrincipalConfiguration.fromEnvironment(environment, null, redis));
    var local = new MockEnvironment().withProperty(URLS, VALID).withProperty("auth.persistence", "memory");
    local.setActiveProfiles("test");
    assertThrows(IllegalArgumentException.class,
        () -> AuthPrincipalConfiguration.fromEnvironment(local, null, null));
  }
}
