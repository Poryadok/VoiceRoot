package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

import java.time.Clock;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.connection.RedisConnectionFactory;
import org.springframework.data.redis.connection.RedisStandaloneConfiguration;
import org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory;
import org.springframework.data.redis.core.StringRedisTemplate;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.OtpCodeRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.GuestConversionPendingUserWorker;
import voice.backend.auth.service.InMemoryOtpThrottle;
import voice.backend.auth.service.OtpThrottle;
import voice.backend.auth.service.RedisOtpThrottle;

class OtpThrottleConfigurationTest {
  private final ApplicationContextRunner jdbcWithRedis =
      new ApplicationContextRunner()
          .withUserConfiguration(OtpAndMailConfiguration.class, OtpDependencies.class)
          .withBean(RedisConnectionFactory.class, OtpThrottleConfigurationTest::redisConnectionFactory)
          .withBean(StringRedisTemplate.class, OtpThrottleConfigurationTest::redisTemplate)
          .withPropertyValues("auth.persistence=jdbc");

  private final ApplicationContextRunner jdbcWithoutRedis =
      new ApplicationContextRunner()
          .withUserConfiguration(OtpAndMailConfiguration.class, OtpDependencies.class)
          .withPropertyValues("auth.persistence=jdbc");

  private final ApplicationContextRunner memory =
      new ApplicationContextRunner()
          .withUserConfiguration(OtpAndMailConfiguration.class, OtpDependencies.class)
          .withPropertyValues("auth.persistence=memory");

  @Test
  void jdbcRuntimeWithRedisWiresTheRedisThrottle() {
    jdbcWithRedis.run(
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(context).hasSingleBean(OtpThrottle.class);
          assertThat(context.getBean(OtpThrottle.class)).isInstanceOf(RedisOtpThrottle.class);
        });
  }

  @Test
  void jdbcRuntimeWithoutRedisDoesNotStart() {
    jdbcWithoutRedis.run(
        context -> {
          assertThat(context).hasFailed();
        });
  }

  @Test
  void explicitMemoryRuntimeWiresTheInMemoryThrottle() {
    memory.run(
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(context).hasSingleBean(OtpThrottle.class);
          assertThat(context.getBean(OtpThrottle.class)).isInstanceOf(InMemoryOtpThrottle.class);
        });
  }

  private static RedisConnectionFactory redisConnectionFactory() {
    LettuceConnectionFactory factory =
        new LettuceConnectionFactory(new RedisStandaloneConfiguration("127.0.0.1", 1));
    factory.afterPropertiesSet();
    return factory;
  }

  private static StringRedisTemplate redisTemplate() {
    StringRedisTemplate template = new StringRedisTemplate(redisConnectionFactory());
    template.afterPropertiesSet();
    return template;
  }

  @Configuration(proxyBeanMethods = false)
  static class OtpDependencies {
    @Bean
    AuthProperties authProperties() {
      return new AuthProperties();
    }

    @Bean
    AccountRepository accounts() {
      return mock(AccountRepository.class);
    }

    @Bean
    OtpCodeRepository otpCodes() {
      return mock(OtpCodeRepository.class);
    }

    @Bean
    RefreshTokenRepository refreshTokens() {
      return mock(RefreshTokenRepository.class);
    }

    @Bean
    RefreshTokenCodec refreshTokenCodec() {
      return new RefreshTokenCodec();
    }

    @Bean
    BCryptPasswordHasher passwordHasher() {
      return new BCryptPasswordHasher();
    }

    @Bean
    Clock clock() {
      return Clock.systemUTC();
    }

    @Bean
    GuestConversionOtpAcceptance guestConversionAcceptance() {
      return mock(GuestConversionOtpAcceptance.class);
    }

    @Bean
    GuestConversionPendingUserWorker pendingUserWorker() {
      return mock(GuestConversionPendingUserWorker.class);
    }
  }
}
