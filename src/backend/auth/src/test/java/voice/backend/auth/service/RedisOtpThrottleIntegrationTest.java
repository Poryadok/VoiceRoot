package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.connection.RedisStandaloneConfiguration;
import org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.config.AuthProperties;

@Testcontainers(disabledWithoutDocker = true)
class RedisOtpThrottleIntegrationTest {
  @Container
  static final GenericContainer<?> redis = new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);
  private static LettuceConnectionFactory firstConnection;
  private static LettuceConnectionFactory secondConnection;

  @BeforeAll static void startClients() { firstConnection = connection(); secondConnection = connection(); }
  @AfterAll static void closeClients() { if (firstConnection != null) firstConnection.destroy(); if (secondConnection != null) secondConnection.destroy(); }

  @Test
  void realRedisAllowsOneConcurrentResendReservation() throws Exception {
    RedisOtpThrottle first = throttle(template(firstConnection), Duration.ofSeconds(5));
    RedisOtpThrottle second = throttle(template(secondConnection), Duration.ofSeconds(5));
    List<Boolean> outcomes = concurrently(20, index -> { try { (index % 2 == 0 ? first : second).reserveSend("send-race"); return true; } catch (AuthException ex) { assertThat(ex.getMessage()).isEqualTo("otp_rate_limited"); return false; } });
    assertThat(outcomes).filteredOn(Boolean::booleanValue).hasSize(1);
  }

  @Test
  void realRedisAtomicallyCapsConcurrentVerificationBeforeComparison() throws Exception {
    RedisOtpThrottle first = throttle(template(firstConnection), Duration.ofSeconds(5));
    RedisOtpThrottle second = throttle(template(secondConnection), Duration.ofSeconds(5));
    List<Boolean> outcomes = concurrently(32, index -> { try { (index % 2 == 0 ? first : second).admitVerify("verify-race"); return true; } catch (AuthException ex) { assertThat(ex.getMessage()).isEqualTo("otp_rate_limited"); return false; } });
    assertThat(outcomes).filteredOn(Boolean::booleanValue).hasSize(3);
  }

  @Test
  void realRedisExpiresOnlyOldEntriesInTheSlidingVerificationWindow() throws Exception {
    Duration window = Duration.ofMillis(700);
    RedisOtpThrottle throttle = throttle(template(firstConnection), window);
    throttle.admitVerify("sliding");
    Thread.sleep(400);
    throttle.admitVerify("sliding");
    throttle.admitVerify("sliding");
    Thread.sleep(350);
    throttle.admitVerify("sliding");
    assertThatThrownBy(() -> throttle.admitVerify("sliding"))
        .isInstanceOf(AuthException.class)
        .hasMessage("otp_rate_limited");
  }

  @Test
  void realRedisCorruptOrTtlLessStateFailsClosedWithoutRedisDetail() {
    StringRedisTemplate template = template(firstConnection);
    RedisOtpThrottle throttle = throttle(template, Duration.ofSeconds(5));
    template.opsForValue().set("auth:otp:verify:corrupt", "not-a-zset");
    assertThatThrownBy(() -> throttle.admitVerify("corrupt")).isInstanceOf(AuthException.class).hasMessage("auth_unavailable");
    template.delete("auth:otp:verify:corrupt");
    template.opsForZSet().add("auth:otp:verify:no-ttl", "attempt", 1D);
    assertThatThrownBy(() -> throttle.admitVerify("no-ttl")).isInstanceOf(AuthException.class).hasMessage("auth_unavailable");
  }

  @Test
  void realRedisCorruptOrTtlLessResendStateFailsClosedWithoutRedisDetail() {
    StringRedisTemplate template = template(firstConnection);
    RedisOtpThrottle throttle = throttle(template, Duration.ofSeconds(5));
    template.opsForValue().set("auth:otp:send:corrupt", "not-a-reservation", Duration.ofSeconds(5));
    assertThatThrownBy(() -> throttle.reserveSend("corrupt"))
        .isInstanceOf(AuthException.class)
        .hasMessage("auth_unavailable");
    template.delete("auth:otp:send:corrupt");
    template.opsForValue().set("auth:otp:send:no-ttl", "1");
    assertThatThrownBy(() -> throttle.reserveSend("no-ttl"))
        .isInstanceOf(AuthException.class)
        .hasMessage("auth_unavailable");
  }

  @Test
  void realRedisValidResendMarkerWithPositiveTtlIsRateLimitedWithoutResettingTtl() {
    StringRedisTemplate template = template(firstConnection);
    RedisOtpThrottle throttle = throttle(template, Duration.ofSeconds(5));
    String key = "auth:otp:send:valid-reservation";
    template.opsForValue().set(key, "1", Duration.ofSeconds(5));
    Long ttlBefore = template.getExpire(key, TimeUnit.MILLISECONDS);
    assertThat(ttlBefore).isPositive();

    assertThatThrownBy(() -> throttle.reserveSend("valid-reservation"))
        .isInstanceOf(AuthException.class)
        .hasMessage("otp_rate_limited");

    Long ttlAfter = template.getExpire(key, TimeUnit.MILLISECONDS);
    assertThat(ttlAfter).isPositive().isLessThanOrEqualTo(ttlBefore);
  }

  private static RedisOtpThrottle throttle(StringRedisTemplate template, Duration window) {
    AuthProperties.Redis.Otp settings = new AuthProperties.Redis.Otp();
    settings.setVerifyWindow(window);
    settings.setSendCooldown(Duration.ofSeconds(5));
    return new RedisOtpThrottle(template, settings);
  }
  private static LettuceConnectionFactory connection() { LettuceConnectionFactory c = new LettuceConnectionFactory(new RedisStandaloneConfiguration(redis.getHost(), redis.getMappedPort(6379))); c.afterPropertiesSet(); return c; }
  private static StringRedisTemplate template(LettuceConnectionFactory c) { StringRedisTemplate t = new StringRedisTemplate(c); t.afterPropertiesSet(); return t; }
  private static List<Boolean> concurrently(int workers, Worker action) throws Exception {
    ExecutorService pool = Executors.newFixedThreadPool(workers); CountDownLatch start = new CountDownLatch(1); List<Future<Boolean>> futures = new ArrayList<>();
    try { for (int i = 0; i < workers; i++) { int index = i; futures.add(pool.submit(() -> { start.await(); return action.run(index); })); } start.countDown(); List<Boolean> result = new ArrayList<>(); for (Future<Boolean> future : futures) result.add(future.get()); return result; } finally { pool.shutdownNow(); }
  }
  @FunctionalInterface private interface Worker { boolean run(int index) throws Exception; }
}
