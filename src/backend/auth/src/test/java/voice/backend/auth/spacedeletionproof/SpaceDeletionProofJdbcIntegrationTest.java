package voice.backend.auth.spacedeletionproof;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.google.protobuf.CodedOutputStream;
import com.google.protobuf.Message;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.Callable;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.Function;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.jdbc.datasource.init.ScriptUtils;
import org.springframework.core.io.FileSystemResource;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.JdbcBackupCodeRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** Real PostgreSQL proof for locking, rollback, replay, recovery and hash-only persistence. */
@Testcontainers(disabledWithoutDocker = true)
class SpaceDeletionProofJdbcIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice")
          .withLabel("voice.task", "R23-AUTH-PROOF")
          .withReuse(false);

  private final Instant now = Instant.parse("2026-09-12T08:00:00.123456Z");
  private final UUID accountId = UUID.randomUUID();
  private final BCryptPasswordHasher passwords = mock(BCryptPasswordHasher.class);
  private final TotpService totp = mock(TotpService.class);
  private final SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
  private NamedParameterJdbcTemplate jdbc;
  private JdbcSpaceDeletionProofStore store;
  private BackupCodeService backup;
  private DeletionProofBinding binding;

  @BeforeAll
  static void migrate() {
    Flyway.configure()
        .dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .locations("classpath:db/migration")
        .load()
        .migrate();
  }

  @BeforeEach
  void seed() {
    var source = source();
    jdbc = new NamedParameterJdbcTemplate(source);
    jdbc.update("TRUNCATE TABLE space_deletion_proofs", params());
    store = new JdbcSpaceDeletionProofStore(jdbc, new DataSourceTransactionManager(source));
    backup = new BackupCodeService(new JdbcBackupCodeRepository(jdbc));
    jdbc.update(
        """
        INSERT INTO accounts (id,password_hash,type,status,session_epoch)
        VALUES (:id,'password-hash','regular','active',7)
        """,
        params("id", accountId));
    binding = new DeletionProofBinding(accountId, UUID.randomUUID(), 7, UUID.randomUUID(), UUID.randomUUID());
    when(passwords.matches("correct-password", "password-hash")).thenReturn(true);
    when(floors.requireFloor(accountId)).thenReturn(7L);
  }

  @Test
  void concurrentConsumersAcrossServiceInstancesReturnOneByteIdenticalDurableReceipt()
      throws Exception {
    var issued = service().issue(binding, "Exact Name", "correct-password", "", "");
    var start = new CyclicBarrier(8);
    var workers = Executors.newFixedThreadPool(8);
    try {
      List<Callable<byte[]>> tasks = new ArrayList<>();
      for (int i = 0; i < 8; i++) {
        tasks.add(
            () -> {
              start.await(10, TimeUnit.SECONDS);
              return deterministic(independentService(now).consume(binding, "Exact Name", issued.proof()));
            });
      }
      var futures = workers.invokeAll(tasks, 30, TimeUnit.SECONDS);
      var receipts = new ArrayList<String>();
      for (var future : futures) {
        assertThat(future.isCancelled()).isFalse();
        receipts.add(java.util.HexFormat.of().formatHex(future.get()));
      }
      assertThat(new HashSet<>(receipts)).hasSize(1);
      assertThat(jdbc.queryForObject(
              "SELECT count(*) FROM space_deletion_proofs WHERE operation_id=:operation",
              params("operation", binding.operationId()), Long.class).longValue())
          .isEqualTo(1L);
      assertThat(jdbc.queryForObject(
              "SELECT count(*) FROM space_deletion_proofs WHERE operation_id=:operation AND consumed_at IS NOT NULL",
              params("operation", binding.operationId()), Long.class).longValue())
          .isEqualTo(1L);
    } finally {
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void persistedRowContainsNoProofNameOrFactorPlaintext() {
    var issued = service().issue(binding, "Secret Space Name", "correct-password", "", "");
    service().consume(binding, "Secret Space Name", issued.proof());
    String row = jdbc.queryForObject(
        "SELECT row_to_json(p)::text FROM space_deletion_proofs p WHERE operation_id=:operation",
        params("operation", binding.operationId()), String.class);
    assertThat(row)
        .isNotNull()
        .doesNotContain("Secret Space Name", issued.proof(), "correct-password")
        .contains("proof_digest_sha256", "confirmation_name_sha256", "receipt_bytes");
  }

  @Test
  void migrationMetadataAllowsOnlyTheHashOnlyDeletionProofColumns() {
    List<String> columns = jdbc.queryForList(
        """
        SELECT column_name FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='space_deletion_proofs'
        """, params(), String.class);
    assertThat(columns).containsExactlyInAnyOrder(
        "operation_id", "account_id", "profile_id", "session_epoch", "space_id",
        "confirmation_name_sha256", "proof_digest_sha256", "security_revision",
        "verified_factors", "expires_at", "receipt_id", "consumed_at",
        "binding_bytes", "binding_sha256", "receipt_bytes", "receipt_sha256",
        "acknowledged_at", "receipt_lookup_hmac", "receipt_hmac_key_version", "created_at");

    List<String> binaryColumns = jdbc.queryForList(
        """
        SELECT column_name FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='space_deletion_proofs'
          AND udt_name='bytea'
        """, params(), String.class);
    assertThat(binaryColumns).containsExactlyInAnyOrder(
        "confirmation_name_sha256", "proof_digest_sha256", "binding_bytes",
        "binding_sha256", "receipt_bytes", "receipt_sha256", "receipt_lookup_hmac");

    String checkDefinitions = String.join("\n", jdbc.queryForList(
        """
        SELECT pg_get_constraintdef(oid) FROM pg_constraint
        WHERE conrelid='space_deletion_proofs'::regclass AND contype='c'
        """, params(), String.class)).toLowerCase();
    for (String digest : List.of(
        "confirmation_name_sha256", "proof_digest_sha256", "binding_sha256",
        "receipt_sha256", "receipt_lookup_hmac")) {
      assertThat(checkDefinitions).contains("octet_length(" + digest + ") = 32");
    }
    assertThat(columns).noneMatch(column -> column.matches(
        "(?i)(confirmation_name|proof|confirmation_name_(plain|raw|bytes|base64|encoded)|"
            + "proof_(plain|raw|bytes|base64|encoded))"));
  }

  @Test
  void validOwnershipTransferProofCannotConsumeTheSeparateDeletionFamily() throws Exception {
    var issued = service().issue(binding, "Space", "correct-password", "", "");
    String ownershipProof = "A".repeat(43).equals(issued.proof())
        ? "B".repeat(43) : "A".repeat(43);
    assertThat(ownershipProof).hasSize(43).matches("[A-Za-z0-9_-]{43}");
    jdbc.update(
        """
        INSERT INTO ownership_transfer_proofs
          (operation_id,account_id,profile_id,space_id,new_owner_profile_id,session_epoch,
           proof_hash,security_revision,verified_factors,expires_at,receipt_id)
        VALUES
          (:operation,:account,:profile,:space,:target,7,:hash,:revision,'password',:expires,:receipt)
        """,
        params("operation", binding.operationId(), "account", accountId,
            "profile", binding.profileId(), "space", binding.spaceId(),
            "target", UUID.randomUUID(), "hash", java.util.HexFormat.of().formatHex(sha256(ownershipProof)),
            "revision", revision(), "expires", java.sql.Timestamp.from(now.plusSeconds(300)),
            "receipt", UUID.randomUUID()));
    String deletionBefore = proofRow();
    String ownershipBefore = jdbc.queryForObject(
        "SELECT row_to_json(p)::text FROM ownership_transfer_proofs p WHERE operation_id=:operation",
        params("operation", binding.operationId()), String.class);

    assertThatThrownBy(() -> service().consume(binding, "Space", ownershipProof))
        .isInstanceOf(DeletionProofDeniedException.class);

    assertThat(proofRow()).isEqualTo(deletionBefore);
    String ownershipAfter = jdbc.queryForObject(
        "SELECT row_to_json(p)::text FROM ownership_transfer_proofs p WHERE operation_id=:operation",
        params("operation", binding.operationId()), String.class);
    assertThat(ownershipAfter).isEqualTo(ownershipBefore);
  }

  @ParameterizedTest
  @CsvSource(delimiter = '|', value = {
      "password_hash='changed'|password_hash='password-hash'",
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
  void everyAccountSecurityChangeAndRevertPermanentlyRevokesUnconsumedProof(
      String change, String revert) {
    var issued = service().issue(binding, "Space", "correct-password", "", "");
    updateAccount(change);
    updateAccount(revert);
    assertThatThrownBy(() -> service().consume(binding, "Space", issued.proof()))
        .isInstanceOf(DeletionProofDeniedException.class);
    assertThat(jdbc.queryForObject(
            "SELECT consumed_at IS NULL FROM space_deletion_proofs WHERE operation_id=:operation",
            params("operation", binding.operationId()), Boolean.class).booleanValue())
        .isTrue();
  }

  @Test
  void backupInsertConsumptionReplacementAndDeleteAdvanceSecurityRevision() {
    long initial = revision();
    List<String> first = backup.generateAndStore(accountId);
    long afterInsert = revision();
    assertThat(afterInsert).isGreaterThan(initial);
    assertThat(backup.consume(accountId, first.getFirst())).isTrue();
    long afterConsume = revision();
    assertThat(afterConsume).isGreaterThan(afterInsert);
    backup.generateAndStore(accountId);
    assertThat(revision()).isGreaterThan(afterConsume);
    jdbc.update("DELETE FROM backup_codes WHERE account_id=:id", params("id", accountId));
    assertThat(revision()).isGreaterThan(afterConsume + 1);
  }

  @Test
  void failedProofInsertRollsBackBackupConsumptionAndSecurityRevision() {
    updateAccount("totp_enabled=TRUE,totp_secret=decode('01','hex')");
    String code = backup.generateAndStore(accountId).getFirst();
    long before = revision();
    jdbc.getJdbcTemplate().execute(
        """
        CREATE FUNCTION r23_reject_proof() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN RAISE EXCEPTION 'injected R23 proof insert failure'; END $$;
        CREATE TRIGGER r23_reject_proof BEFORE INSERT ON space_deletion_proofs
        FOR EACH ROW EXECUTE FUNCTION r23_reject_proof();
        """);
    try {
      assertThatThrownBy(
              () -> service().issue(binding, "Space", "correct-password", "", code))
          .isInstanceOf(RuntimeException.class);
    } finally {
      jdbc.getJdbcTemplate().execute("DROP TRIGGER r23_reject_proof ON space_deletion_proofs");
      jdbc.getJdbcTemplate().execute("DROP FUNCTION r23_reject_proof() ");
    }
    assertThat(revision()).isEqualTo(before);
    assertThat(jdbc.queryForObject(
            "SELECT count(*) FROM backup_codes WHERE account_id=:id AND used_at IS NOT NULL",
            params("id", accountId), Long.class).longValue())
        .isZero();
    assertThat(jdbc.queryForObject(
            "SELECT count(*) FROM space_deletion_proofs WHERE operation_id=:operation",
            params("operation", binding.operationId()), Long.class).longValue())
        .isZero();
    assertThat(service().issue(binding, "Space", "correct-password", "", code)).isNotNull();
  }

  @Test
  void preAckErasureReplacesRawIdentityWithVersionedHmacAndLookupFailsClosedWithoutKey()
      throws Exception {
    var issued = service().issue(binding, "Space", "correct-password", "", "");
    var original = service().consume(binding, "Space", issued.proof());
    var lookup = lookup(issued.proof());
    byte[] hmacKey = new byte[32];
    for (int index = 0; index < hmacKey.length; index++) hmacKey[index] = (byte) index;
    ReceiptErasureKeyring keyring = mock(ReceiptErasureKeyring.class);
    when(keyring.activeVersion()).thenReturn(7);
    when(keyring.retainedVersions()).thenReturn(List.of(7));
    when(keyring.requireKey(7)).thenReturn(hmacKey);
    var keyedStore = new JdbcSpaceDeletionProofStore(
        jdbc, new DataSourceTransactionManager(jdbc.getJdbcTemplate().getDataSource()), keyring);
    byte[] expectedIndex = ReceiptErasureIndex.hmacSha256(
        hmacKey, canonicalBindingBytes(issued.proof()));

    assertThat(keyedStore.pseudonymizeAccount(accountId)).isEqualTo(1);
    Map<String, Object> erased = jdbc.queryForMap(
        """
        SELECT account_id,profile_id,binding_bytes,receipt_lookup_hmac,
               receipt_hmac_key_version,acknowledged_at
        FROM space_deletion_proofs WHERE operation_id=:operation
        """, params("operation", binding.operationId()));
    assertThat(erased.get("account_id")).isNull();
    assertThat(erased.get("profile_id")).isNull();
    assertThat(erased.get("binding_bytes")).isNull();
    assertThat(erased.get("acknowledged_at")).isNull();
    assertThat(erased.get("receipt_hmac_key_version")).isEqualTo(7);
    assertThat((byte[]) erased.get("receipt_lookup_hmac")).containsExactly(expectedIndex);

    assertThat(deterministic(service(keyedStore, now.plusSeconds(86400)).lookup(lookup)))
        .containsExactly(deterministic(original));
    when(keyring.requireKey(7)).thenThrow(new ReceiptErasureKeyUnavailableException(7));
    assertThatThrownBy(() -> service(keyedStore, now.plusSeconds(86400)).lookup(lookup))
        .isInstanceOf(ReceiptErasureKeyUnavailableException.class);
  }

  @Test
  void retentionIsUnboundedBeforeAckThenUsesTheLaterOfThirtyDaysAndAckPlusOneDay()
      throws Exception {
    var neverAcked = issueAndConsume(binding, now);
    assertThat(store.deleteRetainedReceipts(now.plusSeconds(400L * 86400))).isZero();
    assertThat(rowExists(binding.operationId())).isTrue();

    var earlyAck = new DeletionProofBinding(accountId, binding.profileId(), 7, binding.spaceId(), UUID.randomUUID());
    var early = issueAndConsume(earlyAck, now);
    acknowledge(earlyAck, early, now.plusSeconds(86400));
    assertThat(store.deleteRetainedReceipts(
        now.plusSeconds(30L * 86400).minus(1, java.time.temporal.ChronoUnit.MICROS))).isZero();
    assertThat(rowExists(earlyAck.operationId())).isTrue();
    assertThat(store.deleteRetainedReceipts(now.plusSeconds(30L * 86400))).isEqualTo(1);
    assertThat(rowExists(earlyAck.operationId())).isFalse();

    var lateAck = new DeletionProofBinding(accountId, binding.profileId(), 7, binding.spaceId(), UUID.randomUUID());
    var late = issueAndConsume(lateAck, now);
    Instant acknowledgedAt = now.plusSeconds(40L * 86400);
    acknowledge(lateAck, late, acknowledgedAt);
    assertThat(store.deleteRetainedReceipts(
        acknowledgedAt.plusSeconds(86400).minus(1, java.time.temporal.ChronoUnit.MICROS))).isZero();
    assertThat(rowExists(lateAck.operationId())).isTrue();
    assertThat(store.deleteRetainedReceipts(acknowledgedAt.plusSeconds(86400))).isEqualTo(1);
    assertThat(rowExists(lateAck.operationId())).isFalse();
    assertThat(rowExists(binding.operationId())).as("unacknowledged receipt remains").isTrue();
    assertThat(neverAcked).isNotNull();
  }

  @Test
  void ambientRollbackNeverMakesUncommittedConsumeVisibleToLookup() throws Exception {
    var issued = service().issue(binding, "Space", "correct-password", "", "");
    DeletionProofLookup lookup = lookup(issued.proof());
    var manager = new DataSourceTransactionManager(jdbc.getJdbcTemplate().getDataSource());
    var outer = new TransactionTemplate(manager);
    outer.executeWithoutResult(status -> {
      service(new JdbcSpaceDeletionProofStore(jdbc, manager), now)
          .consume(binding, "Space", issued.proof());
      assertThat(jdbc.queryForObject(
          "SELECT consumed_at IS NOT NULL FROM space_deletion_proofs WHERE operation_id=:operation",
          params("operation", binding.operationId()), Boolean.class).booleanValue()).isTrue();
      assertThatThrownBy(() -> independentService(now).lookup(lookup))
          .isInstanceOf(DeletionProofDeniedException.class);
      status.setRollbackOnly();
    });
    assertThatThrownBy(() -> independentService(now).lookup(lookup))
        .isInstanceOf(DeletionProofDeniedException.class);
    assertThat(jdbc.queryForObject(
        "SELECT consumed_at IS NULL FROM space_deletion_proofs WHERE operation_id=:operation",
        params("operation", binding.operationId()), Boolean.class).booleanValue()).isTrue();
  }

  @ParameterizedTest
  @org.junit.jupiter.params.provider.ValueSource(booleans = {false, true})
  void issueAndConsumeHoldAccountLockAgainstSecurityWriter(boolean consuming) throws Exception {
    var initial = service().issue(binding, "Space", "correct-password", "", "");
    var requested = consuming ? binding
        : new DeletionProofBinding(accountId, binding.profileId(), 7, binding.spaceId(), UUID.randomUUID());
    var locked = new CountDownLatch(1);
    var release = new CountDownLatch(1);
    SpaceDeletionProofStore paused = pauseAfterAccountLock(store, locked, release);
    var workers = Executors.newFixedThreadPool(2);
    try (var writer = java.sql.DriverManager.getConnection(
        postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())) {
      int writerPid = backendPid(writer);
      var operation = workers.submit(() -> consuming
          ? service(paused, now).consume(requested, "Space", initial.proof())
          : service(paused, now).issue(requested, "Space", "correct-password", "", ""));
      assertThat(locked.await(10, TimeUnit.SECONDS)).isTrue();
      var mutation = workers.submit(() -> {
        try (var statement = writer.prepareStatement(
            "UPDATE accounts SET password_hash='changed' WHERE id=?")) {
          statement.setObject(1, accountId);
          return statement.executeUpdate();
        }
      });
      org.awaitility.Awaitility.await().atMost(java.time.Duration.ofSeconds(10)).untilAsserted(() ->
          assertThat(jdbc.queryForObject("SELECT cardinality(pg_blocking_pids(:pid))",
              params("pid", writerPid), Integer.class).intValue()).isGreaterThan(0));
      release.countDown();
      assertThat(operation.get(10, TimeUnit.SECONDS)).isNotNull();
      assertThat(mutation.get(10, TimeUnit.SECONDS)).isEqualTo(1);
      if (!consuming) {
        var issued = (SpaceDeletionProofService.IssuedProof) operation.get();
        assertThatThrownBy(() -> independentService(now).consume(requested, "Space", issued.proof()))
            .isInstanceOf(DeletionProofDeniedException.class);
      }
    } finally {
      release.countDown();
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void concurrentIssuesCannotSpendOneBackupCodeTwice() throws Exception {
    updateAccount("totp_enabled=TRUE,totp_secret=decode('01','hex')");
    String code = backup.generateAndStore(accountId).getFirst();
    var other = new DeletionProofBinding(accountId, binding.profileId(), 7, binding.spaceId(), UUID.randomUUID());
    var start = new CyclicBarrier(2);
    var workers = Executors.newFixedThreadPool(2);
    try {
      List<Callable<Boolean>> tasks = List.of(
          () -> issueWithBackup(binding, code, start),
          () -> issueWithBackup(other, code, start));
      var futures = workers.invokeAll(tasks, 30, TimeUnit.SECONDS);
      assertThat(List.of(futures.get(0).get(), futures.get(1).get()))
          .containsExactlyInAnyOrder(true, false);
      assertThat(jdbc.queryForObject(
          "SELECT count(*) FROM space_deletion_proofs WHERE account_id=:id",
          params("id", accountId), Long.class).longValue()).isEqualTo(1L);
      assertThat(jdbc.queryForObject(
          "SELECT count(*) FROM backup_codes WHERE account_id=:id AND used_at IS NOT NULL",
          params("id", accountId), Long.class).longValue()).isEqualTo(1L);
    } finally {
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void downWaitsForConcurrentConsumeThenRefusesAndPreservesSchemaAndReceipt() throws Exception {
    var issued = service().issue(binding, "Space", "correct-password", "", "");
    var written = new CountDownLatch(1);
    var release = new CountDownLatch(1);
    SpaceDeletionProofStore paused = pauseAfterWrite(store, written, release);
    var downPid = new AtomicInteger();
    var downStarted = new CountDownLatch(1);
    var workers = Executors.newFixedThreadPool(2);
    try {
      var consuming = workers.submit(() -> service(paused, now).consume(binding, "Space", issued.proof()));
      assertThat(written.await(10, TimeUnit.SECONDS)).isTrue();
      var down = workers.submit(() -> executeDown(downPid, downStarted));
      assertThat(downStarted.await(10, TimeUnit.SECONDS)).isTrue();
      org.awaitility.Awaitility.await().atMost(java.time.Duration.ofSeconds(10)).untilAsserted(() ->
          assertThat(jdbc.queryForObject("SELECT cardinality(pg_blocking_pids(:pid))",
              params("pid", downPid.get()), Integer.class).intValue()).isGreaterThan(0));
      release.countDown();
      assertThat(consuming.get(10, TimeUnit.SECONDS)).isNotNull();
      assertThatThrownBy(() -> down.get(10, TimeUnit.SECONDS))
          .isInstanceOf(ExecutionException.class)
          .satisfies(error -> assertThat(rootSqlState(error)).isEqualTo("55000"));
      assertThat(jdbc.queryForObject("SELECT to_regclass('space_deletion_proofs') IS NOT NULL",
          params(), Boolean.class).booleanValue()).isTrue();
      assertThat(rowExists(binding.operationId())).isTrue();
    } finally {
      release.countDown();
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void acknowledgementIsDurableIdempotentAndUsesExactReceiptDigest() throws Exception {
    var issued = service().issue(binding, "Space", "correct-password", "", "");
    var receipt = service().consume(binding, "Space", issued.proof());
    var acknowledgement = new DeletionProofAcknowledgement(
        UUID.fromString(receipt.getReceiptId()), binding.spaceId(), binding.operationId(), domainHash(receipt));
    Instant acknowledgedAt = service().acknowledge(acknowledgement);
    assertThat(independentService(now.plusSeconds(10)).acknowledge(acknowledgement))
        .isEqualTo(acknowledgedAt);
    var changed = new DeletionProofAcknowledgement(
        acknowledgement.receiptId(), acknowledgement.spaceId(), acknowledgement.operationId(), sha256("changed"));
    assertThatThrownBy(() -> independentService(now.plusSeconds(20)).acknowledge(changed))
        .isInstanceOf(DeletionProofDeniedException.class);
    assertThat(jdbc.queryForObject(
            "SELECT acknowledged_at FROM space_deletion_proofs WHERE operation_id=:operation",
            params("operation", binding.operationId()), java.sql.Timestamp.class).toInstant())
        .isEqualTo(acknowledgedAt);
  }

  private SpaceDeletionProofService service() {
    return service(store, now);
  }

  private SpaceDeletionProofService independentService(Instant instant) {
    var source = source();
    var connection = new NamedParameterJdbcTemplate(source);
    return service(new JdbcSpaceDeletionProofStore(
        connection, new DataSourceTransactionManager(source)), connection, instant);
  }

  private SpaceDeletionProofService service(SpaceDeletionProofStore target, Instant instant) {
    return service(target, jdbc, instant);
  }

  private SpaceDeletionProofService service(
      SpaceDeletionProofStore target,
      NamedParameterJdbcTemplate factorJdbc,
      Instant instant) {
    return new SpaceDeletionProofService(
        target,
        passwords,
        totp,
        new BackupCodeService(new JdbcBackupCodeRepository(factorJdbc)),
        floors,
        Clock.fixed(instant, ZoneOffset.UTC));
  }

  private app.voice.auth.v1.SpaceDeletionProofReceipt issueAndConsume(
      DeletionProofBinding requested, Instant instant) {
    var issued = service(store, instant).issue(requested, "Space", "correct-password", "", "");
    return service(store, instant).consume(requested, "Space", issued.proof());
  }

  private void acknowledge(
      DeletionProofBinding requested,
      app.voice.auth.v1.SpaceDeletionProofReceipt receipt,
      Instant at) throws Exception {
    service(store, at).acknowledge(new DeletionProofAcknowledgement(
        UUID.fromString(receipt.getReceiptId()), requested.spaceId(), requested.operationId(),
        domainHash(receipt)));
  }

  private boolean issueWithBackup(
      DeletionProofBinding requested, String code, CyclicBarrier start) throws Exception {
    start.await(10, TimeUnit.SECONDS);
    try {
      independentService(now).issue(requested, "Space", "correct-password", "", code);
      return true;
    } catch (DeletionProofDeniedException denied) {
      return false;
    }
  }

  private SpaceDeletionProofStore pauseAfterAccountLock(
      SpaceDeletionProofStore delegate, CountDownLatch locked, CountDownLatch release) {
    return new SpaceDeletionProofStore() {
      @Override public java.util.Optional<StoredDeletionProof> findConsumed(UUID operationId) {
        return delegate.findConsumed(operationId);
      }
      @Override public Instant acknowledge(DeletionProofAcknowledgement value, Instant at) {
        return delegate.acknowledge(value, at);
      }
      @Override public int deleteRetainedReceipts(Instant at) {
        return delegate.deleteRetainedReceipts(at);
      }
      @Override public int pseudonymizeAccount(UUID account) {
        return delegate.pseudonymizeAccount(account);
      }
      @Override public <T> T withAccount(UUID id, Function<Session, T> action) {
        return delegate.withAccount(id, session -> {
          locked.countDown();
          try {
            if (!release.await(15, TimeUnit.SECONDS)) throw new AssertionError("lock release timeout");
          } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            throw new AssertionError(interrupted);
          }
          return action.apply(session);
        });
      }
    };
  }

  private SpaceDeletionProofStore pauseAfterWrite(
      SpaceDeletionProofStore delegate, CountDownLatch written, CountDownLatch release) {
    return new SpaceDeletionProofStore() {
      @Override public java.util.Optional<StoredDeletionProof> findConsumed(UUID operationId) {
        return delegate.findConsumed(operationId);
      }
      @Override public Instant acknowledge(DeletionProofAcknowledgement value, Instant at) {
        return delegate.acknowledge(value, at);
      }
      @Override public int deleteRetainedReceipts(Instant at) {
        return delegate.deleteRetainedReceipts(at);
      }
      @Override public int pseudonymizeAccount(UUID account) {
        return delegate.pseudonymizeAccount(account);
      }
      @Override public <T> T withAccount(UUID id, Function<Session, T> action) {
        return delegate.withAccount(id, session -> {
          T result = action.apply(session);
          written.countDown();
          try {
            if (!release.await(15, TimeUnit.SECONDS)) throw new AssertionError("commit release timeout");
          } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            throw new AssertionError(interrupted);
          }
          return result;
        });
      }
    };
  }

  private Object executeDown(AtomicInteger pid, CountDownLatch started) throws Exception {
    try (var connection = java.sql.DriverManager.getConnection(
        postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())) {
      pid.set(backendPid(connection));
      started.countDown();
      ScriptUtils.executeSqlScript(connection, new FileSystemResource(
          voice.backend.auth.SpaceDeletionProofMigrationContractTest.repositoryRoot()
              .resolve("src/backend/migrations/auth_db/000014_space_deletion_proofs.down.sql")));
      return null;
    }
  }

  private static int backendPid(java.sql.Connection connection) throws Exception {
    try (var statement = connection.createStatement();
        var rows = statement.executeQuery("SELECT pg_backend_pid()")) {
      rows.next();
      return rows.getInt(1);
    }
  }

  private static String rootSqlState(Throwable error) {
    Throwable current = error;
    while (current != null) {
      if (current instanceof java.sql.SQLException sql && sql.getSQLState() != null) {
        return sql.getSQLState();
      }
      current = current.getCause();
    }
    return null;
  }

  private boolean rowExists(UUID operationId) {
    return jdbc.queryForObject(
        "SELECT EXISTS(SELECT 1 FROM space_deletion_proofs WHERE operation_id=:operation)",
        params("operation", operationId), Boolean.class);
  }

  private byte[] canonicalBindingBytes(String proof) throws Exception {
    var factors = jdbc.queryForObject(
        "SELECT verified_factors FROM space_deletion_proofs WHERE operation_id=:operation",
        params("operation", binding.operationId()), String.class);
    var proto = app.voice.auth.v1.SpaceDeletionProofBinding.newBuilder()
        .setProtocolVersion(1).setAccountId(accountId.toString())
        .setProfileId(binding.profileId().toString()).setSessionEpoch(7)
        .setSpaceId(binding.spaceId().toString()).setOperationId(binding.operationId().toString())
        .setConfirmationNameSha256(com.google.protobuf.ByteString.copyFrom(sha256("Space")))
        .setProofDigestSha256(com.google.protobuf.ByteString.copyFrom(sha256(proof)))
        .addAllVerifiedFactors(protoFactors(factors))
        .setPurpose(app.voice.auth.v1.ProofPurpose.PROOF_PURPOSE_SPACE_DELETE).build();
    return deterministic(proto);
  }

  private static List<app.voice.auth.v1.VerifiedFactor> protoFactors(String encoded) {
    return java.util.Arrays.stream(encoded.split(","))
        .map(factor -> switch (factor) {
          case "password" -> app.voice.auth.v1.VerifiedFactor.VERIFIED_FACTOR_PASSWORD;
          case "totp" -> app.voice.auth.v1.VerifiedFactor.VERIFIED_FACTOR_TOTP;
          case "backup_code" -> app.voice.auth.v1.VerifiedFactor.VERIFIED_FACTOR_BACKUP_CODE;
          default -> throw new IllegalArgumentException("unexpected stored factor");
        })
        .toList();
  }

  private static MapSqlParameterSource params(Object... pairs) {
    if (pairs.length % 2 != 0) throw new IllegalArgumentException("name/value pairs required");
    var parameters = new MapSqlParameterSource();
    for (int index = 0; index < pairs.length; index += 2) {
      parameters.addValue((String) pairs[index], pairs[index + 1]);
    }
    return parameters;
  }

  private DriverManagerDataSource source() {
    return new DriverManagerDataSource(
        postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
  }

  private DeletionProofLookup lookup(String proof) throws Exception {
    return new DeletionProofLookup(
        accountId, binding.profileId(), 7, binding.spaceId(), binding.operationId(),
        sha256("Space"), sha256(proof));
  }

  private void updateAccount(String assignment) {
    jdbc.update("UPDATE accounts SET " + assignment + " WHERE id=:id", params("id", accountId));
  }

  private long revision() {
    return jdbc.queryForObject(
        "SELECT security_revision FROM accounts WHERE id=:id", params("id", accountId), Long.class);
  }

  private String proofRow() {
    return jdbc.queryForObject(
        "SELECT row_to_json(p)::text FROM space_deletion_proofs p WHERE operation_id=:operation",
        params("operation", binding.operationId()), String.class);
  }

  private static byte[] sha256(String value) throws Exception {
    return MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8));
  }

  private static byte[] deterministic(Message message) throws Exception {
    byte[] bytes = new byte[message.getSerializedSize()];
    CodedOutputStream output = CodedOutputStream.newInstance(bytes);
    output.useDeterministicSerialization();
    message.writeTo(output);
    output.checkNoSpaceLeft();
    return bytes;
  }

  private static byte[] domainHash(Message message) throws Exception {
    byte[] name = message.getDescriptorForType().getFullName().getBytes(StandardCharsets.UTF_8);
    byte[] bytes = deterministic(message);
    byte[] material = new byte[name.length + 1 + bytes.length];
    System.arraycopy(name, 0, material, 0, name.length);
    System.arraycopy(bytes, 0, material, name.length + 1, bytes.length);
    return MessageDigest.getInstance("SHA-256").digest(material);
  }
}
