package voice.backend.auth.roleidentityauthority;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.when;

import java.nio.charset.StandardCharsets;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import java.util.UUID;
import javax.sql.DataSource;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Order;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestMethodOrder;
import org.junit.jupiter.api.MethodOrderer.OrderAnnotation;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Import;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AccountDeletionOperationStarter;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.support.JdbcUserContractTestConfiguration;

@SpringBootTest
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
@Import({JdbcUserContractTestConfiguration.class, AuthAccountLifecycleProducerJdbcIntegrationTest.Transactions.class})
@TestMethodOrder(OrderAnnotation.class)
class AuthAccountLifecycleProducerJdbcIntegrationTest {
  @Container static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db").withUsername("voice").withPassword("voice")
          .withLabel("voice.fixture","auth-account-lifecycle-producer")
          .withLabel("voice.actor",System.getenv().getOrDefault("VOICE_TEST_ACTOR","ci-auth-lifecycle"));

  @DynamicPropertySource static void databaseProperties(DynamicPropertyRegistry properties) {
    properties.add("voice.auth.jdbc.url",postgres::getJdbcUrl);
    properties.add("spring.datasource.username",postgres::getUsername);
    properties.add("spring.datasource.password",postgres::getPassword);
    properties.add("spring.flyway.user",postgres::getUsername);
    properties.add("spring.flyway.password",postgres::getPassword);
  }

  @Autowired AuthService auth;
  @Autowired AccountRepository accounts;
  @Autowired AccountDeletionOperationStarter starter;
  @Autowired DataSource source;
  @Autowired PlatformTransactionManager manager;
  @MockBean SessionEpochFloorStore floors;
  @MockBean TokenBlacklist blacklist;
  JdbcTemplate jdbc;
  TransactionTemplate transactions;
  AuthAccountLifecycleCycleStore cycles;

