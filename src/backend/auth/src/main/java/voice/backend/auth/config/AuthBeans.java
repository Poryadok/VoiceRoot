package voice.backend.auth.config;

import java.time.Clock;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.BackupCodeRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.BackupCodeService;
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
      voice.backend.auth.userdb.PrimaryProfileProvisioner primaryProfileProvisioner) {
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
        primaryProfileProvisioner);
  }
}
