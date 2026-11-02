package voice.backend.auth.roleidentityauthority;

import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.Objects;
import java.util.Optional;
import java.util.UUID;
import javax.sql.DataSource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.ConnectionHolder;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/** Local immutable persistence view; not a protected observation or currentness grant. */
public final class AuthAccountLifecycleCycleStore {
  public enum DecisionKind { RESTORED, LIFETIME_TERMINAL }
  public record Schedule(UUID ownerHandle, UUID cycle, long revision, Instant scheduledAt, Instant purgeAt) {}
  public record Decision(DecisionKind kind, long revision, Instant at) {}
  public record Cycle(Schedule schedule, Optional<Decision> decision) {}

  private final DataSource source;
  private final JdbcTemplate jdbc;

  public AuthAccountLifecycleCycleStore(DataSource source) {
    this.source = Objects.requireNonNull(source);
    this.jdbc = new JdbcTemplate(source);
  }

  public Optional<Cycle> find(UUID owner, UUID cycle) {
    requireOwningTransaction();
    var rows = jdbc.query("""
        SELECT s.protocol_version,s.owner_handle,s.deletion_cycle_id,s.schedule_revision,s.scheduled_at,s.purge_at,
               d.decision,d.decision_revision,d.decision_at
        FROM auth_account_lifecycle_schedules s
        LEFT JOIN auth_account_lifecycle_decisions d USING(owner_handle,deletion_cycle_id)
        WHERE s.owner_handle=? AND s.deletion_cycle_id=?
        """, (rs, index) -> {
          if (rs.getInt("protocol_version") != 1) throw new IllegalStateException("unsupported cycle source version");
          var schedule = new Schedule(rs.getObject("owner_handle",UUID.class),rs.getObject("deletion_cycle_id",UUID.class),
              rs.getLong("schedule_revision"),rs.getTimestamp("scheduled_at").toInstant(),rs.getTimestamp("purge_at").toInstant());
          String kind = rs.getString("decision");
          Timestamp at = rs.getTimestamp("decision_at");
          var decision = kind == null ? Optional.<Decision>empty()
              : Optional.of(new Decision(DecisionKind.valueOf(kind),rs.getLong("decision_revision"),at.toInstant()));
          return new Cycle(schedule,decision);
        }, Objects.requireNonNull(owner),Objects.requireNonNull(cycle));
    if (rows.size()>1) throw new IllegalStateException("cycle source identity conflict");
    return rows.stream().findFirst(); // No row means unrecorded, never invented history.
  }

  private void requireOwningTransaction() {
    Object resource = TransactionSynchronizationManager.getResource(source);
    if (!TransactionSynchronizationManager.isActualTransactionActive() || !(resource instanceof ConnectionHolder holder))
      throw new IllegalStateException("cycle source requires owning transaction");
    try {
      if (holder.getConnection().getAutoCommit()) throw new IllegalStateException("cycle source requires owning transaction");
    } catch (SQLException failure) {
      throw new IllegalStateException("cycle source transaction unavailable",failure);
    }
  }
}
