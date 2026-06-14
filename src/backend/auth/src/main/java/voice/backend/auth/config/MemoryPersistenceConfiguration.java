package voice.backend.auth.config;

import java.time.Clock;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.BackupCodeRepository;
import voice.backend.auth.repository.E2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.TokenBlacklist;

@Configuration
@ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "memory")
public class MemoryPersistenceConfiguration {
  @Bean
  AccountRepository accountRepository() {
    return new InMemoryAccountRepository();
  }

  @Bean
  RefreshTokenRepository refreshTokenRepository() {
    return new InMemoryRefreshTokenRepository();
  }

  @Bean
  BackupCodeRepository backupCodeRepository() {
    return new InMemoryBackupCodeRepository();
  }

  @Bean
  E2EKeyBackupRepository e2eKeyBackupRepository() {
    return new InMemoryE2EKeyBackupRepository();
  }

  @Bean
  TokenBlacklist tokenBlacklist(Clock clock) {
    return new InMemoryTokenBlacklist(clock);
  }
}
