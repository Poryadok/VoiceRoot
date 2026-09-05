package voice.backend.auth.repository;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

/** JDBC outbox state. Every update is monotonic so recovery can safely repeat a completed step. */
public final class JdbcAccountDeletionOperationRepository implements AccountDeletionOperationRepository {
  private static final RowMapper<AccountDeletionOperation> ROW =
      (rs, row) ->
          new AccountDeletionOperation(
              rs.getObject("operation_id", UUID.class),
              rs.getObject("account_id", UUID.class),
              rs.getLong("session_epoch"),
              rs.getString("restore_token_hash"),
              AccountDeletionState.valueOf(rs.getString("state")),
              instant(rs.getTimestamp("floor_recorded_at")),
              instant(rs.getTimestamp("event_published_at")),
              instant(rs.getTimestamp("created_at")),
              instant(rs.getTimestamp("updated_at")));
  private final NamedParameterJdbcTemplate jdbc;

  public JdbcAccountDeletionOperationRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @Override
  public AccountDeletionOperation createOrResume(
      UUID operationId, UUID accountId, long epoch, String tokenHash, Instant now) {
    MapSqlParameterSource p =
        new MapSqlParameterSource()
            .addValue("operationId", operationId)
            .addValue("accountId", accountId)
            .addValue("epoch", epoch)
            .addValue("tokenHash", tokenHash)
            .addValue("now", Timestamp.from(now));
    jdbc.update(
        """
        INSERT INTO account_deletion_operations (
          operation_id, account_id, session_epoch, restore_token_hash, state, created_at, updated_at)
        VALUES (:operationId, :accountId, :epoch, :tokenHash, 'PENDING_FLOOR', :now, :now)
        ON CONFLICT (account_id, session_epoch) DO NOTHING
        """,
        p);
    return findByAccountAndEpoch(accountId, epoch)
        .orElseThrow(() -> new IllegalStateException("account deletion operation missing after insert"));
  }

  @Override
  public Optional<AccountDeletionOperation> findByAccountAndEpoch(UUID accountId, long epoch) {
    return jdbc
        .query(
            """
            SELECT operation_id, account_id, session_epoch, restore_token_hash, state,
                   floor_recorded_at, event_published_at, created_at, updated_at
            FROM account_deletion_operations
            WHERE account_id = :accountId AND session_epoch = :epoch
            """,
            new MapSqlParameterSource().addValue("accountId", accountId).addValue("epoch", epoch), ROW)
        .stream()
        .findFirst();
  }

  @Override
  public AccountDeletionOperation markFloorRecorded(UUID operationId, Instant now) {
    return transition(operationId, AccountDeletionState.PENDING_FLOOR, AccountDeletionState.PENDING_EVENT,
        "floor_recorded_at = :now", now);
  }

  @Override
  public AccountDeletionOperation markEventPublished(UUID operationId, Instant now) {
    return transition(operationId, AccountDeletionState.PENDING_EVENT, AccountDeletionState.COMPLETED,
        "event_published_at = :now", now);
  }

  private AccountDeletionOperation transition(
      UUID operationId, AccountDeletionState expected, AccountDeletionState next, String marker, Instant now) {
    java.util.List<AccountDeletionOperation> updated =
        jdbc.query(
            """
            UPDATE account_deletion_operations
            SET state = :next, %s, updated_at = :now
            WHERE operation_id = :operationId AND state = :expected
            RETURNING operation_id, account_id, session_epoch, restore_token_hash, state,
                      floor_recorded_at, event_published_at, created_at, updated_at
            """.formatted(marker),
            new MapSqlParameterSource()
                .addValue("operationId", operationId)
                .addValue("expected", expected.name())
                .addValue("next", next.name())
                .addValue("now", Timestamp.from(now)), ROW);
    if (!updated.isEmpty()) {
      return updated.getFirst();
    }
    return jdbc.query(
            """
            SELECT operation_id, account_id, session_epoch, restore_token_hash, state,
                   floor_recorded_at, event_published_at, created_at, updated_at
            FROM account_deletion_operations WHERE operation_id = :operationId
            """, new MapSqlParameterSource("operationId", operationId), ROW)
        .stream().findFirst()
        .orElseThrow(() -> new IllegalArgumentException("unknown account deletion operation"));
  }

  private static Instant instant(Timestamp timestamp) {
    return timestamp == null ? null : timestamp.toInstant();
  }
}
