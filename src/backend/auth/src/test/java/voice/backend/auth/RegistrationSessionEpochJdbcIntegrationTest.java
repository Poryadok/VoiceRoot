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
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
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
import voice.backend.auth.repository.BackupCodeRepository;
import voice.backend.auth.repository.JdbcBackupCodeRepository;
import voice.backend.auth.repository.JdbcRefreshTokenRepository;
import voice.backend.auth.repository.RefreshTokenRecord;
import voice.backend.auth.repository.RefreshTokenRepository;
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
import voice.backend.auth.userdb.ProfileSwitchValidator;
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
    jdbc().getJdbcTemplate().update("DELETE FROM backup_codes");
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

  @Test
  void refreshObservedBeforePasswordChangeCannotIssueFromItsStaleRow() throws Exception {
    DriverManagerDataSource dataSource = dataSource();
    NamedParameterJdbcTemplate jdbc = new NamedParameterJdbcTemplate(dataSource);
    JdbcAccountRepository accounts = new JdbcAccountRepository(jdbc);
    JdbcRefreshTokenRepository tokens = new JdbcRefreshTokenRepository(jdbc);
    SwitchableFloors floors = new SwitchableFloors();
    RecordingProfiles profiles = new RecordingProfiles(false);
    AuthService issuingService = service(dataSource, jdbc, accounts, tokens, floors, profiles);
    var initial = issuingService.register(command("refresh-race@example.test", false));

    CountDownLatch observedBeforeLock = new CountDownLatch(1);
    CountDownLatch continueRefresh = new CountDownLatch(1);
    AtomicBoolean firstLookup = new AtomicBoolean(true);
    RefreshTokenRepository gated = new DelegatingRefreshTokens(tokens) {
      @Override
      public Optional<RefreshTokenRecord> findByHash(String tokenHash) {
        Optional<RefreshTokenRecord> record = super.findByHash(tokenHash);
        if (firstLookup.compareAndSet(true, false)) {
          observedBeforeLock.countDown();
          await(continueRefresh);
        }
        return record;
      }
    };
    AuthService refreshService = service(dataSource, jdbc, accounts, gated, floors, profiles);
    var delayedRefresh = java.util.concurrent.CompletableFuture.supplyAsync(
        () -> refreshService.refresh(new voice.backend.auth.service.RefreshCommand(initial.refreshToken(), "{}")));
    assertThat(observedBeforeLock.await(5, TimeUnit.SECONDS)).isTrue();

    AuthService passwordService = service(dataSource, jdbc, accounts, tokens, floors, profiles);
    passwordService.changePassword(initial.accessToken(), "Correct horse battery staple", "New safer password", null);
    continueRefresh.countDown();
    assertThatThrownBy(() -> delayedRefresh.get(5, TimeUnit.SECONDS))
        .hasCauseInstanceOf(voice.backend.auth.service.AuthException.class)
        .hasMessageContaining("token_revoked");

    assertThatThrownBy(() -> passwordService.validate(initial.accessToken()))
        .isInstanceOf(voice.backend.auth.service.AuthException.class);
    assertThatThrownBy(() -> passwordService.login(new voice.backend.auth.service.LoginCommand(
        "refresh-race@example.test", null, "Correct horse battery staple", null, "{}")))
        .isInstanceOf(voice.backend.auth.service.AuthException.class);
    assertThat(passwordService.login(new voice.backend.auth.service.LoginCommand(
        "refresh-race@example.test", null, "New safer password", null, "{}")).accessToken()).isNotBlank();
    assertThat(activeRefreshTokenCount()).isEqualTo(1);
  }

  @Test
  void profileSwitchPausedBeforeAccountLockCannotIssueAfterPasswordChange() throws Exception {
    DriverManagerDataSource dataSource = dataSource();
    NamedParameterJdbcTemplate jdbc = new NamedParameterJdbcTemplate(dataSource);
    JdbcAccountRepository accounts = new JdbcAccountRepository(jdbc);
    JdbcRefreshTokenRepository tokens = new JdbcRefreshTokenRepository(jdbc);
    JdbcBackupCodeRepository backupCodes = new JdbcBackupCodeRepository(jdbc);
    SwitchableFloors floors = new SwitchableFloors();
    RecordingProfiles profiles = new RecordingProfiles(false);
    AuthService setup = service(dataSource, jdbc, accounts, tokens, floors, profiles);
    var initial = setup.register(command("profile-switch-race@example.test", false));

    CountDownLatch validationStarted = new CountDownLatch(1);
    CountDownLatch continueSwitch = new CountDownLatch(1);
    ProfileSwitchValidator gated = new ProfileSwitchValidator() {
      @Override
      public void validateOwnedSwitchable(UUID accountId, UUID profileId) {
        validationStarted.countDown();
        await(continueSwitch);
      }
    };
    AuthService switchService = service(
        dataSource, jdbc, accounts, tokens, backupCodes, floors, profiles, gated);
    var delayedSwitch = java.util.concurrent.CompletableFuture.supplyAsync(() ->
        switchService.switchActiveProfile(initial.accessToken(), UUID.randomUUID().toString(), "{}"));
    assertThat(validationStarted.await(5, TimeUnit.SECONDS)).isTrue();

    AuthService passwordService = service(dataSource, jdbc, accounts, tokens, floors, profiles);
    passwordService.changePassword(initial.accessToken(), "Correct horse battery staple", "New safer password", null);
    continueSwitch.countDown();
    assertThatThrownBy(() -> delayedSwitch.get(5, TimeUnit.SECONDS))
        .hasCauseInstanceOf(voice.backend.auth.service.AuthException.class)
        .hasMessageContaining("token_revoked");
    assertThat(activeRefreshTokenCount()).isZero();
  }

  @Test
  void failedEpochFloorRollsBackPasswordRefreshRowsAndBackupCodeConsumption() {
    DriverManagerDataSource dataSource = dataSource();
    NamedParameterJdbcTemplate jdbc = new NamedParameterJdbcTemplate(dataSource);
    JdbcAccountRepository accounts = new JdbcAccountRepository(jdbc);
    JdbcRefreshTokenRepository tokens = new JdbcRefreshTokenRepository(jdbc);
    JdbcBackupCodeRepository backupCodes = new JdbcBackupCodeRepository(jdbc);
    SwitchableFloors floors = new SwitchableFloors();
    AuthService service = service(dataSource, jdbc, accounts, tokens, backupCodes, floors,
        new RecordingProfiles(false));
    var initial = service.register(command("password-floor@example.test", false));
    UUID accountId = UUID.fromString(initial.accountId());
    TotpService totp = new TotpService(jdbcTotpProperties());
    accounts.saveTotpSecret(accountId, totp.encryptSecret("synthetic-test-secret"), true);
    String backup = new BackupCodeService(backupCodes).generateAndStore(accountId).getFirst();
    long epochBefore = epochForEmail("password-floor@example.test");
    long refreshesBefore = countRefreshTokens();
    floors.fail = true;

    assertThatThrownBy(() -> service.changePassword(
        initial.accessToken(), "Correct horse battery staple", "New safer password", backup))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(epochForEmail("password-floor@example.test")).isEqualTo(epochBefore);
    assertThat(countRefreshTokens()).isEqualTo(refreshesBefore);
    assertThat(service.validate(initial.accessToken()).userId()).isEqualTo(initial.accountId());
    floors.fail = false;
    assertThat(service.login(new voice.backend.auth.service.LoginCommand(
        "password-floor@example.test", null, "Correct horse battery staple", backup, "{}")).accessToken())
        .isNotBlank();
  }

  private AuthService service(SessionEpochFloorStore floors, RecordingProfiles profiles) {
    DriverManagerDataSource dataSource = dataSource();
    NamedParameterJdbcTemplate jdbc = new NamedParameterJdbcTemplate(dataSource);
    JdbcAccountRepository accounts = new JdbcAccountRepository(jdbc);
    return service(dataSource, jdbc, accounts, new JdbcRefreshTokenRepository(jdbc),
        new JdbcBackupCodeRepository(jdbc), floors, profiles);
  }

  private AuthService service(DriverManagerDataSource dataSource, NamedParameterJdbcTemplate jdbc,
      JdbcAccountRepository accounts, RefreshTokenRepository refreshTokens,
      SessionEpochFloorStore floors, RecordingProfiles profiles) {
    return service(dataSource, jdbc, accounts, refreshTokens,
        new JdbcBackupCodeRepository(jdbc), floors, profiles);
  }

  private AuthService service(DriverManagerDataSource dataSource, NamedParameterJdbcTemplate jdbc,
      JdbcAccountRepository accounts, RefreshTokenRepository refreshTokens,
      BackupCodeRepository backupCodes, SessionEpochFloorStore floors, RecordingProfiles profiles) {
    return service(dataSource, jdbc, accounts, refreshTokens, backupCodes, floors, profiles,
        new NoOpProfileSwitchValidator());
  }

  private AuthService service(DriverManagerDataSource dataSource, NamedParameterJdbcTemplate jdbc,
      JdbcAccountRepository accounts, RefreshTokenRepository refreshTokens,
      BackupCodeRepository backupCodes, SessionEpochFloorStore floors, RecordingProfiles profiles,
      ProfileSwitchValidator profileSwitchValidator) {
    AuthService service =
        new AuthService(
            accounts, refreshTokens, new RefreshTokenCodec(),
            new BCryptPasswordHasher(),
            JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), CLOCK),
            new InMemoryTokenBlacklist(CLOCK), new TotpService(jdbcTotpProperties()),
            new BackupCodeService(backupCodes), CLOCK, Duration.ofDays(30),
            profiles, (PhoneHashResolver) hashes -> Map.of(), new InMemorySubscriptionTierStore(),
            profileSwitchValidator, new InMemoryE2EKeyBackupRepository(),
            new NoopAuthEventPublisher(), new SimpleMeterRegistry(), new InMemoryAccountRestoreTokenStore(),
            new NoopMailSender(), floors);
    TransactionTemplate transactions = new TransactionTemplate(new DataSourceTransactionManager(dataSource));
    service.configureSecurityTransactions(transactions);
    service.configureRegistrationSessionEpochPreparer(
        new RegistrationSessionEpochPreparer(
            transactions, accounts,
            new SessionEpochIssuanceGate(accounts, floors)));
    return service;
  }

  private static DriverManagerDataSource dataSource() {
    return new DriverManagerDataSource(
        POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword());
  }

  private long activeRefreshTokenCount() {
    return jdbc().getJdbcTemplate().queryForObject(
        "SELECT count(*) FROM refresh_tokens WHERE revoked_at IS NULL", Long.class);
  }

  private static void await(CountDownLatch latch) {
    try {
      if (!latch.await(5, TimeUnit.SECONDS)) throw new AssertionError("test barrier timed out");
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      throw new AssertionError("test barrier interrupted", interrupted);
    }
  }

  private static class DelegatingRefreshTokens implements RefreshTokenRepository {
    private final RefreshTokenRepository delegate;
    DelegatingRefreshTokens(RefreshTokenRepository delegate) { this.delegate = delegate; }
    @Override public RefreshTokenRecord create(UUID accountId, UUID profileId, String tokenHash,
        String deviceInfoJson, String accessJti, Instant expiresAt, Instant now) {
      return delegate.create(accountId, profileId, tokenHash, deviceInfoJson, accessJti, expiresAt, now);
    }
    @Override public Optional<RefreshTokenRecord> findByHash(String tokenHash) { return delegate.findByHash(tokenHash); }
    @Override public Optional<RefreshTokenRecord> findById(UUID id) { return delegate.findById(id); }
    @Override public List<RefreshTokenRecord> listActiveByAccount(UUID id) { return delegate.listActiveByAccount(id); }
    @Override public RefreshTokenRecord revoke(String tokenHash, Instant now) { return delegate.revoke(tokenHash, now); }
    @Override public boolean revokeIfActive(String tokenHash, Instant now) { return delegate.revokeIfActive(tokenHash, now); }
    @Override public RefreshTokenRecord revokeById(UUID id, Instant now) { return delegate.revokeById(id, now); }
    @Override public void revokeAllForAccount(UUID id, Instant now) { delegate.revokeAllForAccount(id, now); }
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

  private static final class SwitchableFloors implements SessionEpochFloorStore {
    boolean fail;
    long floor;
    @Override public synchronized long recordAtLeast(UUID id, long epoch) {
      if (fail) throw new SessionEpochFloorUnavailableException("floor unavailable");
      floor = Math.max(floor, epoch);
      return floor;
    }
    @Override public synchronized long requireFloor(UUID id) {
      if (floor <= 0) throw new SessionEpochFloorUnavailableException("floor unavailable");
      return floor;
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
