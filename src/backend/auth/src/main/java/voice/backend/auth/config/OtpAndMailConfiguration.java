package voice.backend.auth.config;

import org.springframework.boot.autoconfigure.condition.ConditionalOnBean;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.context.annotation.Profile;
import org.springframework.data.redis.core.StringRedisTemplate;
import voice.backend.auth.mail.MailSender;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.support.CapturingMailSender;
import voice.backend.auth.repository.OtpCodeRepository;
import voice.backend.auth.service.AccountRestoreTokenStore;
import voice.backend.auth.service.InMemoryAccountRestoreTokenStore;
import voice.backend.auth.service.InMemoryOtpThrottle;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.service.OtpThrottle;
import voice.backend.auth.service.RedisAccountRestoreTokenStore;
import voice.backend.auth.service.RedisOtpThrottle;
import voice.backend.auth.security.RefreshTokenCodec;

@Configuration
public class OtpAndMailConfiguration {
  @Bean
  @Profile("!test")
  @ConditionalOnMissingBean(MailSender.class)
  MailSender mailSender() {
    return new NoopMailSender();
  }

  @Bean
  @Profile("test")
  @Primary
  CapturingMailSender capturingMailSender() {
    return new CapturingMailSender();
  }

  @Bean
  @ConditionalOnBean(StringRedisTemplate.class)
  OtpThrottle redisOtpThrottle(StringRedisTemplate redis) {
    return new RedisOtpThrottle(redis);
  }

  @Bean
  @ConditionalOnMissingBean(OtpThrottle.class)
  OtpThrottle inMemoryOtpThrottle() {
    return new InMemoryOtpThrottle();
  }

  @Bean
  @ConditionalOnBean(StringRedisTemplate.class)
  AccountRestoreTokenStore redisAccountRestoreTokenStore(StringRedisTemplate redis) {
    return new RedisAccountRestoreTokenStore(redis);
  }

  @Bean
  @ConditionalOnMissingBean(AccountRestoreTokenStore.class)
  AccountRestoreTokenStore inMemoryAccountRestoreTokenStore() {
    return new InMemoryAccountRestoreTokenStore();
  }

  @Bean
  OtpService otpService(
      voice.backend.auth.repository.AccountRepository accounts,
      OtpCodeRepository otpCodes,
      RefreshTokenCodec refreshTokenCodec,
      MailSender mailSender,
      OtpThrottle throttle,
      java.time.Clock clock) {
    return new OtpService(accounts, otpCodes, refreshTokenCodec, mailSender, throttle, clock);
  }
}
