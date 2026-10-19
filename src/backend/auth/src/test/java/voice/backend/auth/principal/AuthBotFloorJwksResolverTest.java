package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import java.time.Duration;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;

class AuthBotFloorJwksResolverTest {
  static final byte[] COMPLETE = AuthPrincipalJwksResolverTest.set(
      AuthPrincipalJwksResolverTest.jwk("current", AuthPrincipalJwksResolverTest.CURRENT),
      AuthPrincipalJwksResolverTest.jwk("next", AuthPrincipalJwksResolverTest.NEXT));
  final AuthPrincipalJwksResolverTest.MutableClock clock = new AuthPrincipalJwksResolverTest.MutableClock();
  final AtomicReference<byte[]> response = new AtomicReference<>(COMPLETE);
  final AtomicInteger fetches = new AtomicInteger();
  final AuthPrincipalJwksResolver resolver = new AuthPrincipalJwksResolver(issuer -> {
    fetches.incrementAndGet();
    if (response.get() == null) throw new IllegalStateException("HTTP 500 or timeout");
    return response.get();
  }, clock, Duration.ofSeconds(5), Duration.ofSeconds(10), Duration.ofSeconds(1));

  @Test void completeSuccessfulAbsenceIsTypedInvalidAndCooldownNeedsExactEvidence() {
    assertThrows(AuthPrincipalJwksResolver.UnapprovedPrincipalKeyException.class,
        () -> resolver.resolveForBotFloor("bot", "absent"));
    assertThrows(AuthPrincipalJwksResolver.UnapprovedPrincipalKeyException.class,
        () -> resolver.resolveForBotFloor("bot", "absent"));
    assertThrows(PrincipalDependencyUnavailableException.class,
        () -> resolver.resolveForBotFloor("bot", "different-unobserved-kid"));
    assertEquals(1, fetches.get());
  }

  @Test void fetchFailureAndMalformedOrIncompleteSnapshotsAreUnavailableNotAbsence() {
    for (byte[] invalid : new byte[][]{null, new byte[0], "not-json".getBytes(),
        AuthPrincipalJwksResolverTest.set(AuthPrincipalJwksResolverTest.jwk("current", AuthPrincipalJwksResolverTest.CURRENT))}) {
      response.set(invalid);
      String issuer = "fixture-" + fetches.get();
      assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolveForBotFloor(issuer, "absent"));
      assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolveForBotFloor(issuer, "absent"));
    }
    assertEquals(4, fetches.get(), "outage cooldown does not imply a successful negative snapshot");
  }

  @Test void failedRefreshAfterSuccessfulNegativeInvalidatesNegativeCooldownEvidence() {
    assertThrows(AuthPrincipalJwksResolver.UnapprovedPrincipalKeyException.class,
        () -> resolver.resolveForBotFloor("bot", "absent"));
    clock.advance(2); response.set(null);
    assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolveForBotFloor("bot", "absent"));
    assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolveForBotFloor("bot", "absent"));
    assertEquals(2, fetches.get());
  }

  @Test void hardExpiredNegativeCannotAuthorizeAnInvalidClassificationDuringOutage() {
    assertThrows(AuthPrincipalJwksResolver.UnapprovedPrincipalKeyException.class,
        () -> resolver.resolveForBotFloor("bot", "absent"));
    clock.advance(10); response.set(null);
    assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolveForBotFloor("bot", "absent"));
  }

  @Test void knownLastGoodKeyRetainsOnlyTheExistingHardExpiryBounds() {
    assertEquals(AuthPrincipalJwksResolverTest.CURRENT, resolver.resolveForBotFloor("bot", "current"));
    clock.advance(6); response.set(null);
    assertEquals(AuthPrincipalJwksResolverTest.CURRENT, resolver.resolveForBotFloor("bot", "current"));
    clock.advance(4);
    assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolveForBotFloor("bot", "current"));
  }

  @Test void oldSpaceResolverKeepsItsPriorOutcomeForTheSameCompleteAndFailedSnapshots() {
    assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolve("space", "absent"));
    clock.advance(2);
    assertThrows(AuthPrincipalJwksResolver.UnapprovedPrincipalKeyException.class,
        () -> resolver.resolveForBotFloor("space", "absent"));
    assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolve("space", "absent"));
    clock.advance(2); response.set(null);
    assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolve("space", "absent"));
    assertThrows(PrincipalDependencyUnavailableException.class, () -> resolver.resolveForBotFloor("space", "absent"));
  }
}
