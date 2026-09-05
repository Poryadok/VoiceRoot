package voice.backend.auth.repository;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

/** JDBC outbox repository. Claims and transitions are fenced by the exact lease expiry. */
public final class JdbcAccountDeletionOperationRepository implements AccountDeletionOperationRepository {
  private static final String COLUMNS =
      "operation_id, account_id, session_epoch, restore_token_hash, state, attempt_count, "
          + "next_attempt_at, locked_until, last_error_code, floor_recorded_at, event_published_at, "
          + "created_at, updated_at";
  private static final RowMapper<AccountDeletionOperation> ROW =
      (rs, row) ->
          new AccountDeletionOperation(
              rs.getObject("operation_id", UUID.class), rs.getObject("account_id", UUID.class),
              rs.getLong("session_epoch"), rs.getString("restore_token_hash"),
              AccountDeletionState.valueOf(rs.getString("state")), rs.getInt("attempt_count"),
              instant(rs.getTimestamp("next_attempt_at")), instant(rs.getTimestamp("locked_until")),
              rs.getString("last_error_code"), instant(rs.getTimestamp("floor_recorded_at")),
              instant(rs.getTimestamp("event_published_at")), instant(rs.getTimestamp("created_at")),
              instant(rs.getTimestamp("updated_at")));
  private final NamedParameterJdbcTemplate jdbc;

  public JdbcAccountDeletionOperationRepository(NamedParameterJdbcTemplate jdbc) { this.jdbc = jdbc; }

  @Override
  public AccountDeletionOperation createOrResume(
      UUID operationId, UUID accountId, long epoch, String tokenHash, Instant now) {
    jdbc.update(
        """
        INSERT INTO account_deletion_operations (
          operation_id, account_id, session_epoch, restore_token_hash, state, attempt_count,
          next_attempt_at, created_at, updated_at)
        VALUES (:operationId, :accountId, :epoch, :tokenHash, 'PENDING_FLOOR', 0, :now, :now, :now)
        ON CONFLICT (account_id, session_epoch) DO NOTHING
        """,
        params(operationId, now).addValue("accountId", accountId).addValue("epoch", epoch)
            .addValue("tokenHash", tokenHash));
    return findByAccountAndEpoch(accountId, epoch)
        .orElseThrow(() -> new IllegalStateException("account deletion operation missing after insert"));
  }

  @Override
  public Optional<AccountDeletionOperation> findByAccountAndEpoch(UUID accountId, long epoch) {
    return jdbc.query("SELECT " + COLUMNS + " FROM account_deletion_operations "
            + "WHERE account_id = :accountId AND session_epoch = :epoch",
        new MapSqlParameterSource().addValue("accountId", accountId).addValue("epoch", epoch), ROW)
        .stream().findFirst();
  }

  @Override
  public List<AccountDeletionOperation> leaseDue(
      AccountDeletionState state, int batchSize, Instant now, Instant leaseUntil) {
    if (batchSize <= 0 || !leaseUntil.isAfter(now)) throw new IllegalArgumentException("invalid lease");
    String leasedColumns = "leased." + COLUMNS.replace(", ", ", leased.");
    return jdbc.query(
        """
        WITH eligible AS (
          SELECT operation_id, next_attempt_at, created_at
          FROM account_deletion_operations
          WHERE state = :state AND next_attempt_at <= :now
            AND (locked_until IS NULL OR locked_until <= :now)
          ORDER BY next_attempt_at, created_at, operation_id
          LIMIT :batchSize FOR UPDATE SKIP LOCKED
        ), leased AS (
          UPDATE account_deletion_operations operation
          SET locked_until = :leaseUntil, updated_at = :now
          FROM eligible WHERE operation.operation_id = eligible.operation_id
          RETURNING operation.*
        )
        SELECT %s FROM leased JOIN eligible USING (operation_id)
        ORDER BY eligible.next_attempt_at, eligible.created_at, eligible.operation_id
        """.formatted(leasedColumns),
        new MapSqlParameterSource().addValue("state", state.name()).addValue("batchSize", batchSize)
            .addValue("now", Timestamp.from(now)).addValue("leaseUntil", Timestamp.from(leaseUntil)), ROW);
  }

