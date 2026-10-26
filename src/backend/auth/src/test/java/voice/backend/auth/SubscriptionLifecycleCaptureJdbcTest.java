package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.time.Instant;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.Callable;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.JdbcAccountDeletionOperationRepository;
import voice.backend.auth.repository.JdbcAccountRepository;
import voice.backend.auth.service.TransactionalAccountDeletionOperationStarter;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException;
import voice.backend.auth.roleidentityauthority.AuthIdentityDatabasePolicy;
import voice.backend.auth.roleidentityauthority.AuthSubscriptionLifecycleOwnerStore;

/** Actual Auth transaction producers and PostgreSQL; no Subscription eligibility claim. */
@Testcontainers
class SubscriptionLifecycleCaptureJdbcTest {
  private static DriverManagerDataSource database;
  @Container static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("subscription_lifecycle_capture")
          .withUsername("voice").withPassword("voice");

  @BeforeAll static void migrate() {
    database = new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
    Flyway.configure().dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .locations("classpath:db/migration").load().migrate();
    new JdbcTemplate(database).execute("CREATE ROLE subscription_owner LOGIN PASSWORD 'fixture-only'");
    new JdbcTemplate(database).execute("GRANT USAGE ON SCHEMA public TO subscription_owner");
    new JdbcTemplate(database).execute("GRANT SELECT,UPDATE ON accounts,account_deletion_operations,account_subscription_lifecycle_heads,account_subscription_lifecycle_outbox TO subscription_owner");
    new JdbcTemplate(database).execute("GRANT SELECT ON flyway_schema_history TO subscription_owner");
  }

  DriverManagerDataSource source() {
    return database;
  }
  JdbcTemplate jdbc() { return new JdbcTemplate(source()); }
  JdbcAccountRepository accounts() { return new JdbcAccountRepository(new NamedParameterJdbcTemplate(source())); }
  TransactionTemplate transactions() { return new TransactionTemplate(new DataSourceTransactionManager(source())); }
  SessionEpochFloorStore floors() {
    var floors = mock(SessionEpochFloorStore.class);
    when(floors.recordAtLeast(any(), anyLong())).thenAnswer(call -> call.getArgument(1));
    return floors;
  }
  TransactionalAccountDeletionOperationStarter starter(SessionEpochFloorStore floors) {
    return new TransactionalAccountDeletionOperationStarter(transactions(), accounts(),
        new JdbcAccountDeletionOperationRepository(new NamedParameterJdbcTemplate(source())), floors);
  }
  Account account() {
    return accounts().create(UUID.randomUUID()+"@fixture.invalid", null, "fixture-hash", "regular");
  }
  AuthSubscriptionLifecycleOwnerStore owner() {
    return new AuthSubscriptionLifecycleOwnerStore(
        new DriverManagerDataSource(postgres.getJdbcUrl(), "subscription_owner", "fixture-only"),
        new AuthIdentityDatabasePolicy(postgres.getUsername(), AuthIdentityDatabasePolicy.Loader.FLYWAY));
  }

  @Test void ownerReturnsFullSourceStateAndNeverCertifiesUnrecordedAccount() {
    var account = account();
    var snapshot = owner().observe(account.id(), 0);
    assertThat(snapshot.accountId()).isEqualTo(account.id());
    assertThat(snapshot.status()).isEqualTo("active");
    assertThat(snapshot.accountType()).isEqualTo("regular");
    assertThat(snapshot.sessionEpoch()).isEqualTo(account.sessionEpoch());
    assertThat(snapshot.securityRevision()).isPositive();
    assertThat(snapshot.coverage()).isEqualTo(AuthSubscriptionLifecycleOwnerStore.Coverage.UNRECORDED);
    assertThat(snapshot.facts()).isEmpty();
    assertThat(snapshot.observedAt()).isNotNull();
    assertThatThrownBy(() -> owner().observe(account.id(), 1)).isInstanceOf(IllegalStateException.class);
    assertThatThrownBy(() -> owner().observe(UUID.randomUUID(), 0)).isInstanceOf(IllegalStateException.class);
  }

