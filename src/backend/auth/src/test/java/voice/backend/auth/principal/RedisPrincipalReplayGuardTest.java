package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.*;
import java.util.HexFormat;
import java.util.List;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

class RedisPrincipalReplayGuardTest {
  static final Instant NOW = Instant.parse("2026-09-10T10:00:00Z");
  final StringRedisTemplate redis = mock(StringRedisTemplate.class);
  @SuppressWarnings("unchecked")
  final ValueOperations<String,String> values = mock(ValueOperations.class);
  final RedisPrincipalReplayGuard guard = new RedisPrincipalReplayGuard(redis, Clock.fixed(NOW, ZoneOffset.UTC));

  RedisPrincipalReplayGuardTest() { when(redis.opsForValue()).thenReturn(values); }

  @Test void atomicallyRecordsHashedIssuerScopedKeyWithExactRemainingLifetime() throws Exception {
    when(values.setIfAbsent(anyString(), eq("1"), any(Duration.class))).thenReturn(true);
    guard.record("gateway", "jti:with/private-data", NOW.plusMillis(12500));
    String hash = HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
        .digest("jti:with/private-data".getBytes(StandardCharsets.UTF_8)));
    verify(values).setIfAbsent("auth:principal:replay:gateway:" + hash, "1", Duration.ofMillis(12500));
    verifyNoMoreInteractions(values);
    guard.record("space", "jti:with/private-data", NOW.plusSeconds(30));
    verify(values).setIfAbsent("auth:principal:replay:space:" + hash, "1", Duration.ofSeconds(30));
  }

  @Test void duplicateNullAndRedisFailureDeny() {
    when(values.setIfAbsent(anyString(), eq("1"), any(Duration.class))).thenReturn(false);
    assertThrows(RuntimeException.class, () -> guard.record("gateway", "duplicate", NOW.plusSeconds(30)));
    when(values.setIfAbsent(anyString(), eq("1"), any(Duration.class))).thenReturn(null);
    assertThrows(RuntimeException.class, () -> guard.record("gateway", "unknown", NOW.plusSeconds(30)));
    when(values.setIfAbsent(anyString(), eq("1"), any(Duration.class)))
        .thenThrow(new IllegalStateException("Redis unavailable"));
    assertThrows(RuntimeException.class, () -> guard.record("gateway", "unavailable", NOW.plusSeconds(30)));
  }

  @Test void expiredCredentialNeverWritesRedis() {
    for (Instant expiry : List.of(NOW, NOW.minusMillis(1))) {
      assertThrows(RuntimeException.class, () -> guard.record("gateway", "expired", expiry));
    }
    verifyNoInteractions(values);
  }

  @Test void slowRedisIsDeniedWithinBoundedDeadline() {
    when(values.setIfAbsent(anyString(), eq("1"), any(Duration.class))).thenAnswer(invocation -> {
      Thread.sleep(5000);
      return true;
    });
    assertTimeoutPreemptively(Duration.ofSeconds(3), () ->
        assertThrows(RuntimeException.class, () -> guard.record("gateway", "slow", NOW.plusSeconds(30))));
  }

  @Test void concurrentSameCredentialCanBeAcceptedOnlyOnce() throws Exception {
    var stored = new AtomicBoolean();
    when(values.setIfAbsent(anyString(), eq("1"), any(Duration.class)))
        .thenAnswer(invocation -> stored.compareAndSet(false, true));
    ExecutorService executor = Executors.newFixedThreadPool(2);
    CountDownLatch start = new CountDownLatch(1);
    Callable<Boolean> attempt = () -> {
      start.await();
      try { guard.record("gateway", "same-jti", NOW.plusSeconds(30)); return true; }
      catch (RuntimeException denied) { return false; }
    };
    try {
      Future<Boolean> first = executor.submit(attempt), second = executor.submit(attempt);
      start.countDown();
      int accepted = (first.get(5, TimeUnit.SECONDS) ? 1 : 0) + (second.get(5, TimeUnit.SECONDS) ? 1 : 0);
      assertEquals(1, accepted);
      verify(values, times(2)).setIfAbsent(anyString(), eq("1"), eq(Duration.ofSeconds(30)));
    } finally { executor.shutdownNow(); }
  }
}
