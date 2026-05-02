package voice.backend.auth.config;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.JdbcAccountRepository;
import voice.backend.auth.repository.JdbcRefreshTokenRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.RedisTokenBlacklist;
import voice.backend.auth.security.TokenBlacklist;

@Configuration
@ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc")
public class JdbcPersistenceConfiguration {
  @Bean
  AccountRepository accountRepository(NamedParameterJdbcTemplate jdbc) {
    return new JdbcAccountRepository(jdbc);
  }

  @Bean
  RefreshTokenRepository refreshTokenRepository(NamedParameterJdbcTemplate jdbc) {
    return new JdbcRefreshTokenRepository(jdbc);
  }

  @Bean
  TokenBlacklist tokenBlacklist(StringRedisTemplate redis, AuthProperties properties) {
    return new RedisTokenBlacklist(redis, properties.getRedis().getBlacklistPrefix());
  }
}
