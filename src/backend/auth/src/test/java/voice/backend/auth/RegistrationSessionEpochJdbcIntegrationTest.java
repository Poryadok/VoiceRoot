package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.nimbusds.jwt.SignedJWT;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.sql.Connection;
import java.sql.DriverManager;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Map;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.events.NoopAuthEventPublisher;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.JdbcAccountRepository;
import voice.backend.auth.repository.JdbcRefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.InMemoryAccountRestoreTokenStore;
import voice.backend.auth.service.InMemorySubscriptionTierStore;
import voice.backend.auth.service.RegistrationSessionEpochPreparer;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException;
import voice.backend.auth.sessionepoch.SessionEpochIssuanceGate;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

/** Real PostgreSQL proof that registration commits local create-and-seed before the User boundary. */
@Testcontainers(disabledWithoutDocker = true)
class RegistrationSessionEpochJdbcIntegrationTest {
  private static final Clock CLOCK =
      Clock.fixed(Instant.parse("2026-09-06T09:00:00Z"), ZoneOffset.UTC);

  @Container
  static final PostgreSQLContainer<?> POSTGRES =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_registration")
          .withUsername("voice")
          .withPassword("voice");

  @BeforeAll
  static void migrate() {
    Flyway.configure()
        .dataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword())
        .locations(
            "filesystem:"
                + GuestConversionDurabilityMigrationContractTest.authProjectRoot()
                    .resolve("src/main/resources/db/migration"))
        .load()
        .migrate();
  }

  @AfterEach
  void clear() {
    jdbc().getJdbcTemplate().update("DELETE FROM refresh_tokens");
    jdbc().getJdbcTemplate().update("DELETE FROM accounts");
  }

  @Test
  void regularFloorFailureRollsBackBeforeUserOrRefresh() {
    RecordingProfiles profiles = new RecordingProfiles(false);
    AuthService service = service(new FailingFloors(), profiles);
    assertThatThrownBy(() -> service.register(command("regular-fail@example.test", false)))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
    assertThat(countAccounts("regular-fail@example.test")).isZero();
    assertThat(profiles.calls).isZero();
    assertThat(countRefreshTokens()).isZero();
  }

  @Test
  void guestFloorFailureRollsBackBeforeUserOrRefresh() {
    RecordingProfiles profiles = new RecordingProfiles(false);
    AuthService service = service(new FailingFloors(), profiles);
    long before = countAccounts(null);
    assertThatThrownBy(() -> service.register(command(null, true)))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
    assertThat(countAccounts(null)).isEqualTo(before);
    assertThat(profiles.calls).isZero();
    assertThat(countRefreshTokens()).isZero();
  }

  @Test
  void successfulRegistrationCommitsPositiveEpochBeforeUserCall() throws Exception {
    RecordingProfiles profiles = new RecordingProfiles(false);
    HealthyFloors floors = new HealthyFloors();
    AuthService service = service(floors, profiles);
    var session = service.register(command("committed@example.test", false));
    assertThat(profiles.calls).isEqualTo(1);
    assertThat(profiles.sawCommittedPositiveEpoch).isTrue();
    assertThat(floors.recordCalls).isEqualTo(1);
    assertThat(floors.accountId).isEqualTo(UUID.fromString(session.accountId()));
    assertThat(floors.epoch).isPositive();
    assertThat(epochForEmail("committed@example.test")).isEqualTo(floors.epoch);
    var claims = service.validate(session.accessToken());
    assertThat(claims.userId()).isEqualTo(session.accountId());
    assertThat(claims.profileId()).isEqualTo(session.profileId());
    assertThat(claims.accountType()).isEqualTo(session.accountType());
    assertThat(SignedJWT.parse(session.accessToken()).getJWTClaimsSet().getLongClaim("session_epoch"))
        .isEqualTo(floors.epoch);
    assertThat(session.refreshToken()).isNotBlank();
    assertThat(countRefreshTokens()).isEqualTo(1);
  }

  @Test
  void userFailureKeepsCommittedAccountAndCreatesNoRefreshToken() {
    RecordingProfiles profiles = new RecordingProfiles(true);
    AuthService service = service(new HealthyFloors(), profiles);
    assertThatThrownBy(() -> service.register(command("user-fail@example.test", false)))
        .isInstanceOf(IllegalStateException.class);
    assertThat(countAccounts("user-fail@example.test")).isEqualTo(1);
    assertThat(countRefreshTokens()).isZero();
    assertThat(profiles.calls).isEqualTo(1);
  }

  private AuthService service(SessionEpochFloorStore floors, RecordingProfiles profiles) {
    DriverManagerDataSource dataSource =
        new DriverManagerDataSource(
            POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword());
    NamedParameterJdbcTemplate jdbc = new NamedParameterJdbcTemplate(dataSource);
    JdbcAccountRepository accounts = new JdbcAccountRepository(jdbc);
    AuthService service =
        new AuthService(
            accounts, new JdbcRefreshTokenRepository(jdbc), new RefreshTokenCodec(),
            new BCryptPasswordHasher(),
            JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), CLOCK),
            new InMemoryTokenBlacklist(CLOCK), new TotpService(jdbcTotpProperties()),
            new BackupCodeService(new InMemoryBackupCodeRepository()), CLOCK, Duration.ofDays(30),
            profiles, (PhoneHashResolver) hashes -> Map.of(), new InMemorySubscriptionTierStore(),
            new NoOpProfileSwitchValidator(), new InMemoryE2EKeyBackupRepository(),
            new NoopAuthEventPublisher(), new SimpleMeterRegistry(), new InMemoryAccountRestoreTokenStore(),
            new NoopMailSender(), floors);
    service.configureRegistrationSessionEpochPreparer(
        new RegistrationSessionEpochPreparer(
            new TransactionTemplate(new DataSourceTransactionManager(dataSource)),
            accounts,
            new SessionEpochIssuanceGate(accounts, floors)));
    return service;
  }

  private static RegisterCommand command(String email, boolean guest) {
    return new RegisterCommand(email, null, "Correct horse battery staple", guest, "{}");
  }

  private static AuthProperties jdbcTotpProperties() {
    AuthProperties properties = new AuthProperties();
    properties.getTotp().setTestBypass(false);
    properties.getTotp().setEncryptionKey("staging-totp-encryption-key-32b!!");
    return properties;
  }

  private NamedParameterJdbcTemplate jdbc() {
    return new NamedParameterJdbcTemplate(
        new DriverManagerDataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword()));
  }

  private long countAccounts(String email) {
    if (email == null) {
      return jdbc().getJdbcTemplate().queryForObject("SELECT count(*) FROM accounts", Long.class);
    }
    return jdbc().getJdbcTemplate().queryForObject(
        "SELECT count(*) FROM accounts WHERE email = ?", Long.class, email);
  }

  private long countRefreshTokens() {
    return jdbc().getJdbcTemplate().queryForObject("SELECT count(*) FROM refresh_tokens", Long.class);
  }

  private long epochForEmail(String email) {
    return jdbc().getJdbcTemplate().queryForObject(
        "SELECT session_epoch FROM accounts WHERE email = ?", Long.class, email);
  }

  private static final class HealthyFloors implements SessionEpochFloorStore {
    int recordCalls;
    UUID accountId;
    long epoch;

    @Override
    public long recordAtLeast(UUID id, long epoch) {
      recordCalls++;
      accountId = id;
      this.epoch = epoch;
      return epoch;
    }

    @Override
    public long requireFloor(UUID id) {
      throw new AssertionError();
    }
  }

  private static final class FailingFloors implements SessionEpochFloorStore {
    @Override
    public long recordAtLeast(UUID id, long epoch) {
      throw new IllegalStateException("redis down");
    }

    @Override
    public long requireFloor(UUID id) {
      throw new AssertionError();
    }
  }

  private static final class RecordingProfiles implements PrimaryProfileProvisioner {
    final boolean fail;
    int calls;
    boolean sawCommittedPositiveEpoch;

    RecordingProfiles(boolean fail) {
      this.fail = fail;
    }

    @Override
    public String ensurePrimaryProfile(UUID id, String hint, boolean guest) {
      calls++;
      assertThat(TransactionSynchronizationManager.isActualTransactionActive()).isFalse();
      try (Connection connection =
              DriverManager.getConnection(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword());
          var statement = connection.prepareStatement("SELECT session_epoch FROM accounts WHERE id = ?")) {
        statement.setObject(1, id);
        var rows = statement.executeQuery();
        assertThat(rows.next()).isTrue();
        sawCommittedPositiveEpoch = rows.getLong(1) > 0;
      } catch (Exception exception) {
        throw new AssertionError(exception);
      }
      if (fail) {
        throw new IllegalStateException("user unavailable");
      }
      return UUID.randomUUID().toString();
    }

    @Override
    public void clearGuestAccountFlag(UUID id) {
      throw new UnsupportedOperationException();
    }
  }
}
