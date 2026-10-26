package voice.backend.auth.roleidentityauthority;

import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import java.util.Objects;
import java.util.UUID;
import javax.sql.DataSource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Complete fresh Auth input observation. No paid eligibility or CURRENT result is minted here. */
public final class AuthSubscriptionLifecycleOwnerStore {
  /** Retained capture coverage only; neither value asserts paid eligibility. */
  public enum Coverage { UNRECORDED, CAPTURED }
  public record Fact(UUID accountId, long sourceRevision, UUID eventId, UUID deletionCycleId,
      String kind, long sessionEpoch, Instant scheduledAt, Instant purgeAt, Instant effectiveAt, Instant capturedAt) {}
  public record Snapshot(UUID accountId, String accountType, String status, boolean emailVerificationPending,
      long sessionEpoch, long securityRevision, Instant deletedAt, Instant observedAt,
      Coverage coverage, Fact head, List<Fact> facts) {
    public Snapshot { facts = List.copyOf(facts); }
  }
  private static final int MAX_FACTS = 256;
  private final JdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final SubscriptionLifecycleCatalog catalog;

  public AuthSubscriptionLifecycleOwnerStore(DataSource source, AuthIdentityDatabasePolicy policy) {
    Objects.requireNonNull(source);
    catalog = new SubscriptionLifecycleCatalog(policy);
    jdbc = new JdbcTemplate(source);
    transactions = new TransactionTemplate(new DataSourceTransactionManager(source));
    transactions.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);
    transactions.setTimeout(1);
  }

  public Snapshot observe(UUID accountId, long minimumRevision) {
    if (accountId == null || accountId.equals(new UUID(0,0)) || minimumRevision < 0) throw new IllegalArgumentException("invalid Subscription owner observation");
    if (TransactionSynchronizationManager.isActualTransactionActive()) throw new IllegalStateException("Subscription owner observation requires independent transaction");
    return transactions.execute(status -> read(accountId, minimumRevision));
  }

  private Snapshot read(UUID accountId, long minimumRevision) {
    catalog.validate(jdbc);
    // This account mutex is the linearization point, shared with actual delete,
    // restore and capture triggers. Fresh RC reads after a wait see their commit.
    SubscriptionLifecycleCatalog.budget(jdbc);
    var accounts = jdbc.queryForList("SELECT id,type,status,regular_email_verification_pending,session_epoch,security_revision,deleted_at FROM public.accounts WHERE id=? FOR UPDATE", accountId);
    SubscriptionLifecycleCatalog.require(accounts.size() == 1);
    var account = accounts.getFirst();
    SubscriptionLifecycleCatalog.budget(jdbc);
    var heads = jdbc.query("SELECT * FROM public.account_subscription_lifecycle_heads WHERE account_id=? FOR SHARE", (rs, row) -> fact(rs), accountId);
    SubscriptionLifecycleCatalog.budget(jdbc);
    var facts = jdbc.query("SELECT * FROM public.account_subscription_lifecycle_outbox WHERE account_id=? ORDER BY source_revision LIMIT ? FOR SHARE", (rs, row) -> {
      SubscriptionLifecycleCatalog.require(rs.getInt("protocol_version") == 1);
      return fact(rs);
    }, accountId, MAX_FACTS+1);
    SubscriptionLifecycleCatalog.require(heads.size() <= 1 && facts.size() <= MAX_FACTS);
    Fact head = heads.isEmpty() ? null : heads.getFirst();
    String state = (String)account.get("status");
    Instant deletedAt = instant((Timestamp)account.get("deleted_at"));
    long epoch = ((Number)account.get("session_epoch")).longValue();
    long security = ((Number)account.get("security_revision")).longValue();
    SubscriptionLifecycleCatalog.require(epoch > 0 && security > 0 && state != null);
    SubscriptionLifecycleCatalog.require(state.equals("deleted") == (deletedAt != null));
    if (head == null) {
      SubscriptionLifecycleCatalog.require(facts.isEmpty() && minimumRevision == 0);
    } else {
      SubscriptionLifecycleCatalog.require(head.sourceRevision() >= minimumRevision && !facts.isEmpty() && head.equals(facts.getLast()));
      Fact previous = null;
      long revision = 0;
      for (Fact fact : facts) {
        SubscriptionLifecycleCatalog.require(accountId.equals(fact.accountId()) && fact.sourceRevision() == ++revision && fact.sessionEpoch() > 0 && fact.sessionEpoch() <= epoch);
        SubscriptionLifecycleCatalog.require(fact.eventId() != null && fact.deletionCycleId() != null && fact.scheduledAt() != null && fact.purgeAt() != null && fact.effectiveAt() != null && fact.capturedAt() != null);
        SubscriptionLifecycleCatalog.require(fact.purgeAt().equals(fact.scheduledAt().plusSeconds(720L*3600)));
        if (fact.kind().equals("DELETE_SCHEDULED")) {
          SubscriptionLifecycleCatalog.require(fact.eventId().equals(fact.deletionCycleId()) && fact.effectiveAt().equals(fact.scheduledAt())
              && (previous == null || previous.kind().equals("RESTORE_REQUESTED")));
        } else {
          SubscriptionLifecycleCatalog.require(fact.kind().equals("RESTORE_REQUESTED") && previous != null && previous.kind().equals("DELETE_SCHEDULED")
              && fact.deletionCycleId().equals(previous.deletionCycleId()) && fact.scheduledAt().equals(previous.scheduledAt()) && fact.purgeAt().equals(previous.purgeAt()));
        }
        previous = fact;
      }
      SubscriptionLifecycleCatalog.require(state.equals("deleted") ? head.kind().equals("DELETE_SCHEDULED") && deletedAt.equals(head.scheduledAt()) : head.kind().equals("RESTORE_REQUESTED"));
    }
    SubscriptionLifecycleCatalog.budget(jdbc);
    Instant observedAt = jdbc.queryForObject("SELECT clock_timestamp()", Timestamp.class).toInstant();
    for (var fact : facts) SubscriptionLifecycleCatalog.require(!fact.capturedAt().isAfter(observedAt) && !fact.effectiveAt().isAfter(observedAt));
    return new Snapshot(accountId, (String)account.get("type"), state, Boolean.TRUE.equals(account.get("regular_email_verification_pending")), epoch, security,
        deletedAt, observedAt, head == null ? Coverage.UNRECORDED : Coverage.CAPTURED, head, facts);
  }

  private static Fact fact(ResultSet rs) throws SQLException {
    return new Fact(rs.getObject("account_id",UUID.class),rs.getLong("source_revision"),rs.getObject("event_id",UUID.class),
        rs.getObject("deletion_cycle_id",UUID.class),rs.getString("kind"),rs.getLong("session_epoch"),
        instant(rs.getTimestamp("scheduled_at")),instant(rs.getTimestamp("purge_at")),instant(rs.getTimestamp("effective_at")),instant(rs.getTimestamp("captured_at")));
  }
  private static Instant instant(Timestamp value) { return value == null ? null : value.toInstant(); }
}
