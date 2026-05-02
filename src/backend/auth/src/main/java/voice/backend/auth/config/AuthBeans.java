package voice.backend.auth.config;

import java.time.Clock;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;

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
  AuthService authService(
      AccountRepository accounts,
      RefreshTokenRepository refreshTokens,
      RefreshTokenCodec refreshTokenCodec,
      BCryptPasswordHasher passwordHasher,
      JwtService jwtService,
      TokenBlacklist tokenBlacklist,
      Clock clock,
      AuthProperties properties) {
    return new AuthService(
        accounts,
        refreshTokens,
        refreshTokenCodec,
        passwordHasher,
        jwtService,
        tokenBlacklist,
        clock,
        properties.getRefresh().getTtl());
  }
}
