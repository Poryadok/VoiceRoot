package voice.backend.auth.config;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.autoconfigure.condition.ConditionalOnBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.BackupCodeRepository;
import voice.backend.auth.repository.E2EKeyBackupRepository;
import voice.backend.auth.repository.JdbcAccountRepository;
import voice.backend.auth.repository.JdbcAccountDeletionOperationRepository;
import voice.backend.auth.repository.JdbcBackupCodeRepository;
import voice.backend.auth.repository.JdbcE2EKeyBackupRepository;
import voice.backend.auth.repository.JdbcOtpCodeRepository;
import voice.backend.auth.repository.JdbcGuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.OtpCodeRepository;
import voice.backend.auth.repository.JdbcRefreshTokenRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.oauth.OAuthAuthorizationCodeCodec;
import voice.backend.auth.oauth.OAuthAuthorizationCodeStore;
import voice.backend.auth.oauth.RedisOAuthAuthorizationCodeStore;
import voice.backend.auth.security.RedisTokenBlacklist;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.GuestConversionLocalPromotion;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.GuestConversionPendingUserWorker;
import voice.backend.auth.service.TransactionalGuestConversionLocalPromotion;
import voice.backend.auth.service.TransactionalGuestConversionOtpAcceptance;
import voice.backend.auth.service.AccountDeletionOperationStarter;
import voice.backend.auth.service.TransactionalAccountDeletionOperationStarter;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

@Configuration
@ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc")
public class JdbcPersistenceConfiguration {
  @Bean
  AccountRepository accountRepository(NamedParameterJdbcTemplate jdbc) {
    return new JdbcAccountRepository(jdbc);
  }

  @Bean
  AccountDeletionOperationRepository accountDeletionOperationRepository(
      NamedParameterJdbcTemplate jdbc) {
    return new JdbcAccountDeletionOperationRepository(jdbc);
  }

  @Bean
  RefreshTokenRepository refreshTokenRepository(NamedParameterJdbcTemplate jdbc) {
    return new JdbcRefreshTokenRepository(jdbc);
  }

  @Bean
  BackupCodeRepository backupCodeRepository(NamedParameterJdbcTemplate jdbc) {
    return new JdbcBackupCodeRepository(jdbc);
  }

  @Bean
  E2EKeyBackupRepository e2eKeyBackupRepository(NamedParameterJdbcTemplate jdbc) {
    return new JdbcE2EKeyBackupRepository(jdbc);
  }

  @Bean
  OtpCodeRepository otpCodeRepository(NamedParameterJdbcTemplate jdbc) {
    return new JdbcOtpCodeRepository(jdbc);
  }

  @Bean
  GuestConversionOperationRepository guestConversionOperationRepository(
      NamedParameterJdbcTemplate jdbc) {
    return new JdbcGuestConversionOperationRepository(jdbc);
  }

  @Bean
  @ConditionalOnBean(PlatformTransactionManager.class)
  TransactionTemplate guestConversionTransactionTemplate(PlatformTransactionManager transactions) {
    return new TransactionTemplate(transactions);
  }

  @Bean
  @ConditionalOnBean(TransactionTemplate.class)
  AccountDeletionOperationStarter accountDeletionOperationStarter(
      TransactionTemplate guestConversionTransactionTemplate,
      AccountRepository accounts,
      AccountDeletionOperationRepository operations) {
    return new TransactionalAccountDeletionOperationStarter(
        guestConversionTransactionTemplate, accounts, operations);
  }

  @Bean
  @ConditionalOnBean(TransactionTemplate.class)
  GuestConversionOtpAcceptance guestConversionOtpAcceptance(
      TransactionTemplate guestConversionTransactionTemplate,
      OtpCodeRepository otpCodes,
      GuestConversionOperationRepository operations) {
    return new TransactionalGuestConversionOtpAcceptance(
        guestConversionTransactionTemplate, otpCodes, operations);
  }

  @Bean
  @ConditionalOnBean(TransactionTemplate.class)
  GuestConversionLocalPromotion guestConversionLocalPromotion(
      TransactionTemplate guestConversionTransactionTemplate,
      AccountRepository accounts,
      GuestConversionOperationRepository operations) {
    return new TransactionalGuestConversionLocalPromotion(
        guestConversionTransactionTemplate, accounts, operations);
  }

  @Bean
  @ConditionalOnBean(GuestConversionLocalPromotion.class)
  GuestConversionPendingUserWorker guestConversionPendingUserWorker(
      GuestConversionOperationRepository operations,
      PrimaryProfileProvisioner primaryProfiles,
      GuestConversionLocalPromotion localPromotion,
      java.time.Clock clock) {
    return new GuestConversionPendingUserWorker(operations, primaryProfiles, localPromotion, clock);
  }

  @Bean
  TokenBlacklist tokenBlacklist(StringRedisTemplate redis, AuthProperties properties) {
    return new RedisTokenBlacklist(redis, properties.getRedis().getBlacklistPrefix());
  }

  @Bean
  OAuthAuthorizationCodeStore oauthAuthorizationCodeStore(
      StringRedisTemplate redis, OAuthAuthorizationCodeCodec codec) {
    return new RedisOAuthAuthorizationCodeStore(redis, codec);
  }

  @Bean
  OAuthAuthorizationCodeCodec oauthAuthorizationCodeCodec() {
    return new OAuthAuthorizationCodeCodec();
  }
}
