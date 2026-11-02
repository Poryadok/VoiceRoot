package voice.backend.auth;

import static org.assertj.core.api.Assertions.*;

import java.lang.reflect.*;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.*;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.*;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.*;
import voice.backend.auth.security.*;
import voice.backend.auth.service.*;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.events.NoopAuthEventPublisher;
import voice.backend.auth.sessionepoch.InMemorySessionEpochFloorStore;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;

class OtpResetJdbcSecurityIntegrationTest {
  static PostgreSQLContainer<?> postgres;
  static DriverManagerDataSource dataSource;
  static final Instant NOW = Instant.parse("2026-11-02T10:00:00Z");
  static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);
  final RefreshTokenCodec codec = new RefreshTokenCodec();
  final BCryptPasswordHasher passwords = new BCryptPasswordHasher();
  final InMemorySessionEpochFloorStore floors = new InMemorySessionEpochFloorStore();
  NamedParameterJdbcTemplate jdbc;
  JdbcAccountRepository accounts;
  JdbcOtpCodeRepository codes;
  JdbcRefreshTokenRepository refresh;
  TransactionTemplate transactions;

  @BeforeAll static void database() {
    String url = System.getenv("AUTH_OTP_SECURITY_TEST_JDBC_URL");
    if (url == null) {
      postgres = new PostgreSQLContainer<>("postgres:16-alpine")
          .withDatabaseName("auth_db").withUsername("voice").withPassword("voice");
      postgres.start();
      url = postgres.getJdbcUrl();
    }
    dataSource = new DriverManagerDataSource(url, "voice", "voice");
    Flyway.configure().dataSource(dataSource)
        .locations("filesystem:" + GuestConversionDurabilityMigrationContractTest.authProjectRoot()
            .resolve("src/main/resources/db/migration"))
        .load().migrate();
  }

  @AfterAll static void cleanup() { if (postgres != null) postgres.stop(); }

  @BeforeEach void repositories() {
    jdbc = new NamedParameterJdbcTemplate(dataSource);
    accounts = new JdbcAccountRepository(jdbc);
    codes = new JdbcOtpCodeRepository(jdbc);
    refresh = new JdbcRefreshTokenRepository(jdbc);
    transactions = new TransactionTemplate(new DataSourceTransactionManager(dataSource));
  }

  OtpService service(AccountRepository accountRepository, OtpCodeRepository otpRepository,
      RefreshTokenRepository refreshRepository) {
    var service = new OtpService(accountRepository, otpRepository, refreshRepository, codec,
        passwords, new NoopMailSender(), new InMemoryOtpThrottle(CLOCK), CLOCK,
        (account, otp, now) -> {});
    service.configureSecurityTransactions(transactions);
    service.configureSessionEpochFloors(floors);
    return service;
  }

  Account account() {
    var account = accounts.create(UUID.randomUUID() + "@example.com", null,
        passwords.hash("OldPassword99!"), "regular");
    floors.recordAtLeast(account.id(), account.sessionEpoch());
    codes.create(account.id(), codec.hash("123456"), "password_reset", NOW.plusSeconds(600), NOW);
    refresh.create(account.id(), "refresh-" + account.id(), "{}", "jti", NOW.plusSeconds(3600), NOW);
    return account;
  }

  @Test void replacementConsumptionDoesNotReactivatePriorOtpAcrossJdbcInstances() {
    var account = account();
    for (String type : List.of("email_verify", "password_reset")) {
      var a = codes.create(account.id(), codec.hash("111111"), type, NOW.plusSeconds(600), NOW);
      var b = new JdbcOtpCodeRepository(jdbc).create(account.id(), codec.hash("222222"), type,
          NOW.plusSeconds(660), NOW.plusSeconds(60));
      codes.markUsed(b.id(), NOW.plusSeconds(61));
      assertThat(codes.findLatestValid(account.id(), type, NOW.plusSeconds(62))).isEmpty();
      assertThatThrownBy(() -> codes.markUsed(a.id(), NOW.plusSeconds(62)))
          .isInstanceOf(AuthException.class).hasMessage("invalid_otp");
    }
  }

  @Test void preUpgradeRowsCannotResurrectAfterNewestCodeWasAlreadyConsumed() {
    var account = account();
    jdbc.getJdbcTemplate().update("DELETE FROM otp_codes WHERE account_id = ?", account.id());
    insertLegacyCode(account.id(), "111111", NOW.minusSeconds(120), null);
    insertLegacyCode(account.id(), "222222", NOW.minusSeconds(60), NOW.minusSeconds(30));
    assertThatThrownBy(() -> service(accounts, codes, refresh).resetPassword(
        new ResetPasswordCommand(account.email(), "111111", "RejectedPassword99!")))
        .isInstanceOf(AuthException.class).hasMessage("invalid_otp");
    assertThat(accounts.findById(account.id().toString()).orElseThrow().passwordHash())
        .isEqualTo(account.passwordHash());
    assertThat(refresh.findByHash("refresh-" + account.id()).orElseThrow().revoked()).isFalse();
  }

  @Test void consumingPreUpgradeNewestCodeDoesNotExposeItsUnusedPredecessor() {
    var account = account();
    jdbc.getJdbcTemplate().update("DELETE FROM otp_codes WHERE account_id = ?", account.id());
    insertLegacyCode(account.id(), "111111", NOW.minusSeconds(120), null);
    insertLegacyCode(account.id(), "222222", NOW.minusSeconds(60), null);
    var service = service(accounts, codes, refresh);
    service.resetPassword(new ResetPasswordCommand(account.email(), "222222", "AcceptedPassword99!"));
    assertThatThrownBy(() -> service.resetPassword(
        new ResetPasswordCommand(account.email(), "111111", "RejectedPassword99!")))
        .isInstanceOf(AuthException.class).hasMessage("invalid_otp");
    assertThat(passwords.matches("AcceptedPassword99!",
        accounts.findById(account.id().toString()).orElseThrow().passwordHash())).isTrue();
  }

  private void insertLegacyCode(UUID accountId, String code, Instant issued, Instant used) {
    // The pre-upgrade writer inserted without invalidating earlier rows.
    jdbc.getJdbcTemplate().update("""
        INSERT INTO otp_codes(account_id, code, type, expires_at, created_at, used_at)
        VALUES (?, decode(?, 'hex'), 'password_reset', ?, ?, ?)
        """, accountId, codec.hash(code), java.sql.Timestamp.from(issued.plusSeconds(600)),
        java.sql.Timestamp.from(issued), used == null ? null : java.sql.Timestamp.from(used));
  }

  @Test void newestLegacyIssuanceIsAuthoritativeEvenWhenItExpiresBeforeItsPredecessor() {
    var account = account();
    jdbc.getJdbcTemplate().update("DELETE FROM otp_codes WHERE account_id = ?", account.id());
    insertLegacyCode(account.id(), "111111", NOW.minusSeconds(120), null);
    insertLegacyCode(account.id(), "222222", NOW.minusSeconds(60), null);
    jdbc.getJdbcTemplate().update("UPDATE otp_codes SET expires_at = ? WHERE account_id = ? AND created_at = ?",
        java.sql.Timestamp.from(NOW.minusSeconds(1)), account.id(), java.sql.Timestamp.from(NOW.minusSeconds(60)));
    assertThat(codes.findLatestValid(account.id(), "password_reset", NOW)).isEmpty();
  }

  @Test void ambiguousLegacyLatestIssuanceFailsClosedForUnusedAndSpentTies() {
    for (Instant used : Arrays.asList(null, NOW.minusSeconds(10))) {
      var account = account();
      jdbc.getJdbcTemplate().update("DELETE FROM otp_codes WHERE account_id = ?", account.id());
      insertLegacyCode(account.id(), "111111", NOW.minusSeconds(60), null);
      insertLegacyCode(account.id(), "222222", NOW.minusSeconds(60), used);
      assertThat(codes.findLatestValid(account.id(), "password_reset", NOW)).isEmpty();
      // A real resend establishes a fresh unambiguous authority even with a backward clock.
      var fresh = codes.create(account.id(), codec.hash("333333"), "password_reset",
          NOW.plusSeconds(30), NOW.minusSeconds(120));
      assertThat(codes.findLatestValid(account.id(), "password_reset", NOW)).contains(fresh);
    }
  }

  @Test void fixedAndBackwardClocksAndShorterExpiryDoNotChangeIssuanceOrder() {
    var account = account();
    var second = codes.create(account.id(), codec.hash("222222"), "password_reset", NOW.plusSeconds(300), NOW);
    assertThat(codes.findLatestValid(account.id(), "password_reset", NOW)).contains(second);
    var third = codes.create(account.id(), codec.hash("333333"), "password_reset",
        NOW.plusSeconds(100), NOW.minusSeconds(60));
    assertThat(codes.findLatestValid(account.id(), "password_reset", NOW)).contains(third);
    codes.markUsed(third.id(), NOW);
    assertThat(codes.findLatestValid(account.id(), "password_reset", NOW)).isEmpty();
  }

  @Test void independentResetServicesHaveExactlyOneWinner() throws Exception {
    var account = account();
    var entered = new CountDownLatch(1);
    var release = new CountDownLatch(1);
    var secondRead = new CountDownLatch(1);
    String secondTag = "otp-reset-second-" + UUID.randomUUID();
    OtpCodeRepository firstCodes = new JdbcOtpCodeRepository(jdbc) {
      @Override public Optional<OtpCodeRecord> findLatestValid(UUID id, String type, Instant now) {
        var observed = super.findLatestValid(id, type, now);
        entered.countDown(); await(release);
        return observed;
      }
    };
    var first = service(accounts, firstCodes, refresh);
    var secondAccounts = new JdbcAccountRepository(jdbc) {
      @Override public void lockForSecurityMutation(UUID id) {
        OtpResetJdbcSecurityIntegrationTest.this.jdbc.getJdbcTemplate().queryForObject(
            "SELECT set_config('application_name', ?, false)", String.class, secondTag);
        super.lockForSecurityMutation(id);
      }
    };
    var secondCodes = new JdbcOtpCodeRepository(jdbc) {
      @Override public Optional<OtpCodeRecord> findLatestValid(UUID id, String type, Instant now) {
        var observed = super.findLatestValid(id, type, now);
        secondRead.countDown();
        return observed;
      }
    };
    var second = service(secondAccounts, secondCodes, new JdbcRefreshTokenRepository(jdbc));
    try (var executor = Executors.newFixedThreadPool(2)) {
      var a = executor.submit(() -> reset(first, account, "FirstPassword99!"));
      assertThat(entered.await(10, TimeUnit.SECONDS)).isTrue();
      var b = executor.submit(() -> reset(second, account, "SecondPassword99!"));
      // Release only after the other request has either observed the OTP or is
      // blocked by database serialization. Without lock/CAS, both retain A.
      boolean blocked = false;
      long deadline = System.nanoTime() + Duration.ofSeconds(8).toNanos();
      while (secondRead.getCount() != 0 && System.nanoTime() < deadline) {
        Integer waiting = jdbc.getJdbcTemplate().queryForObject(
            "SELECT count(*) FROM pg_stat_activity WHERE application_name = ? AND wait_event_type = 'Lock'",
            Integer.class, secondTag);
        if (waiting != null && waiting > 0) { blocked = true; break; }
        secondRead.await(20, TimeUnit.MILLISECONDS);
      }
      assertThat(blocked || secondRead.getCount() == 0).as("second reset reached the competing boundary").isTrue();
      release.countDown();
      assertThat(List.of(a.get(15, TimeUnit.SECONDS), b.get(15, TimeUnit.SECONDS)))
          .containsExactlyInAnyOrder("success", "invalid_otp");
      assertThat(accounts.findById(account.id().toString()).orElseThrow().sessionEpoch())
          .isEqualTo(account.sessionEpoch() + 1);
      assertThat(refresh.findByHash("refresh-" + account.id()).orElseThrow().revoked()).isTrue();
    } finally { release.countDown(); }
  }

  @Test void resendInvalidatesAnAlreadyReadResetCodeBeforeItCanChangePassword() throws Exception {
    var account = account();
    var observed = new CountDownLatch(1);
    var release = new CountDownLatch(1);
    var pausedCodes = new JdbcOtpCodeRepository(jdbc) {
      @Override public Optional<OtpCodeRecord> findLatestValid(UUID id, String type, Instant now) {
        var result = super.findLatestValid(id, type, now);
        observed.countDown(); await(release);
        return result;
      }
    };
    try (var executor = Executors.newSingleThreadExecutor()) {
      var pending = executor.submit(() -> reset(service(accounts, pausedCodes, refresh), account, "RejectedPassword99!"));
      assertThat(observed.await(10, TimeUnit.SECONDS)).isTrue();
      codes.create(account.id(), codec.hash("222222"), "password_reset", NOW.plusSeconds(660), NOW.plusSeconds(60));
      release.countDown();
      assertThat(pending.get(15, TimeUnit.SECONDS)).isEqualTo("invalid_otp");
      assertThat(accounts.findById(account.id().toString()).orElseThrow().passwordHash()).isEqualTo(account.passwordHash());
      assertThat(refresh.findByHash("refresh-" + account.id()).orElseThrow().revoked()).isFalse();
      service(accounts, codes, refresh).resetPassword(new ResetPasswordCommand(account.email(), "222222", "AcceptedPassword99!"));
    } finally { release.countDown(); }
  }

  @Test void concurrentFirstSendsLeaveOnlyOneAuthorityAndFailedInsertPreservesIt() throws Exception {
    var account = accounts.create(UUID.randomUUID() + "@example.com", null, "old", "regular");
    var inserted = new CountDownLatch(1);
    var release = new CountDownLatch(1);
    try (var executor = Executors.newFixedThreadPool(2)) {
      var a = executor.submit(() -> transactions.execute(status -> {
        var result = codes.create(account.id(), codec.hash("111111"), "email_verify", NOW.plusSeconds(600), NOW);
        inserted.countDown(); await(release); return result;
      }));
      assertThat(inserted.await(10, TimeUnit.SECONDS)).isTrue();
      var b = executor.submit(() -> new JdbcOtpCodeRepository(jdbc).create(account.id(), codec.hash("222222"),
          "email_verify", NOW.plusSeconds(660), NOW.plusSeconds(60)));
      release.countDown();
      a.get(15, TimeUnit.SECONDS);
      var latest = b.get(15, TimeUnit.SECONDS);
      assertThat(jdbc.getJdbcTemplate().queryForObject("SELECT count(*) FROM otp_codes WHERE account_id = ? AND used_at IS NULL",
          Integer.class, account.id())).isEqualTo(1);
      assertThatThrownBy(() -> codes.create(account.id(), "invalid-hex", "email_verify", NOW.plusSeconds(700), NOW.plusSeconds(100)))
          .isInstanceOf(RuntimeException.class);
      assertThat(codes.findLatestValid(account.id(), "email_verify", NOW.plusSeconds(100))).contains(latest);
    } finally { release.countDown(); }
  }

  @Test void everyResetWriteFailureRollsBackOtpPasswordEpochAndRevocation() {
    for (String failingMethod : List.of("markUsed", "updatePasswordHash", "advanceSessionEpochAtLeast", "revokeAllForAccount")) {
      var account = account();
      AccountRepository accountFault = afterFault(AccountRepository.class, accounts, failingMethod);
      OtpCodeRepository otpFault = afterFault(OtpCodeRepository.class, codes, failingMethod);
      RefreshTokenRepository refreshFault = afterFault(RefreshTokenRepository.class, refresh, failingMethod);
      var failing = service(accountFault, otpFault, refreshFault);
      assertThatThrownBy(() -> failing.resetPassword(new ResetPasswordCommand(account.email(), "123456", "NewPassword99!")))
          .isInstanceOf(RuntimeException.class).hasStackTraceContaining("injected " + failingMethod);
      var unchanged = accounts.findById(account.id().toString()).orElseThrow();
      assertThat(unchanged.passwordHash()).isEqualTo(account.passwordHash());
      assertThat(unchanged.sessionEpoch()).isEqualTo(account.sessionEpoch());
      assertThat(codes.findLatestValid(account.id(), "password_reset", NOW)).isPresent();
      assertThat(refresh.findByHash("refresh-" + account.id()).orElseThrow().revoked()).isFalse();
      assertThat(reset(service(accounts, codes, refresh), account, "RetryPassword99!")).isEqualTo("success");
    }
  }

  @Test void lostFloorAcknowledgementRollsBackSqlAndRetryAdvancesBeyondRetainedFloor() {
    var account = account();
    var failing = service(accounts, codes, refresh);
    failing.configureSessionEpochFloors(afterFault(voice.backend.auth.sessionepoch.SessionEpochFloorStore.class,
        floors, "recordAtLeast"));
    assertThatThrownBy(() -> failing.resetPassword(new ResetPasswordCommand(account.email(), "123456", "NewPassword99!")))
        .isInstanceOf(voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException.class);
    assertThat(accounts.findById(account.id().toString()).orElseThrow().passwordHash()).isEqualTo(account.passwordHash());
    assertThat(accounts.findById(account.id().toString()).orElseThrow().sessionEpoch()).isEqualTo(account.sessionEpoch());
    assertThat(codes.findLatestValid(account.id(), "password_reset", NOW)).isPresent();
    assertThat(refresh.findByHash("refresh-" + account.id()).orElseThrow().revoked()).isFalse();
    long retained = floors.requireFloor(account.id());
    assertThat(retained).isGreaterThan(account.sessionEpoch());
    assertThat(reset(service(accounts, codes, refresh), account, "RetryPassword99!")).isEqualTo("success");
    assertThat(accounts.findById(account.id().toString()).orElseThrow().sessionEpoch()).isGreaterThan(retained);
  }

  @Test void legacyRefreshPausedInUserProvisioningCannotIssueAfterIndependentReset() throws Exception {
    var account = account();
    var entered = new CountDownLatch(1);
    var release = new CountDownLatch(1);
    var properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.MEMORY);
    var authentication = new AuthService(new JdbcAccountRepository(jdbc), new JdbcRefreshTokenRepository(jdbc),
        codec, passwords, JwtService.forTests("voice-auth", "voice-client", "otp-race", Duration.ofMinutes(15), CLOCK),
        new InMemoryTokenBlacklist(CLOCK), new TotpService(properties),
        new BackupCodeService(new InMemoryBackupCodeRepository()), CLOCK, Duration.ofDays(30),
        (id, hint, guest) -> { entered.countDown(); await(release); return UUID.randomUUID().toString(); },
        hashes -> Map.of(), new InMemorySubscriptionTierStore(), new NoOpProfileSwitchValidator(),
        new InMemoryE2EKeyBackupRepository(), new NoopAuthEventPublisher(), new SimpleMeterRegistry(),
        new InMemoryAccountRestoreTokenStore(), new NoopMailSender(), floors);
    authentication.configureSecurityTransactions(transactions);
    String credential = "legacy-" + account.id();
    refresh.create(account.id(), codec.hash(credential), "{}", "legacy-jti", NOW.plusSeconds(3600), NOW);
    try (var executor = Executors.newSingleThreadExecutor()) {
      var pending = executor.submit(() -> {
        try { authentication.refresh(new RefreshCommand(credential, "{}")); return "issued"; }
        catch (AuthException failure) { return failure.getMessage(); }
      });
      assertThat(entered.await(10, TimeUnit.SECONDS)).isTrue();
      assertThat(reset(service(accounts, codes, refresh), account, "NewPassword99!")).isEqualTo("success");
      release.countDown();
      assertThat(pending.get(15, TimeUnit.SECONDS)).isEqualTo("token_revoked");
      assertThat(refresh.listActiveByAccount(account.id())).isEmpty();
    } finally { release.countDown(); }
  }

  private static String reset(OtpService service, Account account, String password) {
    try { service.resetPassword(new ResetPasswordCommand(account.email(), "123456", password)); return "success"; }
    catch (AuthException failure) { return failure.getMessage(); }
  }

  private static void await(CountDownLatch latch) {
    try { if (!latch.await(10, TimeUnit.SECONDS)) throw new AssertionError("barrier timeout"); }
    catch (InterruptedException interrupted) { Thread.currentThread().interrupt(); throw new AssertionError(interrupted); }
  }

  @SuppressWarnings("unchecked") static <T> T afterFault(Class<T> type, T delegate, String failingMethod) {
    return (T) Proxy.newProxyInstance(type.getClassLoader(), new Class<?>[] {type}, (p, method, args) -> {
      try {
        Object result = method.invoke(delegate, args);
        if (method.getName().equals(failingMethod)) throw new IllegalStateException("injected " + failingMethod);
        return result;
      } catch (InvocationTargetException failure) { throw failure.getCause(); }
    });
  }
}
