package voice.backend.auth.roleidentityauthority;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.Map;
import java.util.Objects;
import javax.sql.DataSource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Registers a preloaded owner keyset; performs no secret-provider or peer I/O under SQL locks. */
public final class AuthIdentityKeysetBootstrap {
  private final RoleIdentitySelectors selectors;
  private final JdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final AuthIdentityCatalog catalog = new AuthIdentityCatalog();

  public AuthIdentityKeysetBootstrap(DataSource source, RoleIdentitySelectors selectors) {
    this.selectors = selectors;
    jdbc = source == null ? null : new JdbcTemplate(source);
    if (jdbc != null) jdbc.setQueryTimeout(2);
    if (selectors == null) { transactions = null; return; }
    Objects.requireNonNull(source);
    transactions = new TransactionTemplate(new DataSourceTransactionManager(source));
    transactions.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);
    transactions.setTimeout(3);
  }

  public void initialize() {
    if (jdbc == null) return;
    if (TransactionSynchronizationManager.isActualTransactionActive()) {
      throw new IllegalStateException("identity keyset bootstrap requires independent transaction");
    }
    if (selectors == null) {
      if (!Boolean.FALSE.equals(jdbc.queryForObject(
          "SELECT enforcement_enabled FROM role_identity_owner_state", Boolean.class))) {
        throw new IllegalStateException("identity owner keyset unavailable");
      }
      return;
    }
    final String fingerprints;
    try {
      var keys = new ArrayList<Map<String, Object>>();
      var local = selectors.keyFingerprints();
      for (int version : selectors.retainedVersions()) {
        keys.add(Map.of("version", version, "fingerprint", HexFormat.of().formatHex(local.get(version))));
      }
      fingerprints = new ObjectMapper().writeValueAsString(keys);
    } catch (Exception ex) {
      throw new IllegalStateException("identity owner keyset unavailable");
    }
    transactions.executeWithoutResult(status -> {
      catalog.validate(jdbc);
      jdbc.execute("SET LOCAL lock_timeout='2s'");
      jdbc.execute("SET LOCAL statement_timeout='2s'");
      Long revision = jdbc.queryForObject("SELECT role_identity_register_keyset(?,?,?::jsonb)",
          Long.class, selectors.keysetRevision(), selectors.activeVersion(), fingerprints);
      if (revision == null || revision != selectors.keysetRevision()
          || jdbc.update("UPDATE role_identity_owner_state SET enforcement_enabled=true "
              + "WHERE keyset_revision=? AND active_version=?",
              selectors.keysetRevision(), selectors.activeVersion()) != 1) {
        throw new IllegalStateException("identity owner keyset unavailable");
      }
    });
  }
}
