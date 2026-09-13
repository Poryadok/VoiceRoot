package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.time.Duration;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.dao.DataAccessResourceFailureException;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.script.RedisScript;
import voice.backend.auth.config.AuthProperties;

class RedisOtpThrottleTest {
  @Test
  void resendUsesOneAtomicSetNxWithConfiguredCooldownAndAuthPrefix() {
    StringRedisTemplate redis = mock(StringRedisTemplate.class);
    when(redis.execute(any(RedisScript.class), anyList(), any(Object[].class))).thenReturn(1L);
    AuthProperties.Redis.Otp config = config();
    config.setPrefix("tenant:auth:otp");
    config.setSendCooldown(Duration.ofSeconds(17));

    new RedisOtpThrottle(redis, config).reserveSend("account-1");

    ArgumentCaptor<RedisScript<Long>> script = scriptCaptor();
    ArgumentCaptor<List<String>> keys = keysCaptor();
    ArgumentCaptor<Object> argument = ArgumentCaptor.forClass(Object.class);
    verify(redis).execute(script.capture(), keys.capture(), argument.capture());
    assertThat(script.getValue().getScriptAsString()).contains("'SET'", "'NX'", "'PX'");
    assertThat(keys.getValue()).containsExactly("tenant:auth:otp:send:account-1");
    assertThat(argument.getValue()).isEqualTo("17000");
  }

  @Test
  void verifyAdmissionIsAtomicSlidingWindowAndRedisFailureIsCoarse() {
    StringRedisTemplate redis = mock(StringRedisTemplate.class);
    when(redis.execute(any(RedisScript.class), anyList(), any(Object[].class))).thenReturn(1L);
    AuthProperties.Redis.Otp config = config();
    config.setVerifyWindow(Duration.ofSeconds(23));

    new RedisOtpThrottle(redis, config).admitVerify("account-1");

    ArgumentCaptor<RedisScript<Long>> script = scriptCaptor();
    ArgumentCaptor<List<String>> keys = keysCaptor();
    verify(redis).execute(script.capture(), keys.capture(), any(Object[].class));
    assertThat(script.getValue().getScriptAsString())
        .contains("'ZREMRANGEBYSCORE'", "'ZCARD'", "'ZADD'", "'PEXPIRE'", "'PTTL'");
    assertThat(keys.getValue()).containsExactly("auth:otp:verify:account-1");

    when(redis.execute(any(RedisScript.class), anyList(), any(Object[].class)))
        .thenThrow(new DataAccessResourceFailureException("redis hostname leaked"));
    assertThatThrownBy(() -> new RedisOtpThrottle(redis, config).admitVerify("account-1"))
        .isInstanceOf(AuthException.class)
        .hasMessage("auth_unavailable");
  }

  @Test
  void configurationRejectsNonPositiveDurationsAndAttemptLimits() {
    AuthProperties.Redis.Otp config = config();
    assertThatThrownBy(() -> config.setSendCooldown(Duration.ZERO)).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> config.setVerifyWindow(Duration.ZERO)).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> config.setMaxVerifyAttempts(0)).isInstanceOf(IllegalArgumentException.class);
  }

  private static AuthProperties.Redis.Otp config() { return new AuthProperties.Redis.Otp(); }

  @SuppressWarnings({"unchecked", "rawtypes"})
  private static ArgumentCaptor<RedisScript<Long>> scriptCaptor() { return (ArgumentCaptor) ArgumentCaptor.forClass(RedisScript.class); }
  @SuppressWarnings({"unchecked", "rawtypes"})
  private static ArgumentCaptor<List<String>> keysCaptor() { return (ArgumentCaptor) ArgumentCaptor.forClass(List.class); }
}
