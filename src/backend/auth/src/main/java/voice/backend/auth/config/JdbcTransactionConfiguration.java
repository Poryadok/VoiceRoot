package voice.backend.auth.config;

import java.time.Clock;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.OtpCodeRepository;
import voice.backend.auth.service.AccountDeletionOperationStarter;
import voice.backend.auth.service.GuestConversionLocalPromotion;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.GuestConversionPendingUserWorker;
import voice.backend.auth.service.TransactionalAccountDeletionOperationStarter;
import voice.backend.auth.service.TransactionalGuestConversionLocalPromotion;
import voice.backend.auth.service.TransactionalGuestConversionOtpAcceptance;
import voice.backend.auth.service.RegistrationSessionEpochPreparer;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochIssuanceGate;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

/** JDBC-only transaction collaborators, created after all required bean definitions are registered. */
@Configuration(proxyBeanMethods = false)
@ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc")
public class JdbcTransactionConfiguration {
  @Bean
  TransactionTemplate guestConversionTransactionTemplate(PlatformTransactionManager transactions) {
    return new TransactionTemplate(transactions);
  }

  @Bean
  RegistrationSessionEpochPreparer registrationSessionEpochPreparer(
      @Qualifier("guestConversionTransactionTemplate") TransactionTemplate transactions,
      AccountRepository accounts,
      SessionEpochFloorStore floors) {
    return new RegistrationSessionEpochPreparer(
        transactions, accounts, new SessionEpochIssuanceGate(accounts, floors));
  }

  @Bean
  AccountDeletionOperationStarter accountDeletionOperationStarter(
      @Qualifier("guestConversionTransactionTemplate") TransactionTemplate transactions,
      AccountRepository accounts,
      AccountDeletionOperationRepository operations,
      SessionEpochFloorStore floors) {
    return new TransactionalAccountDeletionOperationStarter(
        transactions, accounts, operations, floors);
  }

  @Bean
  GuestConversionOtpAcceptance guestConversionOtpAcceptance(
      @Qualifier("guestConversionTransactionTemplate") TransactionTemplate transactions,
      OtpCodeRepository otpCodes,
      GuestConversionOperationRepository operations) {
    return new TransactionalGuestConversionOtpAcceptance(transactions, otpCodes, operations);
  }

  @Bean
  GuestConversionLocalPromotion guestConversionLocalPromotion(
      @Qualifier("guestConversionTransactionTemplate") TransactionTemplate transactions,
      AccountRepository accounts,
      GuestConversionOperationRepository operations) {
    return new TransactionalGuestConversionLocalPromotion(transactions, accounts, operations);
  }

  @Bean
  GuestConversionPendingUserWorker guestConversionPendingUserWorker(
      GuestConversionOperationRepository operations,
      PrimaryProfileProvisioner primaryProfiles,
      GuestConversionLocalPromotion localPromotion,
      Clock clock) {
    return new GuestConversionPendingUserWorker(operations, primaryProfiles, localPromotion, clock);
  }
}
