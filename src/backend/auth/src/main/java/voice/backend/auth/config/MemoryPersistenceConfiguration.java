package voice.backend.auth.config;

import java.time.Clock;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import voice.backend.auth.repository.BackupCodeRepository;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.E2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryAccountDeletionOperationRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryGuestConversionOperationRepository;
import voice.backend.auth.repository.InMemoryOtpCodeRepository;
import voice.backend.auth.repository.OtpCodeRepository;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.oauth.InMemoryOAuthAuthorizationCodeStore;
import voice.backend.auth.oauth.OAuthAuthorizationCodeCodec;
import voice.backend.auth.oauth.OAuthAuthorizationCodeStore;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.GuestConversionLocalPromotion;
import voice.backend.auth.service.GuestConversionPendingUserWorker;
import voice.backend.auth.service.InMemoryGuestConversionLocalPromotion;
import voice.backend.auth.service.InMemoryGuestConversionOtpAcceptance;

@Configuration
@ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "memory")
public class MemoryPersistenceConfiguration {
  @Bean
  InMemoryAccountRepository accountRepository() {
    return new InMemoryAccountRepository();
  }

  @Bean
  AccountDeletionOperationRepository accountDeletionOperationRepository() {
    return new InMemoryAccountDeletionOperationRepository();
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
  OtpCodeRepository otpCodeRepository() {
    return new InMemoryOtpCodeRepository();
  }

  @Bean
  GuestConversionOperationRepository guestConversionOperationRepository() {
    return new InMemoryGuestConversionOperationRepository();
  }

  @Bean
  GuestConversionOtpAcceptance guestConversionOtpAcceptance(
      OtpCodeRepository otpCodes, GuestConversionOperationRepository operations) {
    return new InMemoryGuestConversionOtpAcceptance(otpCodes, operations);
  }

  @Bean
  InMemoryGuestConversionLocalPromotion guestConversionLocalPromotion(
      InMemoryAccountRepository accounts, GuestConversionOperationRepository operations) {
    return new InMemoryGuestConversionLocalPromotion(accounts, operations);
  }

  @Bean
  GuestConversionPendingUserWorker guestConversionPendingUserWorker(
      GuestConversionOperationRepository operations,
      voice.backend.auth.userdb.PrimaryProfileProvisioner primaryProfiles,
      GuestConversionLocalPromotion localPromotion,
      Clock clock) {
    return new GuestConversionPendingUserWorker(operations, primaryProfiles, localPromotion, clock);
  }

  @Bean
  TokenBlacklist tokenBlacklist(Clock clock) {
    return new InMemoryTokenBlacklist(clock);
  }

  @Bean
  OAuthAuthorizationCodeStore oauthAuthorizationCodeStore(Clock clock) {
    return new InMemoryOAuthAuthorizationCodeStore(clock);
  }

  @Bean
  OAuthAuthorizationCodeCodec oauthAuthorizationCodeCodec() {
    return new OAuthAuthorizationCodeCodec();
  }
}