  @Override
  public Optional<AccountDeletionOperation> lease(
      UUID operationId, AccountDeletionState state, Instant now, Instant leaseUntil) {
    return jdbc.query(
        "UPDATE account_deletion_operations SET locked_until = :leaseUntil, updated_at = :now "
            + "WHERE operation_id = :id AND state = :state AND next_attempt_at <= :now "
            + "AND (locked_until IS NULL OR locked_until <= :now) RETURNING " + COLUMNS,
        params(operationId, now).addValue("state", state.name())
            .addValue("leaseUntil", Timestamp.from(leaseUntil)), ROW).stream().findFirst();
  }

  @Override
  public AccountDeletionAdvanceResult markFloorRecorded(UUID id, Instant lease, Instant now) {
    return advance(id, AccountDeletionState.PENDING_FLOOR, lease, now);
  }

  @Override
  public AccountDeletionAdvanceResult markEventPublished(UUID id, Instant lease, Instant now) {
    return advance(id, AccountDeletionState.PENDING_EVENT, lease, now);
  }

  @Override
  public Optional<AccountDeletionOperation> recordFailure(
      UUID id, Instant lease, String code, Instant next, Instant now) {
    return jdbc.query(
        "UPDATE account_deletion_operations SET attempt_count = attempt_count + 1, "
            + "last_error_code = :code, next_attempt_at = :next, locked_until = NULL, updated_at = :now "
            + "WHERE operation_id = :id AND state <> 'COMPLETED' AND locked_until = :lease "
            + "AND locked_until > :now RETURNING " + COLUMNS,
        params(id, now).addValue("lease", Timestamp.from(lease)).addValue("code", code)
            .addValue("next", Timestamp.from(next)), ROW).stream().findFirst();
  }

  private AccountDeletionAdvanceResult advance(UUID id, AccountDeletionState expected, Instant lease, Instant now) {
    AccountDeletionState next = expected == AccountDeletionState.PENDING_FLOOR
        ? AccountDeletionState.PENDING_EVENT : AccountDeletionState.COMPLETED;
    String marker = expected == AccountDeletionState.PENDING_FLOOR
        ? "floor_recorded_at = :now" : "event_published_at = :now";
    boolean applied = !jdbc.query(
        "UPDATE account_deletion_operations SET state = :next, " + marker
            + ", locked_until = NULL, updated_at = :now WHERE operation_id = :id AND state = :expected "
            + "AND locked_until = :lease AND locked_until > :now RETURNING operation_id",
        params(id, now).addValue("expected", expected.name()).addValue("next", next.name())
            .addValue("lease", Timestamp.from(lease)), (rs, row) -> rs.getObject(1)).isEmpty();
    if (applied) return AccountDeletionAdvanceResult.APPLIED;
    Optional<AccountDeletionOperation> current = jdbc.query("SELECT " + COLUMNS
            + " FROM account_deletion_operations WHERE operation_id = :id", new MapSqlParameterSource("id", id), ROW)
        .stream().findFirst();
    if (current.isEmpty()) return AccountDeletionAdvanceResult.NOT_FOUND;
    return current.get().state().ordinal() > expected.ordinal()
        ? AccountDeletionAdvanceResult.ALREADY_APPLIED : AccountDeletionAdvanceResult.LEASE_LOST;
  }

  private static MapSqlParameterSource params(UUID id, Instant now) {
    return new MapSqlParameterSource().addValue("id", id).addValue("operationId", id)
        .addValue("now", Timestamp.from(now));
  }

  private static Instant instant(Timestamp value) { return value == null ? null : value.toInstant(); }
}
