package voice.backend.auth.sdkidentity;

import java.sql.Timestamp;
import java.time.Clock;
import java.util.Map;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.RegistrationIntentBinder;

/** Consumes only inside the same transaction that just created the normal Voice account. */
public final class JdbcSdkRegistrationIntentBinder implements RegistrationIntentBinder {
  private final NamedParameterJdbcTemplate jdbc;
  private final Clock clock;

  public JdbcSdkRegistrationIntentBinder(NamedParameterJdbcTemplate jdbc, Clock clock) {
    this.jdbc = jdbc;
    this.clock = clock;
  }

  @Override
  public void bind(UUID intentId, UUID newAccountId) {
    if (!TransactionSynchronizationManager.isActualTransactionActive()) throw new AuthException("auth_unavailable");
    if (intentId == null || newAccountId == null) throw invalid();
    var accounts = jdbc.queryForList("""
        SELECT id FROM accounts WHERE id=:account AND status='active'
          AND (type='regular' OR (type='guest' AND regular_email_verification_pending=TRUE))
        """, Map.of("account", newAccountId));
    if (accounts.isEmpty()) throw invalid();
    var keys = jdbc.queryForList("""
        SELECT o.operation_id,o.source_account_id FROM sdk_registration_intents r
        JOIN sdk_conversion_operations o ON o.operation_id=r.operation_id WHERE r.intent_id=:intent
        """, Map.of("intent", intentId));
    if (keys.isEmpty()) throw invalid();
    Map<String, Object> key = keys.getFirst();
    // Match every SDK operation: identity -> operation -> intent/device.
    jdbc.query("SELECT account_id FROM sdk_identities WHERE account_id=:source_account_id FOR UPDATE", key, rs -> { });
    jdbc.query("SELECT operation_id FROM sdk_conversion_operations WHERE operation_id=:operation_id FOR UPDATE", key, rs -> { });
    var rows = jdbc.queryForList("""
        SELECT r.expires_at,r.consumed_at FROM sdk_registration_intents r
        JOIN sdk_conversion_operations o ON o.operation_id=r.operation_id
        JOIN sdk_identities i ON i.account_id=o.source_account_id
        JOIN sdk_devices d ON d.device_id=o.device_id AND d.account_id=i.account_id
        WHERE r.intent_id=:intent AND o.mode='new' AND o.state='prepared'
          AND o.registered_account_id IS NULL AND i.status='active'
          AND i.ownership_generation=o.source_generation AND d.revoked_at IS NULL
        FOR UPDATE OF r,d
        """, Map.of("intent", intentId));
    if (rows.isEmpty() || rows.getFirst().get("consumed_at") != null
        || !((Timestamp) rows.getFirst().get("expires_at")).toInstant().isAfter(clock.instant())) throw invalid();
    if (jdbc.update("""
        UPDATE sdk_registration_intents SET account_id=:account,consumed_at=:now
        WHERE intent_id=:intent AND consumed_at IS NULL AND expires_at>:now
        """, Map.of("account", newAccountId, "now", Timestamp.from(clock.instant()), "intent", intentId)) != 1) throw invalid();
    jdbc.update("""
        UPDATE sdk_conversion_operations SET registered_account_id=:account,revision=revision+1
        WHERE operation_id=:operation AND registered_account_id IS NULL
        """, Map.of("account", newAccountId, "operation", key.get("operation_id")));
  }

  private static AuthException invalid() { return new AuthException("validation_failed"); }
}