  @Test void ownerReturnsExactOrderedHeadAndRestoreInputWithoutPaidGrant() {
    var account = account();
    UUID cycle = UUID.randomUUID();
    var deleted = starter(floors()).startOrResume(account, cycle, "fixture-token-hash", Instant.now());
    var before = owner().observe(account.id(), 1);
    assertThat(before.status()).isEqualTo("deleted");
    assertThat(before.deletedAt()).isEqualTo(deleted.account().deletedAt());
    assertThat(before.coverage()).isEqualTo(AuthSubscriptionLifecycleOwnerStore.Coverage.CAPTURED);
    assertThat(before.facts()).hasSize(1);
    assertThat(before.head()).isEqualTo(before.facts().getLast());
    assertThat(before.head().deletionCycleId()).isEqualTo(cycle);
    assertThat(accounts().restoreDeleted(account.id())).isTrue();
    var after = owner().observe(account.id(), 1);
    assertThat(after.status()).isEqualTo("active");
    assertThat(after.deletedAt()).isNull();
    assertThat(after.facts()).hasSize(2);
    assertThat(after.head().kind()).isEqualTo("RESTORE_REQUESTED");
    assertThat(after.head().sourceRevision()).isEqualTo(2);
    assertThat(after.head().purgeAt()).isEqualTo(before.head().scheduledAt().plusSeconds(720L*3600));
    assertThatThrownBy(() -> owner().observe(account.id(), 3)).isInstanceOf(IllegalStateException.class);
  }

  @Test void ownerRejectsLostFactInsteadOfReturningPartialCleanState() {
    var account = account();
    starter(floors()).startOrResume(account, UUID.randomUUID(), "fixture-token-hash", Instant.now());
    jdbc().update("DELETE FROM account_subscription_lifecycle_outbox WHERE account_id=?", account.id());
    assertThatThrownBy(() -> owner().observe(account.id(), 0)).isInstanceOf(IllegalStateException.class);
  }

  @Test void ownerRejectsChangedCaptureInsteadOfTrustingMigrationLabel() {
    var account = account();
    for(String trigger:List.of("auth_subscription_restore_capture","accounts_security_revision")) {
    // Actual transactional DDL through the installed Auth maintenance barrier.
    transactions().executeWithoutResult(tx -> {
      jdbc().execute("LOCK TABLE ONLY auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE");
      jdbc().execute("ALTER TABLE accounts DISABLE TRIGGER "+trigger);
    });
    try {
      assertThatThrownBy(() -> owner().observe(account.id(), 0)).isInstanceOf(IllegalStateException.class);
    } finally {
      transactions().executeWithoutResult(tx -> {
        jdbc().execute("LOCK TABLE ONLY auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE");
        jdbc().execute("ALTER TABLE accounts ENABLE TRIGGER "+trigger);
      });
    }
    }
    assertThat(owner().observe(account.id(), 0).status()).isEqualTo("active");
  }

  @Test void ownerRejectsInstallerSessionAndSchemaCapabilityDrift() {
    var account = account();
    var installer = new AuthSubscriptionLifecycleOwnerStore(source(),new AuthIdentityDatabasePolicy(postgres.getUsername(),AuthIdentityDatabasePolicy.Loader.FLYWAY));
    assertThatThrownBy(() -> installer.observe(account.id(),0)).isInstanceOf(IllegalStateException.class);
    jdbc().execute("GRANT CREATE ON SCHEMA public TO subscription_owner");
    try { assertThatThrownBy(() -> owner().observe(account.id(),0)).isInstanceOf(IllegalStateException.class); }
    finally { jdbc().execute("REVOKE CREATE ON SCHEMA public FROM subscription_owner"); }
    assertThat(owner().observe(account.id(),0).accountId()).isEqualTo(account.id());
  }

  @Test void ownerWaitsForActualRestoreAccountLockThenReadsCoherentCommittedFacts() throws Exception {
    var account = account();
    starter(floors()).startOrResume(account, UUID.randomUUID(), "fixture-token-hash", Instant.now());
    var restored = new java.util.concurrent.CountDownLatch(1);
    var release = new java.util.concurrent.CountDownLatch(1);
    var pid = new java.util.concurrent.atomic.AtomicInteger();
    var executor = Executors.newFixedThreadPool(2);
    try {
      var write = executor.submit(() -> transactions().execute(tx -> {
        assertThat(accounts().restoreDeleted(account.id())).isTrue();
        pid.set(jdbc().queryForObject("SELECT pg_backend_pid()", Integer.class));
        restored.countDown();
        try { assertThat(release.await(10, TimeUnit.SECONDS)).isTrue(); }
        catch (InterruptedException interrupted) { Thread.currentThread().interrupt(); throw new IllegalStateException(interrupted); }
        return true;
      }));
      assertThat(restored.await(10, TimeUnit.SECONDS)).isTrue();
      var read = executor.submit(() -> owner().observe(account.id(), 1));
      boolean blocked = false;
      long deadline = System.nanoTime()+TimeUnit.MILLISECONDS.toNanos(350);
      while (!blocked && System.nanoTime()<deadline) {
        blocked = Boolean.TRUE.equals(jdbc().queryForObject("SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE usename='subscription_owner' AND ?=ANY(pg_blocking_pids(pid)))", Boolean.class, pid.get()));
        if (!blocked) Thread.sleep(5);
      }
      assertThat(blocked).as("real owner observation must wait on the source account mutex").isTrue();
      release.countDown();
      assertThat(write.get(10, TimeUnit.SECONDS)).isTrue();
      var observed = read.get(10, TimeUnit.SECONDS);
      assertThat(observed.status()).isEqualTo("active");
      assertThat(observed.deletedAt()).isNull();
      assertThat(observed.facts()).hasSize(2);
      assertThat(observed.head()).isEqualTo(observed.facts().getLast());
      assertThat(observed.head().kind()).isEqualTo("RESTORE_REQUESTED");
    } finally { release.countDown(); executor.shutdownNow(); }
  }
  List<java.util.Map<String,Object>> facts(UUID account) {
    return jdbc().queryForList("SELECT * FROM account_subscription_lifecycle_outbox WHERE account_id=? ORDER BY source_revision", account);
  }

