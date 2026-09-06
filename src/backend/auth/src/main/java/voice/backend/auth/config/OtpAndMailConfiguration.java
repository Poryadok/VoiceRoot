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
import voice.backend.auth.mail.ResendMailSender;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.OtpCodeRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.service.AccountRestoreTokenStore;
import voice.backend.auth.service.InMemoryAccountRestoreTokenStore;
import voice.backend.auth.service.InMemoryOtpThrottle;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.service.OtpThrottle;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.GuestConversionPendingUserWorker;
import voice.backend.auth.service.RedisAccountRestoreTokenStore;
import voice.backend.auth.service.RedisOtpThrottle;
import voice.backend.auth.support.CapturingMailSender;

@Configuration
public class OtpAndMailConfiguration {
  @Bean
  @Profile("!test")
  @ConditionalOnMissingBean(MailSender.class)
  MailSender mailSender(AuthProperties properties) {
    AuthProperties.Resend resend = properties.getResend();
    String apiKey = resend == null ? null : resend.getApiKey();
    if (apiKey != null && !apiKey.isBlank()) {
      String from = resend.getFrom();
      if (from == null || from.isBlank()) {
        from = "Voice <onboarding@resend.dev>";
      }
      return new ResendMailSender(apiKey, from);
    }
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
      AccountRepository accounts,
      OtpCodeRepository otpCodes,
      RefreshTokenRepository refreshTokens,
      RefreshTokenCodec refreshTokenCodec,
      BCryptPasswordHasher passwordHasher,
      MailSender mailSender,
      OtpThrottle throttle,
      java.time.Clock clock,
      GuestConversionOtpAcceptance guestConversionAcceptance,
      GuestConversionPendingUserWorker pendingUserWorker) {
    return new OtpService(
        accounts,
        otpCodes,
        refreshTokens,
        refreshTokenCodec,
        passwordHasher,
        mailSender,
        throttle,
        clock,
        guestConversionAcceptance,
        pendingUserWorker);
  }

  /** Compatibility factory for narrow configuration contract tests without a recovery worker. */
  OtpService otpService(
      AccountRepository accounts,
      OtpCodeRepository otpCodes,
      RefreshTokenRepository refreshTokens,
      RefreshTokenCodec refreshTokenCodec,
      BCryptPasswordHasher passwordHasher,
      MailSender mailSender,
      OtpThrottle throttle,
      java.time.Clock clock,
      GuestConversionOtpAcceptance guestConversionAcceptance) {
    return new OtpService(
        accounts,
        otpCodes,
        refreshTokens,
        refreshTokenCodec,
        passwordHasher,
        mailSender,
        throttle,
        clock,
        guestConversionAcceptance);
  }
}
