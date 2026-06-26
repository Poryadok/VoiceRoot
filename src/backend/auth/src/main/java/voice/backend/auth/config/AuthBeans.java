package voice.backend.auth.config;

import io.micrometer.core.instrument.MeterRegistry;
import java.time.Clock;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.BackupCodeRepository;
import voice.backend.auth.repository.E2EKeyBackupRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.InMemorySubscriptionTierStore;
import voice.backend.auth.service.SubscriptionTierResolver;
import voice.backend.auth.oauth.OAuth2Service;
import voice.backend.auth.oauth.OAuthAuthorizationCodeStore;
import voice.backend.auth.service.TotpService;

@Configuration
public class AuthBeans {
  @Bean
  Clock clock() {
    return Clock.systemUTC();
  }

  @Bean
  RefreshTokenCodec refreshTokenCodec() {
    return new RefreshTokenCodec();
  }

  @Bean
  BCryptPasswordHasher passwordHasher() {
    return new BCryptPasswordHasher(new BCryptPasswordEncoder(12));
  }

  @Bean
  TotpService totpService(AuthProperties properties) {
    return new TotpService(properties);
  }

  @Bean
  BackupCodeService backupCodeService(BackupCodeRepository backupCodes) {
    return new BackupCodeService(backupCodes);
  }

  @Bean
  SubscriptionTierResolver subscriptionTierResolver() {
    return new InMemorySubscriptionTierStore();
  }

  @Bean
  AuthService authService(
      AccountRepository accounts,
      RefreshTokenRepository refreshTokens,
      BackupCodeService backupCodeService,
      RefreshTokenCodec refreshTokenCodec,
      BCryptPasswordHasher passwordHasher,
      JwtService jwtService,
      TokenBlacklist tokenBlacklist,
      TotpService totpService,
      Clock clock,
      AuthProperties properties,
      voice.backend.auth.userdb.PrimaryProfileProvisioner primaryProfileProvisioner,
      voice.backend.auth.userdb.PhoneHashResolver phoneHashResolver,
      SubscriptionTierResolver subscriptionTierResolver,
      voice.backend.auth.userdb.ProfileSwitchValidator profileSwitchValidator,
      E2EKeyBackupRepository e2eKeyBackupRepository,
      AuthEventPublisher authEventPublisher,
      MeterRegistry meterRegistry) {
    return new AuthService(
        accounts,
        refreshTokens,
        refreshTokenCodec,
        passwordHasher,
        jwtService,
        tokenBlacklist,
        totpService,
        backupCodeService,
        clock,
        properties.getRefresh().getTtl(),
        primaryProfileProvisioner,
        phoneHashResolver,
        subscriptionTierResolver,
        profileSwitchValidator,
        e2eKeyBackupRepository,
        authEventPublisher,
        meterRegistry);
  }

  @Bean
  OAuth2Service oauth2Service(
      AuthProperties properties,
      AuthService authService,
      OAuthAuthorizationCodeStore codeStore,
      Clock clock) {
    return new OAuth2Service(properties, authService, codeStore, clock);
  }
}
