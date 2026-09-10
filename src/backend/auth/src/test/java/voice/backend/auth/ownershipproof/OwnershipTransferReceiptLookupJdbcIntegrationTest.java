package voice.backend.auth.ownershipproof;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.function.Function;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
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

@Testcontainers(disabledWithoutDocker = true)
class OwnershipTransferReceiptLookupJdbcIntegrationTest {
  @Container static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db").withUsername("voice").withPassword("voice");
  private final Instant now = Instant.parse("2026-09-10T10:00:00.123456789Z");
  private final ProofBinding binding = new ProofBinding(UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), 7);
  private NamedParameterJdbcTemplate jdbc;
  private JdbcOwnershipTransferProofStore store;
  private OwnershipTransferProofService service;

  @BeforeAll static void migrate() {
    Flyway.configure().dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .locations("classpath:db/migration").load().migrate();
  }

  @BeforeEach void seed() {
    var source = source();
    jdbc = new NamedParameterJdbcTemplate(source);
    store = new JdbcOwnershipTransferProofStore(jdbc, new DataSourceTransactionManager(source));
    jdbc.update("INSERT INTO accounts(id,password_hash,type,status,session_epoch) VALUES(:id,'hash','regular','active',7)",
        Map.of("id", binding.accountId()));
    new BackupCodeService(new JdbcBackupCodeRepository(jdbc)).generateAndStore(binding.accountId());
    service = issuingService(store);
  }

  private DriverManagerDataSource source() {
    return new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
  }

  private OwnershipTransferProofService issuingService(OwnershipTransferProofStore target) {
    var passwords = mock(BCryptPasswordHasher.class);
    when(passwords.matches("correct", "hash")).thenReturn(true);
    var floors = mock(SessionEpochFloorStore.class);
    when(floors.requireFloor(binding.accountId())).thenReturn(7L);
    return new OwnershipTransferProofService(target, passwords, mock(TotpService.class),
        new BackupCodeService(new JdbcBackupCodeRepository(jdbc)), floors, Clock.fixed(now, ZoneOffset.UTC));
  }

  /** Fresh DB connections, expired clock and unavailable current-session infrastructure. */
  private OwnershipTransferProofService restarted() {
    var source = source();
    var readonly = new JdbcOwnershipTransferProofStore(new NamedParameterJdbcTemplate(source), new DataSourceTransactionManager(source));
    var floors = mock(SessionEpochFloorStore.class);
    when(floors.requireFloor(any())).thenThrow(new AssertionError("historical lookup consulted current epoch floor"));
    return new OwnershipTransferProofService(readonly, mock(BCryptPasswordHasher.class), mock(TotpService.class),
        mock(BackupCodeService.class), floors, Clock.fixed(now.plusSeconds(86400), ZoneOffset.UTC));
  }

  @Test void committedReceiptSurvivesSecurityChangesAndPhysicalAccountRemovalExactly() throws Exception {
    var issued = service.issue(binding, "correct", "", "");
    var original = service.consume(binding, issued.proof());
    String digest = digest(issued.proof());
    assertThat(original.consumedAt()).isEqualTo(now.truncatedTo(java.time.temporal.ChronoUnit.MICROS));
    assertThat(restarted().lookup(binding, digest)).isEqualTo(original);
    jdbc.update("""
        UPDATE accounts SET password_hash='new-password',totp_enabled=TRUE,totp_secret=decode('01','hex'),
          session_epoch=8,status='deleted',deleted_at=CURRENT_TIMESTAMP WHERE id=:id
        """, Map.of("id", binding.accountId()));
    var changed = snapshot();
    assertThat(restarted().lookup(binding, digest)).isEqualTo(original);
    var currentEpoch = new ProofBinding(binding.accountId(), binding.profileId(), binding.spaceId(),
        binding.newOwnerProfileId(), binding.operationId(), 8);
    assertThatThrownBy(() -> restarted().lookup(currentEpoch, digest)).isInstanceOf(ProofDeniedException.class);
    assertThat(snapshot()).isEqualTo(changed);
    jdbc.update("DELETE FROM accounts WHERE id=:id", Map.of("id", binding.accountId()));
    var erased = snapshot();
    assertThat(restarted().lookup(binding, digest)).isEqualTo(original);
    assertThat(restarted().lookup(binding, digest)).isEqualTo(original);
    assertThat(snapshot()).isEqualTo(erased);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM accounts WHERE id=:id", Map.of("id", binding.accountId()), Long.class)).isZero();
  }

  @Test void missingUnconsumedAndWrongTupleOrDigestNeverWriteAnything() throws Exception {
    var issued = service.issue(binding, "correct", "", "");
    var before = snapshot();
    assertThatThrownBy(() -> restarted().lookup(binding, digest(issued.proof()))).isInstanceOf(ProofDeniedException.class);
    assertThat(store.findConsumed(binding.operationId())).isEmpty();
    assertThat(snapshot()).isEqualTo(before);
    var receipt = service.consume(binding, issued.proof());
    var committed = snapshot();
    var wrongTarget = new ProofBinding(binding.accountId(), binding.profileId(), binding.spaceId(), UUID.randomUUID(), binding.operationId(), 7);
    var missing = new ProofBinding(binding.accountId(), binding.profileId(), binding.spaceId(), binding.newOwnerProfileId(), UUID.randomUUID(), 7);
    assertThatThrownBy(() -> restarted().lookup(wrongTarget, digest(issued.proof()))).isInstanceOf(ProofDeniedException.class);
    assertThatThrownBy(() -> restarted().lookup(missing, digest(issued.proof()))).isInstanceOf(ProofDeniedException.class);
    assertThatThrownBy(() -> restarted().lookup(binding, "f".repeat(64))).isInstanceOf(ProofDeniedException.class);
    assertThat(restarted().lookup(binding, digest(issued.proof()))).isEqualTo(receipt);
    assertThat(snapshot()).isEqualTo(committed);
  }

  @ParameterizedTest @ValueSource(booleans = {false, true})
  void lookupDoesNotWaitOnAccountLockOrExposeUncommittedConsume(boolean rollback) throws Exception {
    var issued = service.issue(binding, "correct", "", "");
    String digest = digest(issued.proof());
    var before = snapshot();
    var written = new CountDownLatch(1);
    var finish = new CountDownLatch(1);
    OwnershipTransferProofStore delayedCommit = new OwnershipTransferProofStore() {
      public Optional<StoredProof> findConsumed(UUID id) { return store.findConsumed(id); }
      public <T> T withAccount(UUID id, Function<Session, T> callback) {
        return store.withAccount(id, session -> {
          T result = callback.apply(session);
          written.countDown();
          try {
            if (!finish.await(15, TimeUnit.SECONDS)) throw new AssertionError("consume transaction not released");
          } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            throw new AssertionError(interrupted);
          }
          if (rollback) throw new InjectedRollback();
          return result;
        });
      }
    };
    var workers = Executors.newFixedThreadPool(2);
    try {
      var consuming = workers.submit(() -> issuingService(delayedCommit).consume(binding, issued.proof()));
      assertThat(written.await(10, TimeUnit.SECONDS)).isTrue();
      // The real consume has updated its receipt and holds the account lock, but has not committed.
      var lookup = workers.submit(() -> {
        assertThatThrownBy(() -> restarted().lookup(binding, digest)).isInstanceOf(ProofDeniedException.class);
        assertThat(store.findConsumed(binding.operationId())).isEmpty();
      });
      lookup.get(3, TimeUnit.SECONDS);
      assertThat(consuming).isNotDone();
      assertThat(snapshot()).isEqualTo(before);
      finish.countDown();
      if (rollback) {
        assertThatThrownBy(() -> consuming.get(10, TimeUnit.SECONDS)).isInstanceOf(ExecutionException.class)
            .hasCauseInstanceOf(InjectedRollback.class);
        assertThatThrownBy(() -> restarted().lookup(binding, digest)).isInstanceOf(ProofDeniedException.class);
        assertThat(snapshot()).isEqualTo(before);
      } else {
        var receipt = consuming.get(10, TimeUnit.SECONDS);
        var committed = snapshot();
        assertThat(restarted().lookup(binding, digest)).isEqualTo(receipt);
        assertThat(snapshot()).isEqualTo(committed);
      }
    } finally {
      finish.countDown();
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @ParameterizedTest @ValueSource(booleans = {false, true})
  void sameThreadLookupCannotRecoverReceiptFromItsOwnUncommittedOuterTransaction(boolean rollback) throws Exception {
    var issued = service.issue(binding, "correct", "", "");
    String digest = digest(issued.proof());
    var before = snapshot();
    // Bound database waits without moving lookup to another thread and losing Spring's ambient transaction.
    jdbc.getJdbcTemplate().setQueryTimeout(3);
    var outer = new org.springframework.transaction.support.TransactionTemplate(
        new DataSourceTransactionManager(jdbc.getJdbcTemplate().getDataSource()));
    outer.setTimeout(10);
    Thread caller = Thread.currentThread();
    var receipt = outer.execute(status -> {
      var uncommitted = service.consume(binding, issued.proof());
      assertThat(org.springframework.transaction.support.TransactionSynchronizationManager.isActualTransactionActive()).isTrue();
      // This control SELECT proves consume joined this exact outer transaction and wrote its receipt.
      assertThat(jdbc.queryForObject(
          "SELECT consumed_at IS NOT NULL FROM ownership_transfer_proofs WHERE operation_id=:id",
          Map.of("id", binding.operationId()), Boolean.class)).isTrue();
      org.junit.jupiter.api.Assertions.assertTimeout(Duration.ofSeconds(5), () -> {
        assertThat(Thread.currentThread()).isSameAs(caller);
        assertThatThrownBy(() -> service.lookup(binding, digest)).isInstanceOf(ProofDeniedException.class);
      });
      assertThat(status.isRollbackOnly()).isFalse();
      if (rollback) status.setRollbackOnly();
      return uncommitted;
    });
    if (rollback) {
      assertThatThrownBy(() -> service.lookup(binding, digest)).isInstanceOf(ProofDeniedException.class);
      assertThat(store.findConsumed(binding.operationId())).isEmpty();
      assertThat(snapshot()).isEqualTo(before);
    } else {
      var committed = snapshot();
      assertThat(service.lookup(binding, digest)).isEqualTo(receipt);
      assertThat(restarted().lookup(binding, digest)).isEqualTo(receipt);
      assertThat(snapshot()).isEqualTo(committed);
    }
  }
  /** Exact row JSON snapshots include hashes, revisions, receipt fields and all backup state. */
  private List<List<String>> snapshot() {
    var params = Map.of("id", binding.accountId());
    return List.of(
        jdbc.queryForList("SELECT row_to_json(a)::text FROM accounts a WHERE id=:id", params, String.class),
        jdbc.queryForList("SELECT row_to_json(p)::text FROM ownership_transfer_proofs p WHERE account_id=:id ORDER BY operation_id", params, String.class),
        jdbc.queryForList("SELECT row_to_json(b)::text FROM backup_codes b WHERE account_id=:id ORDER BY id", params, String.class));
  }

  private static String digest(String proof) throws Exception {
    return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(proof.getBytes(StandardCharsets.UTF_8)));
  }

  private static final class InjectedRollback extends RuntimeException {}
}