  @BeforeEach void setup() {
    when(floors.recordAtLeast(any(),anyLong())).thenAnswer(call -> call.getArgument(1));
    jdbc = new JdbcTemplate(source);
    transactions = new TransactionTemplate(manager);
    transactions.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);
    cycles = new AuthAccountLifecycleCycleStore(source);
  }

  @Test @Order(1) void storedRecordControlUsesTheRealSqlQuery() {
    Account account = account();
    UUID owner = owner(account), cycle = UUID.randomUUID(), intent = UUID.randomUUID();
    Instant scheduled = now(), purge = scheduled.plus(Duration.ofDays(30));
    // A decoding control only: this administrator-seeded row is not producer
    // provenance, a currentness grant, or evidence of a historical transition.
    byte[] bytes = "{\"fixture\":\"stored-record-decoding\"}".getBytes(StandardCharsets.UTF_8);
    transactions.executeWithoutResult(status -> {
      jdbc.update("""
          INSERT INTO auth_account_lifecycle_schedules
            (owner_handle,deletion_cycle_id,schedule_revision,scheduled_at,purge_at,intent_id)
          VALUES(?,?,1,?,?,?)
          """,owner,cycle,Timestamp.from(scheduled),Timestamp.from(purge),intent);
      jdbc.update("""
          INSERT INTO auth_account_lifecycle_intents
            (event_id,kind,owner_handle,deletion_cycle_id,lifecycle_revision,event_bytes,event_sha256,created_at)
          VALUES(?,'DELETE_SCHEDULED',?,?,1,?,sha256(?),?)
          """,intent,owner,cycle,bytes,bytes,Timestamp.from(scheduled));
    });
    var stored = readCycle(owner,cycle).orElseThrow();
    assertThat(stored.schedule().ownerHandle()).isEqualTo(owner);
    assertThat(stored.schedule().cycle()).isEqualTo(cycle);
    assertThat(stored.schedule().scheduledAt()).isEqualTo(scheduled);
    assertThat(stored.schedule().purgeAt()).isEqualTo(purge);
    assertThat(stored.decision()).isEmpty();
  }

  @Test @Order(2) void realDeleteThenRestoreRetainsTheQueriedOldCycleAndIntents() {
    Account account = account();
    var deletion = starter.startOrResume(account,UUID.randomUUID(),"fixture-restore-token-hash",now());
    UUID owner = owner(account);
    var original = jdbc.queryForMap("SELECT deletion_cycle,lifecycle_revision,erase_at FROM role_identity_lifetimes WHERE owner_handle=?",owner);
    UUID cycle = (UUID)original.get("deletion_cycle");
    long revision = ((Number)original.get("lifecycle_revision")).longValue();
    Instant purge = ((Timestamp)original.get("erase_at")).toInstant();
    assertThat(cycle).isNotNull();
    Boolean restored = transactions.execute(status -> accounts.restoreDeleted(account.id()));
    assertThat(restored).isTrue();
    assertThat(jdbc.queryForObject("SELECT state FROM role_identity_lifetimes WHERE owner_handle=?",String.class,owner)).isEqualTo("ACTIVE");
    assertThat(jdbc.queryForObject("SELECT deletion_cycle IS NULL FROM role_identity_lifetimes WHERE owner_handle=?",Boolean.class,owner)).isTrue();
    var recorded = readCycle(owner,cycle);
    assertThat(recorded).as("actual committed delete/restore must preserve its queried old cycle").isPresent();
    var fact = recorded.orElseThrow();
    assertThat(fact.schedule().revision()).isEqualTo(revision);
    assertThat(fact.schedule().scheduledAt()).isEqualTo(deletion.account().deletedAt());
    assertThat(fact.schedule().purgeAt()).isEqualTo(purge);
    var decision = fact.decision().orElseThrow();
    assertThat(decision.kind()).isEqualTo(AuthAccountLifecycleCycleStore.DecisionKind.RESTORED);
    assertThat(decision.revision()).isGreaterThan(revision);
    assertThat(decision.at()).isBefore(purge);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM auth_account_lifecycle_intents WHERE owner_handle=? AND deletion_cycle_id=?",Integer.class,owner,cycle)).isEqualTo(2);
  }

  @Test @Order(3) void sourceQueryRequiresItsOwnTransaction() {
    assertThatThrownBy(() -> cycles.find(UUID.randomUUID(),UUID.randomUUID()))
        .isInstanceOf(IllegalStateException.class).hasMessageContaining("owning transaction");
  }

  @Test @Order(4) void intentFailureRollsBackTheActualDeleteAndRestore() {
    Account first = account();
    failIntent("DELETE_SCHEDULED");
    try {
      assertThatThrownBy(() -> starter.startOrResume(first,UUID.randomUUID(),"fixture",now()))
          .isInstanceOf(RuntimeException.class);
    } finally { clearIntentFailure(); }
    assertThat(accounts.findById(first.id().toString()).orElseThrow().deletedAt()).isNull();
    assertThat(count("auth_account_lifecycle_schedules",owner(first))).isZero();
    assertThat(jdbc.queryForObject("SELECT count(*) FROM account_deletion_operations WHERE account_id=?",Integer.class,first.id())).isZero();

    Account second = account();
    starter.startOrResume(second,UUID.randomUUID(),"fixture",now());
    UUID owner = owner(second), cycle = currentCycle(owner);
    failIntent("RESTORED");
    try {
      assertThatThrownBy(() -> transactions.execute(status -> accounts.restoreDeleted(second.id())))
          .isInstanceOf(RuntimeException.class);
    } finally { clearIntentFailure(); }
    assertThat(accounts.findById(second.id().toString()).orElseThrow().deletedAt()).isNotNull();
    var retained = readCycle(owner,cycle).orElseThrow();
    assertThat(retained.decision()).isEmpty();
    assertThat(count("auth_account_lifecycle_intents",owner)).isEqualTo(1);
    Boolean restored = transactions.execute(status -> accounts.restoreDeleted(second.id()));
    assertThat(restored).isTrue();
  }

  @Test @Order(5) void duplicateDeleteAndRestoreDoNotReplaceFactsOrIntentIdentity() {
    Account account = account(); UUID operation = UUID.randomUUID();
    var first = starter.startOrResume(account,operation,"fixture",now());
    UUID owner = owner(account), cycle = currentCycle(owner);
    var bytes = jdbc.queryForList("SELECT event_id,event_bytes,event_sha256 FROM auth_account_lifecycle_intents WHERE owner_handle=?",owner);
    var repeated = starter.startOrResume(account,operation,"fixture",now().plus(Duration.ofDays(7)));
    assertThat(repeated.operation().operationId()).isEqualTo(first.operation().operationId());
    assertThat(currentCycle(owner)).isEqualTo(cycle);
    assertThat(count("auth_account_lifecycle_intents",owner)).isEqualTo(1);
    var again = jdbc.queryForMap("SELECT event_id,event_bytes,event_sha256 FROM auth_account_lifecycle_intents WHERE owner_handle=?",owner);
    assertThat(again.get("event_id")).isEqualTo(bytes.getFirst().get("event_id"));
    assertThat((byte[])again.get("event_bytes")).isEqualTo((byte[])bytes.getFirst().get("event_bytes"));
    Boolean restored = transactions.execute(status -> accounts.restoreDeleted(account.id()));
    assertThat(restored).isTrue();
    var decision = readCycle(owner,cycle).orElseThrow();
    Boolean replayed = transactions.execute(status -> accounts.restoreDeleted(account.id()));
    assertThat(replayed).isFalse();
    assertThat(readCycle(owner,cycle)).contains(decision);
    assertThat(count("auth_account_lifecycle_intents",owner)).isEqualTo(2);
    // A later cycle must not overwrite the old decision, even though current
    // state no longer identifies that old cycle. Direct owning mutation avoids
    // conflating this with the legacy operation store's restore policy.
    transactions.executeWithoutResult(status -> jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp(),status='deleted' WHERE id=?",account.id()));
    UUID next = currentCycle(owner);
    assertThat(next).isNotEqualTo(cycle);
    assertThat(readCycle(owner,cycle)).contains(decision);
    var nextFact = transactions.execute(status -> cycles.find(owner,next)).orElseThrow();
    assertThat(nextFact.schedule().revision()).isGreaterThan(decision.decision().orElseThrow().revision());
    assertThat(nextFact.decision()).isEmpty();
    assertThat(count("auth_account_lifecycle_intents",owner)).isEqualTo(3);
  }

  @Test @Order(6) void expiredRestoreIsDeniedAndTerminalFenceSurvivesRawAccountErasure() {
    Account account = account(); UUID owner = owner(account);
    // Compressed-time boundary control via a real owning mutation, not a
    // claimed production backfill or evidence of physical deletion at D.
    transactions.executeWithoutResult(status -> jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp()-interval '720 hours',status='deleted' WHERE id=?",account.id()));
    UUID cycle = currentCycle(owner);
    var fact = readCycle(owner,cycle).orElseThrow();
    assertThat(fact.decision().orElseThrow().kind()).isEqualTo(AuthAccountLifecycleCycleStore.DecisionKind.LIFETIME_TERMINAL);
    assertThat(fact.decision().orElseThrow().at()).isAfterOrEqualTo(fact.schedule().purgeAt());
    Boolean restored = transactions.execute(status -> accounts.restoreDeleted(account.id()));
    assertThat(restored).isFalse();
    transactions.executeWithoutResult(status -> jdbc.update("DELETE FROM accounts WHERE id=?",account.id()));
    assertThat(readCycle(owner,cycle)).contains(fact);
    assertThat(count("auth_account_lifecycle_intents",owner)).isEqualTo(2);
    var events = jdbc.queryForList("SELECT convert_from(event_bytes,'UTF8') AS payload FROM auth_account_lifecycle_intents WHERE owner_handle=?",owner);
    for (var event : events) assertThat(event.get("payload").toString()).doesNotContain(account.id().toString(),"account_id","profile_id","ACCOUNT_PURGED");
  }

  @Test @Order(7) void equalityIsNotAValidRestoredDecisionAndFactsCannotBeRewritten() {
    Account account = account(); UUID owner = owner(account);
    starter.startOrResume(account,UUID.randomUUID(),"fixture",now());
    UUID cycle = currentCycle(owner);
    var original = readCycle(owner,cycle).orElseThrow();
    assertThatThrownBy(() -> transactions.executeWithoutResult(status -> jdbc.queryForObject(
        "SELECT auth_account_lifecycle_decide(?,?,'RESTORED',?,?)",Object.class,owner,cycle,
        original.schedule().revision()+1,Timestamp.from(original.schedule().purgeAt()))))
        .isInstanceOf(org.springframework.dao.DataIntegrityViolationException.class);
    assertThatThrownBy(() -> jdbc.update("UPDATE auth_account_lifecycle_schedules SET purge_at=purge_at+interval '1 day' WHERE owner_handle=?",owner))
        .isInstanceOf(RuntimeException.class);
    assertThatThrownBy(() -> jdbc.update("UPDATE auth_account_lifecycle_intents SET event_id=? WHERE owner_handle=?",UUID.randomUUID(),owner))
        .isInstanceOf(RuntimeException.class);
    assertThat(readCycle(owner,cycle)).contains(original);
    assertThat(count("auth_account_lifecycle_intents",owner)).isEqualTo(1);
  }

  @Test @Order(8) void exactSuccessorCatalogQualifiesAndRejectsCaptureHelperDrift() {
    jdbc.execute("CREATE ROLE lifecycle_fixture_reader LOGIN PASSWORD 'fixture-reader'");
    jdbc.execute("GRANT USAGE ON SCHEMA public TO lifecycle_fixture_reader");
    jdbc.execute("GRANT SELECT ON ALL TABLES IN SCHEMA public TO lifecycle_fixture_reader");
    // The barrier's canonical ACL permits only its owner and PUBLIC SELECT.
    // Keep that exact fixture ACL rather than relaxing qualification.
    jdbc.execute("REVOKE SELECT ON public.auth_identity_ddl_barrier FROM lifecycle_fixture_reader");
    var readerSource = new org.springframework.jdbc.datasource.DriverManagerDataSource(postgres.getJdbcUrl(),"lifecycle_fixture_reader","fixture-reader");
    var readerJdbc = new JdbcTemplate(readerSource);
    var readerTx = new TransactionTemplate(new DataSourceTransactionManager(readerSource));
    readerTx.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);
    var catalog = new AuthIdentityCatalog(new AuthIdentityDatabasePolicy("voice",AuthIdentityDatabasePolicy.Loader.FLYWAY));
    readerTx.executeWithoutResult(status -> catalog.validate(readerJdbc));
    String original = jdbc.queryForObject("SELECT pg_get_functiondef('public.auth_account_lifecycle_emit(uuid,text,uuid,uuid,bigint,timestamptz,timestamptz,timestamptz)'::regprocedure)",String.class);
    jdbc.execute("CREATE OR REPLACE FUNCTION public.auth_account_lifecycle_emit(p_event uuid,p_kind text,p_owner uuid,p_cycle uuid,p_revision bigint,p_scheduled timestamptz,p_purge timestamptz,p_decided timestamptz) RETURNS void LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $$ BEGIN RETURN; END $$");
    try {
      assertThatThrownBy(() -> readerTx.executeWithoutResult(status -> catalog.validate(readerJdbc)))
          .isInstanceOf(IllegalStateException.class).hasMessageContaining("catalog unavailable");
    } finally { jdbc.execute(original); }
    readerTx.executeWithoutResult(status -> catalog.validate(readerJdbc));
  }

  @Test @Order(9) void restoreWaitingOnTheAccountLockCannotCrossTheOriginalDeadline() throws Exception {
    Account account = account(); UUID owner = owner(account);
    // A compressed-time control; the real guard captures the original schedule
    // and the real repository must re-evaluate after its account-row lock wait.
    transactions.executeWithoutResult(status -> jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp()-interval '720 hours'+interval '5 seconds',status='deleted' WHERE id=?",account.id()));
    UUID cycle = currentCycle(owner);
    var original = readCycle(owner,cycle).orElseThrow();
    var workerPid = new java.util.concurrent.CompletableFuture<Integer>();
    var executor = java.util.concurrent.Executors.newSingleThreadExecutor();
    try (var hold = source.getConnection()) {
      hold.setAutoCommit(false);
      try (var lock = hold.prepareStatement("SELECT id FROM accounts WHERE id=? FOR UPDATE")) {
        lock.setObject(1,account.id()); lock.executeQuery().close();
      }
      assertThat(now()).isBefore(original.schedule().purgeAt());
      var result = executor.submit(() -> transactions.execute(status -> {
        workerPid.complete(jdbc.queryForObject("SELECT pg_backend_pid()",Integer.class));
        return accounts.restoreDeleted(account.id());
      }));
      int pid = workerPid.get(5,java.util.concurrent.TimeUnit.SECONDS);
      long bound = System.nanoTime()+java.util.concurrent.TimeUnit.SECONDS.toNanos(5);
      while (!Boolean.TRUE.equals(jdbc.queryForObject("SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=? AND wait_event_type='Lock')",Boolean.class,pid))) {
        if(System.nanoTime()>=bound) throw new AssertionError("restore did not reach held account lock");
        Thread.sleep(10);
      }
      jdbc.queryForObject("SELECT pg_sleep(GREATEST(0,EXTRACT(EPOCH FROM (?::timestamptz-clock_timestamp()))+0.02))",Object.class,Timestamp.from(original.schedule().purgeAt()));
      hold.commit();
      assertThat(result.get(5,java.util.concurrent.TimeUnit.SECONDS)).isFalse();
      transactions.executeWithoutResult(status -> jdbc.queryForObject("SELECT role_identity_materialize_handle(?)",Long.class,owner));
      var terminal = readCycle(owner,cycle).orElseThrow();
      assertThat(terminal.schedule()).isEqualTo(original.schedule());
      assertThat(terminal.decision().orElseThrow().kind()).isEqualTo(AuthAccountLifecycleCycleStore.DecisionKind.LIFETIME_TERMINAL);
      assertThat(terminal.decision().orElseThrow().at()).isAfterOrEqualTo(original.schedule().purgeAt());
    } finally {
      executor.shutdownNow();
      assertThat(executor.awaitTermination(5,java.util.concurrent.TimeUnit.SECONDS)).isTrue();
    }
  }

  java.util.Optional<AuthAccountLifecycleCycleStore.Cycle> readCycle(UUID owner, UUID cycle) {
    return transactions.execute(status -> cycles.find(owner,cycle));
  }
  @Test @Order(10) void committedPredeadlineDecisionCanFinishAfterTheJournalWait() throws Exception {
    journalWaitThroughDeadline(true);
  }

  @Test @Order(11) void abortAfterTheJournalWaitPublishesNoRestoredDecisionOrIntent() throws Exception {
    journalWaitThroughDeadline(false);
  }

  void journalWaitThroughDeadline(boolean commit) throws Exception {
    Account account = account(); UUID owner = owner(account);
    transactions.executeWithoutResult(status -> jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp()-interval '720 hours'+interval '5 seconds',status='deleted' WHERE id=?",account.id()));
    UUID cycle = currentCycle(owner);
    var original = readCycle(owner,cycle).orElseThrow();
    var restorePid = new java.util.concurrent.CompletableFuture<Integer>();
    var expiryPid = new java.util.concurrent.CompletableFuture<Integer>();
    var executor = java.util.concurrent.Executors.newFixedThreadPool(2);
    try (var journal = source.getConnection()) {
      journal.setAutoCommit(false);
      int journalPid;
      try (var statement = journal.createStatement()) {
        try (var row = statement.executeQuery("SELECT pg_backend_pid()")) { row.next(); journalPid=row.getInt(1); }
        statement.executeQuery("SELECT singleton FROM role_identity_owner_state FOR UPDATE").close();
      }
      java.util.concurrent.Future<Boolean> restored = executor.submit(() -> transactions.execute(status -> {
        restorePid.complete(jdbc.queryForObject("SELECT pg_backend_pid()",Integer.class));
        Boolean result = accounts.restoreDeleted(account.id());
        if (!commit) status.setRollbackOnly();
        return result;
      }));
      int restoreBackend = restorePid.get(5,java.util.concurrent.TimeUnit.SECONDS);
      awaitBlockedBy(restoreBackend,journalPid);
      // The guard has acquired account+lifetime and inserted its private fact
      // before waiting only on journal evidence; its cut must already be < D.
      assertThat(now()).isBefore(original.schedule().purgeAt());
      assertThat(readCycle(owner,cycle).orElseThrow().decision()).isEmpty();
      assertThat(count("auth_account_lifecycle_intents",owner)).isEqualTo(1);
      jdbc.queryForObject("SELECT pg_sleep(GREATEST(0,EXTRACT(EPOCH FROM (?::timestamptz-clock_timestamp()))+0.02))",Object.class,Timestamp.from(original.schedule().purgeAt()));
      java.util.concurrent.Future<Long> expiry = null;
      if (commit) {
        expiry = executor.submit(() -> transactions.execute(status -> {
          expiryPid.complete(jdbc.queryForObject("SELECT pg_backend_pid()",Integer.class));
          return jdbc.queryForObject("SELECT role_identity_materialize_handle(?)",Long.class,owner);
        }));
        awaitBlockedBy(expiryPid.get(5,java.util.concurrent.TimeUnit.SECONDS),restoreBackend);
      }
      journal.commit();
      assertThat(restored.get(5,java.util.concurrent.TimeUnit.SECONDS)).isTrue();
      if (expiry != null) expiry.get(5,java.util.concurrent.TimeUnit.SECONDS);
      assertThat(now()).isAfterOrEqualTo(original.schedule().purgeAt());
      var recorded = readCycle(owner,cycle).orElseThrow();
      assertThat(recorded.schedule()).isEqualTo(original.schedule());
      if (commit) {
        var decision = recorded.decision().orElseThrow();
        assertThat(decision.kind()).isEqualTo(AuthAccountLifecycleCycleStore.DecisionKind.RESTORED);
        assertThat(decision.at()).isBefore(original.schedule().purgeAt());
        assertThat(jdbc.queryForObject("SELECT state FROM role_identity_lifetimes WHERE owner_handle=?",String.class,owner)).isEqualTo("ACTIVE");
        assertThat(count("auth_account_lifecycle_intents",owner)).isEqualTo(2);
      } else {
        assertThat(recorded.decision()).isEmpty();
        assertThat(count("auth_account_lifecycle_intents",owner)).isEqualTo(1);
        assertThat(jdbc.queryForObject("SELECT state FROM role_identity_lifetimes WHERE owner_handle=?",String.class,owner)).isEqualTo("RECOVERABLE");
      }
    } finally {
      executor.shutdownNow();
      assertThat(executor.awaitTermination(5,java.util.concurrent.TimeUnit.SECONDS)).isTrue();
    }
  }

  void awaitBlockedBy(int waiter, int blocker) throws InterruptedException {
    long bound = System.nanoTime()+java.util.concurrent.TimeUnit.SECONDS.toNanos(5);
    while (!Boolean.TRUE.equals(jdbc.queryForObject("SELECT ? = ANY(pg_blocking_pids(?))",Boolean.class,blocker,waiter))) {
      if (System.nanoTime()>=bound) throw new AssertionError("expected exact database lock blocker was not observed");
      Thread.sleep(10);
    }
  }

  int count(String table, UUID owner) {
    if (!java.util.Set.of("auth_account_lifecycle_schedules","auth_account_lifecycle_intents").contains(table)) throw new IllegalArgumentException();
    return jdbc.queryForObject("SELECT count(*) FROM "+table+" WHERE owner_handle=?",Integer.class,owner);
  }
  UUID currentCycle(UUID owner) { return jdbc.queryForObject("SELECT deletion_cycle FROM role_identity_lifetimes WHERE owner_handle=?",UUID.class,owner); }
  void failIntent(String kind) {
    if (!java.util.Set.of("DELETE_SCHEDULED","RESTORED").contains(kind)) throw new IllegalArgumentException();
    jdbc.execute("CREATE FUNCTION public.fixture_fail_lifecycle_intent() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind='"+kind+"' THEN RAISE EXCEPTION 'fixture intent failure' USING ERRCODE='23514'; END IF; RETURN NEW; END $$");
    jdbc.execute("CREATE TRIGGER fixture_fail_lifecycle_intent BEFORE INSERT ON auth_account_lifecycle_intents FOR EACH ROW EXECUTE FUNCTION public.fixture_fail_lifecycle_intent()");
  }
  void clearIntentFailure() {
    jdbc.execute("DROP TRIGGER fixture_fail_lifecycle_intent ON auth_account_lifecycle_intents");
    jdbc.execute("DROP FUNCTION public.fixture_fail_lifecycle_intent()");
  }

  Account account() {
    String email = "cycle-"+UUID.randomUUID()+"@example.com";
    auth.register(new RegisterCommand(email,null,"Correct horse battery staple",false,"{}"));
    return accounts.findByEmail(email).orElseThrow();
  }
  UUID owner(Account account) {
    return jdbc.queryForObject("SELECT role_identity_handle FROM accounts WHERE id=?",UUID.class,account.id());
  }
  Instant now() { return jdbc.queryForObject("SELECT clock_timestamp()",Timestamp.class).toInstant(); }

  @TestConfiguration(proxyBeanMethods=false) static class Transactions {
    @Bean PlatformTransactionManager transactionManager(DataSource source) {
      return new DataSourceTransactionManager(source);
    }
  }
}