  @Test void realDeletionTransactionCapturesOneStableCompleteFactAndReplayDoesNotAppend() {
    var account = account();
    UUID operation = UUID.randomUUID();
    var starter = starter(floors());
    var started = starter.startOrResume(account, operation, "fixture-token-hash", Instant.now());
    var captured = facts(account.id());
    assertThat(captured).hasSize(1);
    var fact = captured.getFirst();
    assertThat(fact.get("event_id")).isEqualTo(operation);
    assertThat(fact.get("deletion_cycle_id")).isEqualTo(operation);
    assertThat(fact.get("kind")).isEqualTo("DELETE_SCHEDULED");
    assertThat(fact.get("source_revision")).isEqualTo(1L);
    assertThat(fact.get("session_epoch")).isEqualTo(started.account().sessionEpoch());
    assertThat(((java.sql.Timestamp)fact.get("scheduled_at")).toInstant()).isEqualTo(started.account().deletedAt());
    assertThat(((java.sql.Timestamp)fact.get("purge_at")).toInstant())
        .isEqualTo(started.account().deletedAt().plusSeconds(30L*24*60*60));
    starter.startOrResume(started.account(), UUID.randomUUID(), "not-a-new-operation", Instant.now());
    assertThat(facts(account.id())).isEqualTo(captured);
  }

  @Test void floorFailureRollsBackAccountDeletionOperationAndCapturedFactTogether() {
    var account = account();
    var floors = floors();
    doThrow(new SessionEpochFloorUnavailableException("fixture outage"))
        .when(floors).recordAtLeast(any(), anyLong());
    assertThatThrownBy(() -> starter(floors).startOrResume(account, UUID.randomUUID(), "fixture-token-hash", Instant.now()))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
    assertThat(accounts().findById(account.id().toString()).orElseThrow().deletedAt()).isNull();
    assertThat(facts(account.id())).isEmpty();
    assertThat(jdbc().queryForObject("SELECT count(*) FROM account_deletion_operations WHERE account_id=?", Long.class, account.id())).isZero();
  }

  @Test void restoreFactCommitsWithActualConditionalRestoreAndRollbackKeepsDeletion() {
    var account = account();
    UUID cycle = UUID.randomUUID();
    starter(floors()).startOrResume(account, cycle, "fixture-token-hash", Instant.now());
    assertThatThrownBy(() -> transactions().execute(tx -> {
      assertThat(accounts().restoreDeleted(account.id())).isTrue();
      assertThat(facts(account.id())).hasSize(2);
      throw new IllegalStateException("fixture rollback");
    })).isInstanceOf(IllegalStateException.class);
    assertThat(accounts().findById(account.id().toString()).orElseThrow().deletedAt()).isNotNull();
    assertThat(facts(account.id())).hasSize(1);
    assertThat(accounts().restoreDeleted(account.id())).isTrue();
    var captured = facts(account.id());
    assertThat(captured).hasSize(2);
    assertThat(captured.getLast().get("source_revision")).isEqualTo(2L);
    assertThat(captured.getLast().get("kind")).isEqualTo("RESTORE_REQUESTED");
    assertThat(captured.getLast().get("deletion_cycle_id")).isEqualTo(cycle);
    assertThat(captured.getLast().get("event_id")).isNotEqualTo(cycle);
    assertThat(accounts().restoreDeleted(account.id())).isFalse();
    assertThat(facts(account.id())).isEqualTo(captured);
  }

