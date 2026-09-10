package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;

import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.JWSAlgorithm;
import java.nio.charset.StandardCharsets;
import java.security.interfaces.RSAPublicKey;
import java.time.*;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;

class AuthPrincipalJwksResolverTest {
  static final RSAPublicKey CURRENT = (RSAPublicKey) AuthPrincipalVerifierTest.KEY.getPublic();
  static final RSAPublicKey NEXT = (RSAPublicKey) AuthPrincipalVerifierTest.key().getPublic();
  final MutableClock clock = new MutableClock();
  final AtomicInteger calls = new AtomicInteger();
  final AtomicReference<byte[]> response = new AtomicReference<>(set(jwk("current", CURRENT), jwk("next", NEXT)));
  final AuthPrincipalJwksResolver resolver = new AuthPrincipalJwksResolver(issuer -> {
    calls.incrementAndGet();
    if (response.get() == null) throw new IllegalStateException("JWKS unavailable");
    return response.get();
  }, clock, Duration.ofSeconds(30), Duration.ofSeconds(120), Duration.ofSeconds(5));

  static String jwk(String kid, RSAPublicKey key) {
    return new RSAKey.Builder(key).keyID(kid).keyUse(KeyUse.SIGNATURE).algorithm(JWSAlgorithm.RS256).build().toJSONString();
  }

  static byte[] set(String... keys) {
    return ("{\"keys\":[" + String.join(",", keys) + "]}").getBytes(StandardCharsets.UTF_8);
  }

  @Test void loadsCurrentAndNextAndRefreshesAtThirtySeconds() {
    assertEquals(CURRENT, resolver.resolve("gateway", "current"));
    assertEquals(NEXT, resolver.resolve("gateway", "next"));
    assertEquals(1, calls.get());
    clock.advance(29);
    assertEquals(CURRENT, resolver.resolve("gateway", "current"));
    assertEquals(1, calls.get());
    response.set(set(jwk("next", NEXT), jwk("future", CURRENT)));
    clock.advance(1);
    assertEquals(NEXT, resolver.resolve("gateway", "next"));
    assertEquals(CURRENT, resolver.resolve("gateway", "future"));
    assertEquals(2, calls.get());
  }

  @Test void rejectsIncompleteDuplicateAndMalformedInitialSets() {
    for (byte[] invalid : new byte[][] {
        set(), set(jwk("current", CURRENT)),
        set(jwk("current", CURRENT), jwk("current", NEXT)),
        set(jwk("current", CURRENT), "{\"kty\":\"RSA\",\"kid\":\"next\",\"n\":\"!\",\"e\":\"AQAB\"}"),
        "not-json".getBytes(StandardCharsets.UTF_8)}) {
      var empty = new AuthPrincipalJwksResolver(issuer -> invalid, clock,
          Duration.ofSeconds(30), Duration.ofSeconds(120), Duration.ofSeconds(5));
      assertThrows(RuntimeException.class, () -> empty.resolve("gateway", "current"));
    }
  }

  @Test void invalidRefreshCannotPartiallyReplaceLastGoodSetOrExtendItsLifetime() {
    assertEquals(CURRENT, resolver.resolve("gateway", "current"));
    response.set(set(jwk("current", NEXT), "{\"kid\":\"broken\",\"kty\":\"RSA\"}"));
    clock.advance(30);
    assertEquals(CURRENT, resolver.resolve("gateway", "current"));
    assertEquals(NEXT, resolver.resolve("gateway", "next"));
    clock.advance(89);
    assertEquals(CURRENT, resolver.resolve("gateway", "current"));
    clock.advance(1);
    assertThrows(RuntimeException.class, () -> resolver.resolve("gateway", "current"));
  }

  @Test void unavailableRefreshUsesLastGoodOnlyBeforeHardExpiry() {
    assertEquals(CURRENT, resolver.resolve("gateway", "current"));
    response.set(null);
    clock.advance(119);
    assertEquals(CURRENT, resolver.resolve("gateway", "current"));
    clock.advance(1);
    assertThrows(RuntimeException.class, () -> resolver.resolve("gateway", "current"));
    response.set(set(jwk("current", CURRENT), jwk("next", NEXT)));
    clock.advance(5);
    assertEquals(CURRENT, resolver.resolve("gateway", "current"));
  }

  @Test void unknownKidRefreshCooldownIsIssuerWideAndEndsAtFiveSeconds() {
    resolver.resolve("gateway", "current");
    clock.advance(5);
    int before = calls.get();
    assertThrows(RuntimeException.class, () -> resolver.resolve("gateway", "unknown-a"));
    assertEquals(before + 1, calls.get());
    assertThrows(RuntimeException.class, () -> resolver.resolve("gateway", "unknown-b"));
    clock.advance(4);
    assertThrows(RuntimeException.class, () -> resolver.resolve("gateway", "unknown-c"));
    assertEquals(before + 1, calls.get());
    assertEquals(NEXT, resolver.resolve("space", "next"));
    assertEquals(before + 2, calls.get(), "one issuer must not throttle another");
    response.set(set(jwk("current", CURRENT), jwk("rotated", NEXT)));
    clock.advance(1);
    assertEquals(NEXT, resolver.resolve("gateway", "rotated"));
    assertEquals(before + 3, calls.get());
  }

  @Test void successfulUnknownKidRefreshStillStartsIssuerCooldown() {
    resolver.resolve("gateway", "current");
    clock.advance(5);
    response.set(set(jwk("current", CURRENT), jwk("rotated-a", NEXT)));
    assertEquals(NEXT, resolver.resolve("gateway", "rotated-a"));
    assertEquals(2, calls.get());
    response.set(set(jwk("rotated-a", NEXT), jwk("rotated-b", CURRENT)));
    assertThrows(RuntimeException.class, () -> resolver.resolve("gateway", "rotated-b"));
    assertEquals(2, calls.get(), "successful unknown-kid refresh must throttle a different unknown kid");
    clock.advance(5);
    assertEquals(CURRENT, resolver.resolve("gateway", "rotated-b"));
    assertEquals(3, calls.get());
  }

  static final class MutableClock extends Clock {
    private Instant now = AuthPrincipalVerifierTest.NOW;
    void advance(long seconds) { now = now.plusSeconds(seconds); }
    @Override public ZoneId getZone() { return ZoneOffset.UTC; }
    @Override public Clock withZone(ZoneId zone) { return this; }
    @Override public Instant instant() { return now; }
  }
}
