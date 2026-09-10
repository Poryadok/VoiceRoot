package voice.backend.auth.ownershipproof;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;

import java.time.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.function.Function;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.JdbcBackupCodeRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** Real transactions: durable one-use grants and revocation must survive service recreation. */
@Testcontainers(disabledWithoutDocker = true)
class OwnershipTransferProofJdbcIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db").withUsername("voice").withPassword("voice");

  private final Instant now = Instant.parse("2026-09-10T10:00:00Z");
  private final UUID account = UUID.randomUUID();
  private final BCryptPasswordHasher passwords = mock(BCryptPasswordHasher.class);
  private final TotpService totp = mock(TotpService.class);
  private final SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
  private NamedParameterJdbcTemplate jdbc;
  private JdbcOwnershipTransferProofStore store;
  private BackupCodeService backup;
  private ProofBinding binding;

  @BeforeAll static void migrate() {
    Flyway.configure().dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .locations("classpath:db/migration").load().migrate();
  }

  @BeforeEach void seed() {
    var dataSource = new DriverManagerDataSource(
        postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
    jdbc = new NamedParameterJdbcTemplate(dataSource);
    store = new JdbcOwnershipTransferProofStore(jdbc, new DataSourceTransactionManager(dataSource));
    backup = new BackupCodeService(new JdbcBackupCodeRepository(jdbc));
    jdbc.update("""
        INSERT INTO accounts (id,password_hash,type,status,session_epoch)
        VALUES (:id,'hash','regular','active',7)
        """, Map.of("id", account));
    binding = new ProofBinding(account, UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), 7);
    when(passwords.matches("correct", "hash")).thenReturn(true);
    when(floors.requireFloor(account)).thenReturn(7L);
  }

  private OwnershipTransferProofService service(OwnershipTransferProofStore source, Instant at) {
    return new OwnershipTransferProofService(source, passwords, totp, backup, floors, Clock.fixed(at, ZoneOffset.UTC));
  }

  private OwnershipTransferProofService service() { return service(store, now); }

  @Test void concurrentIdenticalConsumesReturnOneDurableReceiptAcrossServiceInstances() throws Exception {
    var issued = service().issue(binding, "correct", "", "");
    var start = new CyclicBarrier(8);
    var workers = Executors.newFixedThreadPool(8);
    try {
      var tasks = new ArrayList<Callable<Object>>();
      for (int i = 0; i < 8; i++) {
        tasks.add(() -> {
          start.await(10, TimeUnit.SECONDS);
          return independentService().consume(binding, issued.proof());
        });
      }
      var receipts = results(workers.invokeAll(tasks, 30, TimeUnit.SECONDS));
      assertThat(receipts).hasSize(8);
      assertThat(new HashSet<>(receipts)).hasSize(1);
      var durable = store.withAccount(account, session -> session.find(binding.operationId()).orElseThrow());
      assertThat(durable.consumedAt()).isEqualTo(now);
      assertThat(durable.proofHash()).hasSize(64).isNotEqualTo(issued.proof());
      var restartedDataSource = new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
      var restartedStore = new JdbcOwnershipTransferProofStore(new NamedParameterJdbcTemplate(restartedDataSource), new DataSourceTransactionManager(restartedDataSource));
      assertThat(service(restartedStore, now.plusSeconds(301)).consume(binding, issued.proof())).isEqualTo(receipts.getFirst());
      assertThat(jdbc.queryForObject("SELECT count(*) FROM ownership_transfer_proofs WHERE operation_id=:id",
          Map.of("id", binding.operationId()), Long.class)).isEqualTo(1L);
    } finally {
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test void failedReceiptTransactionLeavesProofUsableAndNoPartialReceipt() {
    var issued = service().issue(binding, "correct", "", "");
    var failing = failAfterWrite(false);
    assertThatThrownBy(() -> service(failing, now).consume(binding, issued.proof()))
        .isInstanceOf(InjectedFailure.class);
    var persisted = store.withAccount(account, session -> session.find(binding.operationId()).orElseThrow());
    assertThat(persisted.consumedAt()).isNull();
    assertThat(service().consume(binding, issued.proof()).consumedAt()).isEqualTo(now);
  }

  @Test void issueInsertFailureRollsBackBackupCodeAndProofTogether() {
    update("totp_enabled=TRUE,totp_secret=decode('01','hex')");
    String code = backup.generateAndStore(account).getFirst();
    assertThatThrownBy(() -> service(failAfterWrite(true), now).issue(binding, "correct", "", code))
        .isInstanceOf(InjectedFailure.class);
    Optional<StoredProof> rolledBack = store.withAccount(account, session -> session.find(binding.operationId()));
    assertThat(rolledBack).isEmpty();
    assertThat(jdbc.queryForObject("SELECT count(*) FROM backup_codes WHERE account_id=:id AND used_at IS NOT NULL",
        Map.of("id", account), Long.class)).isZero();
    var issued = service().issue(binding, "correct", "", code);
    assertThat(service().consume(binding, issued.proof()).binding()).isEqualTo(binding);
    assertThat(backup.consume(account, code)).isFalse();
  }

  @Test void concurrentIssuesCannotSpendOneBackupCodeTwice() throws Exception {
    update("totp_enabled=TRUE,totp_secret=decode('01','hex')");
    String code = backup.generateAndStore(account).getFirst();
    var other = new ProofBinding(account, binding.profileId(), binding.spaceId(), binding.newOwnerProfileId(), UUID.randomUUID(), 7);
    var start = new CyclicBarrier(2);
    var workers = Executors.newFixedThreadPool(2);
    try {
      var tasks = List.<Callable<Boolean>>of(
          () -> issueAfterBarrier(binding, code, start), () -> issueAfterBarrier(other, code, start));
      assertThat(results(workers.invokeAll(tasks, 30, TimeUnit.SECONDS))).containsExactlyInAnyOrder(true, false);
      assertThat(jdbc.queryForObject("SELECT count(*) FROM ownership_transfer_proofs WHERE account_id=:id",
          Map.of("id", account), Long.class)).isEqualTo(1L);
      assertThat(jdbc.queryForObject("SELECT count(*) FROM backup_codes WHERE account_id=:id AND used_at IS NOT NULL",
          Map.of("id", account), Long.class)).isEqualTo(1L);
    } finally {
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  private boolean issueAfterBarrier(ProofBinding requested, String code, CyclicBarrier start) throws Exception {
    start.await(10, TimeUnit.SECONDS);
    try { independentService().issue(requested, "correct", "", code); return true; }
    catch (ProofDeniedException expected) { return false; }
  }

  @ParameterizedTest
  @CsvSource(delimiter = '|', value = {
      "password_hash='changed'|password_hash='hash'",
      "totp_enabled=TRUE|totp_enabled=FALSE",
      "totp_secret=decode('01','hex')|totp_secret=NULL",
      "status='suspended'|status='active'",
      "deleted_at=CURRENT_TIMESTAMP|deleted_at=NULL",
      "session_epoch=8|session_epoch=7",
      "type='guest'|type='regular'",
      "email='changed@example.com'|email=NULL",
      "phone='+15550101010'|phone=NULL",
      "regular_email_verification_pending=TRUE|regular_email_verification_pending=FALSE"
  })
  void securityChangeAndRevertPermanentlyRevokesUnconsumedProof(String change, String revert) {
    var issued = service().issue(binding, "correct", "", "");
    update(change);
    update(revert);
    assertThatThrownBy(() -> service().consume(binding, issued.proof())).isInstanceOf(ProofDeniedException.class);
    assertThat(store.withAccount(account, session -> session.find(binding.operationId()).orElseThrow()).consumedAt()).isNull();
  }

  @Test void backupReplacementAndConsumptionRevokeOlderProofsButOwnBackupIssueIsUsable() {
    update("totp_enabled=TRUE,totp_secret=decode('01','hex')");
    when(totp.verifyEncrypted(any(), eq("123456"))).thenReturn(true);
    var codes = backup.generateAndStore(account);
    var first = service().issue(binding, "correct", "123456", "");
    assertThat(backup.consume(account, codes.getFirst())).isTrue();
    assertThatThrownBy(() -> service().consume(binding, first.proof())).isInstanceOf(ProofDeniedException.class);
    var secondBinding = new ProofBinding(account, binding.profileId(), binding.spaceId(), binding.newOwnerProfileId(), UUID.randomUUID(), 7);
    var second = service().issue(secondBinding, "correct", "123456", "");
    var replacement = backup.generateAndStore(account);
    assertThatThrownBy(() -> service().consume(secondBinding, second.proof())).isInstanceOf(ProofDeniedException.class);
    var thirdBinding = new ProofBinding(account, binding.profileId(), binding.spaceId(), binding.newOwnerProfileId(), UUID.randomUUID(), 7);
    var third = service().issue(thirdBinding, "correct", "", replacement.getFirst());
    assertThat(service().consume(thirdBinding, third.proof()).binding()).isEqualTo(thirdBinding);
  }

  @Test void exactCommittedReceiptSurvivesSecurityRevocationButChangedBindingNeverReplays() {
    var issued = service().issue(binding, "correct", "", "");
    var receipt = service().consume(binding, issued.proof());
    update("password_hash='changed'");
    assertThat(service(store, now.plusSeconds(301)).consume(binding, issued.proof())).isEqualTo(receipt);
    var changed = new ProofBinding(account, binding.profileId(), binding.spaceId(), UUID.randomUUID(), binding.operationId(), 7);
    assertThatThrownBy(() -> service().consume(changed, issued.proof())).isInstanceOf(ProofDeniedException.class);
    assertThatThrownBy(() -> service().consume(binding, issued.proof() + "x")).isInstanceOf(ProofDeniedException.class);
  }

  private OwnershipTransferProofService independentService() {
    var source = new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
    var connection = new NamedParameterJdbcTemplate(source);
    return new OwnershipTransferProofService(
        new JdbcOwnershipTransferProofStore(connection, new DataSourceTransactionManager(source)),
        passwords, totp, new BackupCodeService(new JdbcBackupCodeRepository(connection)), floors,
        Clock.fixed(now, ZoneOffset.UTC));
  }

  @Test void persistedRowAndIssuedDiagnosticNeverExposePlaintextProof() {
    var issued = service().issue(binding, "correct", "", "");
    String row = jdbc.queryForObject(
        "SELECT row_to_json(p)::text FROM ownership_transfer_proofs p WHERE operation_id=:id",
        Map.of("id", binding.operationId()), String.class);
    assertThat(row).isNotNull().doesNotContain(issued.proof());
    assertThat(issued.toString()).doesNotContain(issued.proof()).containsIgnoringCase("redacted");
  }

  @ParameterizedTest
  @org.junit.jupiter.params.provider.ValueSource(booleans = {false, true})
  void securityMutationWaitsForIssueOrConsumeAccountTransaction(boolean consuming) throws Exception {
    var initial = service().issue(binding, "correct", "", "");
    var requested = consuming ? binding : new ProofBinding(account, binding.profileId(), binding.spaceId(),
        binding.newOwnerProfileId(), UUID.randomUUID(), 7);
    var locked = new CountDownLatch(1);
    var release = new CountDownLatch(1);
    OwnershipTransferProofStore pausedStore = new OwnershipTransferProofStore() {
      @Override public <T> T withAccount(UUID id, Function<Session, T> action) {
        return store.withAccount(id, session -> {
          locked.countDown();
          try {
            if (!release.await(15, TimeUnit.SECONDS)) throw new AssertionError("test did not release account lock");
          } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            throw new AssertionError(interrupted);
          }
          return action.apply(session);
        });
      }
    };
    var workers = Executors.newFixedThreadPool(2);
    try (var writer = java.sql.DriverManager.getConnection(
        postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())) {
      int writerPid;
      try (var statement = writer.createStatement(); var result = statement.executeQuery("SELECT pg_backend_pid()")) {
        result.next();
        writerPid = result.getInt(1);
      }
      var operation = workers.submit(() -> consuming
          ? service(pausedStore, now).consume(requested, initial.proof())
          : service(pausedStore, now).issue(requested, "correct", "", ""));
      assertThat(locked.await(10, TimeUnit.SECONDS)).isTrue();
      var mutation = workers.submit(() -> {
        try (var statement = writer.prepareStatement("UPDATE accounts SET password_hash='changed' WHERE id=?")) {
          statement.setObject(1, account);
          return statement.executeUpdate();
        }
      });
      // Observe an actual PostgreSQL lock wait, not just a worker that has not started yet.
      org.awaitility.Awaitility.await().atMost(Duration.ofSeconds(10)).untilAsserted(() ->
          assertThat(jdbc.queryForObject("SELECT cardinality(pg_blocking_pids(:pid))",
              Map.of("pid", writerPid), Integer.class)).isGreaterThan(0));
      assertThat(mutation).isNotDone();
      release.countDown();
      Object result = operation.get(10, TimeUnit.SECONDS);
      assertThat(mutation.get(10, TimeUnit.SECONDS)).isEqualTo(1);
      if (consuming) {
        assertThat(independentService().consume(binding, initial.proof())).isEqualTo(result);
      } else {
        var issued = (OwnershipTransferProofService.IssuedProof) result;
        assertThatThrownBy(() -> independentService().consume(requested, issued.proof()))
            .isInstanceOf(ProofDeniedException.class);
      }
      assertThatThrownBy(() -> independentService().issue(
          new ProofBinding(account, binding.profileId(), binding.spaceId(), binding.newOwnerProfileId(), UUID.randomUUID(), 7),
          "correct", "", "")).isInstanceOf(ProofDeniedException.class);
    } finally {
      release.countDown();
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }
  @Test void fractionalClockReceiptIsIdenticalAfterDatabaseRoundTrip() {
    Instant fractional = now.plusNanos(123456789);
    var issued = service(store, fractional).issue(binding, "correct", "", "");
    var receipt = service(store, fractional).consume(binding, issued.proof());
    assertThat(independentService().consume(binding, issued.proof())).isEqualTo(receipt);
    assertThat(receipt.consumedAt().getNano() % 1000).isZero();
  }

  @Test void existingBackupConsumeTakesAccountLockBeforeBackupRowDuringProofIssue() throws Exception {
    update("totp_enabled=TRUE,totp_secret=decode('01','hex')");
    String code = backup.generateAndStore(account).getFirst();
    var locked = new CountDownLatch(1);
    var release = new CountDownLatch(1);
    OwnershipTransferProofStore paused = new OwnershipTransferProofStore() {
      public <T> T withAccount(UUID id, Function<Session,T> action) {
        return store.withAccount(id, session -> {
          locked.countDown();
          try { if (!release.await(15, TimeUnit.SECONDS)) throw new AssertionError("lock release timeout"); }
          catch (InterruptedException interrupted) { Thread.currentThread().interrupt(); throw new AssertionError(interrupted); }
          return action.apply(session);
        });
      }
    };
    var workers = Executors.newFixedThreadPool(2);
    try (var connection = java.sql.DriverManager.getConnection(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())) {
      int pid;
      try (var statement = connection.createStatement(); var rows = statement.executeQuery("SELECT pg_backend_pid()")) {
        rows.next(); pid = rows.getInt(1);
      }
      var directJdbc = new NamedParameterJdbcTemplate(new org.springframework.jdbc.datasource.SingleConnectionDataSource(connection, true));
      var directBackup = new BackupCodeService(new JdbcBackupCodeRepository(directJdbc));
      var issuing = workers.submit(() -> service(paused, now).issue(binding, "correct", "", code));
      assertThat(locked.await(10, TimeUnit.SECONDS)).isTrue();
      var consuming = workers.submit(() -> directBackup.consume(account, code));
      org.awaitility.Awaitility.await().atMost(Duration.ofSeconds(10)).untilAsserted(() ->
          assertThat(jdbc.queryForObject("SELECT cardinality(pg_blocking_pids(:pid))", Map.of("pid", pid), Integer.class)).isGreaterThan(0));
      release.countDown();
      var issued = issuing.get(15, TimeUnit.SECONDS);
      assertThat(consuming.get(15, TimeUnit.SECONDS)).isFalse();
      assertThat(service().consume(binding, issued.proof()).binding()).isEqualTo(binding);
    } finally {
      release.countDown(); workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }
  @Test void failedExistingBackupReplacementRollsBackDeletionAndSecurityRevision() {
    var codes = backup.generateAndStore(account);
    var issued = service().issue(binding, "correct", "", "");
    var repository = new JdbcBackupCodeRepository(jdbc);
    assertThatThrownBy(() -> repository.replaceCodes(account, java.util.Arrays.asList("a".repeat(64), null)))
        .isInstanceOf(org.springframework.dao.DataIntegrityViolationException.class);
    assertThat(service().consume(binding, issued.proof())).isNotNull();
    assertThat(backup.consume(account, codes.getFirst())).isTrue();
    assertThat(jdbc.queryForObject("SELECT count(*) FROM backup_codes WHERE account_id=:id", Map.of("id", account), Long.class))
        .isEqualTo(10L);
  }
  private void update(String assignment) {
    jdbc.update("UPDATE accounts SET " + assignment + " WHERE id=:id", Map.of("id", account));
  }

  /** Fail after an actual SQL write, inside the real store's transaction. */
  private OwnershipTransferProofStore failAfterWrite(boolean insert) {
    return new OwnershipTransferProofStore() {
      @Override public <T> T withAccount(UUID id, Function<Session, T> action) {
        return store.withAccount(id, session -> action.apply(new Session() {
          public ProofAccount account() { return session.account(); }
          public Optional<StoredProof> find(UUID operation) { return session.find(operation); }
          public void insert(StoredProof proof) {
            session.insert(proof);
            if (insert) throw new InjectedFailure();
          }
          public void markConsumed(UUID operation, Instant at) {
            session.markConsumed(operation, at);
            if (!insert) throw new InjectedFailure();
          }
        }));
      }
    };
  }

  private static <T> List<T> results(List<Future<T>> futures) {
    return futures.stream().map(future -> {
      try { return future.get(5, TimeUnit.SECONDS); }
      catch (Exception failure) { throw new AssertionError("concurrent proof operation failed", failure); }
    }).toList();
  }

  private static final class InjectedFailure extends RuntimeException {}
}