  @Test void twoRealConnectionsRaceRestoreButAppendOneSuccessor() throws Exception {
    var account = account();
    starter(floors()).startOrResume(account, UUID.randomUUID(), "fixture-token-hash", Instant.now());
    var start = new CyclicBarrier(2);
    var executor = Executors.newFixedThreadPool(2);
    try {
      Callable<Boolean> restore = () -> { start.await(10, TimeUnit.SECONDS); return accounts().restoreDeleted(account.id()); };
      var results = executor.invokeAll(List.of(restore, restore), 30, TimeUnit.SECONDS);
      assertThat(results.stream().map(result -> {
        try { return result.get(); } catch (Exception failure) { throw new AssertionError(failure); }
      }).toList()).containsExactlyInAnyOrder(true, false);
      assertThat(facts(account.id())).hasSize(2);
    } finally {
      executor.shutdownNow();
      assertThat(executor.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test void legacyRestoreWithoutProvenDeletionCycleWorksButCannotInventAuthorityFact() {
    var account = account();
    accounts().markDeleted(account.id(), Instant.now());
    assertThat(accounts().restoreDeleted(account.id())).isTrue();
    assertThat(facts(account.id())).isEmpty();
  }

  @Test void coordinatorReceiptCleanupCannotErasePendingFactsOrResetRevision() {
    var account = account();
    UUID cycle = UUID.randomUUID();
    starter(floors()).startOrResume(account, cycle, "fixture-token-hash", Instant.now());
    assertThat(accounts().restoreDeleted(account.id())).isTrue();
    var captured = facts(account.id());
    jdbc().update("DELETE FROM account_deletion_operations WHERE operation_id=?", cycle);
    assertThat(facts(account.id())).as("undelivered facts have their own retention, not coordinator-receipt retention")
        .isEqualTo(captured);
    var live = accounts().findById(account.id().toString()).orElseThrow();
    starter(floors()).startOrResume(live, UUID.randomUUID(), "next-token-hash", Instant.now());
    assertThat(facts(account.id()).getLast().get("source_revision")).isEqualTo(3L);
  }

  @Test void purgeDeadlineIsFixedP30DEvenAcrossSessionTimezoneDst() {
    var account = account();
    Instant scheduled = Instant.parse("2026-10-20T00:00:00Z");
    transactions().execute(tx -> {
      jdbc().execute("SET LOCAL TIME ZONE 'Europe/Berlin'");
      starter(floors()).startOrResume(account, UUID.randomUUID(), "fixture-token-hash", scheduled);
      return null;
    });
    assertThat(((java.sql.Timestamp)facts(account.id()).getFirst().get("purge_at")).toInstant())
        .isEqualTo(scheduled.plusSeconds(30L*24*60*60));
  }

  @Test void storedFactBodyCannotBeChangedWhileLeaseMetadataCanAdvance() {
    var account = account();
    UUID cycle = UUID.randomUUID();
    starter(floors()).startOrResume(account, cycle, "fixture-token-hash", Instant.now());
    var captured = facts(account.id());
    assertThatThrownBy(() -> jdbc().update("UPDATE account_subscription_lifecycle_outbox SET kind='RESTORE_REQUESTED' WHERE event_id=?", cycle))
        .isInstanceOf(org.springframework.dao.DataAccessException.class);
    assertThat(facts(account.id())).isEqualTo(captured);
    jdbc().update("UPDATE account_subscription_lifecycle_outbox SET attempts=attempts+1 WHERE event_id=?", cycle);
    assertThat(facts(account.id()).getFirst().get("attempts")).isEqualTo(1);
    assertThat(facts(account.id()).getFirst().get("kind")).isEqualTo("DELETE_SCHEDULED");
  }

  @Test void actualOwningAuthReaderAcceptsExactNewHistoryAndRejectsChangedChecksum() {
    try (var reader = new voice.backend.auth.authoritysource.AuthAuthorityReader(source())) {
      reader.checkSchema();
      Integer checksum = jdbc().queryForObject("SELECT checksum FROM flyway_schema_history WHERE version='33'", Integer.class);
      jdbc().update("UPDATE flyway_schema_history SET checksum=checksum+1 WHERE version='33'");
      try {
        assertThatThrownBy(reader::checkSchema).isInstanceOf(IllegalStateException.class);
      } finally {
        jdbc().update("UPDATE flyway_schema_history SET checksum=? WHERE version='33'", checksum);
      }
      reader.checkSchema();
    }
  }
}
