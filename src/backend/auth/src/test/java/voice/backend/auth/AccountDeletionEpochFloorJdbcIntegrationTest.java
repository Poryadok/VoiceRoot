package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.verify;

import javax.sql.DataSource;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Import;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.AccountDeletionOperationStarter;
import voice.backend.auth.service.GuestConversionLocalPromotion;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.GuestConversionPendingUserWorker;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.TransactionalAccountDeletionOperationStarter;
import voice.backend.auth.service.TransactionalGuestConversionLocalPromotion;
import voice.backend.auth.service.TransactionalGuestConversionOtpAcceptance;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException;
import voice.backend.auth.support.JdbcUserContractTestConfiguration;

/** Proves a Redis floor outage cannot leave a PostgreSQL deletion visible to strict consumers. */
@SpringBootTest
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
@Import({JdbcUserContractTestConfiguration.class, JdbcTransactionTestConfiguration.class})
class AccountDeletionEpochFloorJdbcIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  @DynamicPropertySource
  static void databaseProperties(DynamicPropertyRegistry registry) {
    registry.add("voice.auth.jdbc.url", postgres::getJdbcUrl);
    registry.add("spring.datasource.username", postgres::getUsername);
    registry.add("spring.datasource.password", postgres::getPassword);
    registry.add("spring.flyway.user", postgres::getUsername);
    registry.add("spring.flyway.password", postgres::getPassword);
  }

  @Autowired AuthService authService;
  @Autowired AccountRepository accounts;
  @Autowired AccountDeletionOperationRepository deletionOperations;
  @Autowired AccountDeletionOperationStarter deletionStarter;
  @Autowired PlatformTransactionManager transactionManager;
  @Autowired
  @Qualifier("guestConversionTransactionTemplate")
  TransactionTemplate guestConversionTransactions;
  @Autowired GuestConversionOtpAcceptance guestConversionOtpAcceptance;
  @Autowired GuestConversionLocalPromotion guestConversionLocalPromotion;
  @Autowired GuestConversionPendingUserWorker guestConversionPendingUserWorker;
  @MockBean SessionEpochFloorStore sessionEpochFloors;
  @MockBean TokenBlacklist tokenBlacklist;

  @Test
  void floorFailureRollsBackJdbcDeletionAndItsPendingFloorOperation() {
    assertThat(deletionStarter).isInstanceOf(TransactionalAccountDeletionOperationStarter.class);
    assertThat(guestConversionTransactions.getTransactionManager()).isSameAs(transactionManager);
    assertThat(guestConversionOtpAcceptance)
        .isInstanceOf(TransactionalGuestConversionOtpAcceptance.class);
    assertThat(guestConversionLocalPromotion)
        .isInstanceOf(TransactionalGuestConversionLocalPromotion.class);
    assertThat(guestConversionPendingUserWorker).isNotNull();
    org.mockito.Mockito.when(sessionEpochFloors.recordAtLeast(any(), anyLong()))
        .thenAnswer(invocation -> invocation.getArgument(1));
    var session =
        authService.register(
            new RegisterCommand(
                "jdbc-floor-rollback@example.com",
                null,
                "Correct horse battery staple",
                false,
                "{}"));
    Account before = accounts.findByEmail("jdbc-floor-rollback@example.com").orElseThrow();
    doThrow(new SessionEpochFloorUnavailableException("redis unavailable"))
        .when(sessionEpochFloors)
        .recordAtLeast(any(), anyLong());

    assertThatThrownBy(
            () ->
                authService.deleteAccount(
                    "Bearer " + session.accessToken(), "Correct horse battery staple"))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    Account afterFailure = accounts.findById(before.id().toString()).orElseThrow();
    assertThat(afterFailure)
        .extracting(Account::status, Account::deletedAt, Account::sessionEpoch)
        .containsExactly("active", null, before.sessionEpoch());
    assertThat(deletionOperations.findByAccountAndEpoch(before.id(), before.sessionEpoch() + 1))
        .isEmpty();
    verify(sessionEpochFloors).recordAtLeast(before.id(), before.sessionEpoch() + 1);
  }
}

@TestConfiguration(proxyBeanMethods = false)
class JdbcTransactionTestConfiguration {
  @Bean
  PlatformTransactionManager transactionManager(DataSource dataSource) {
    return new DataSourceTransactionManager(dataSource);
  }
}
