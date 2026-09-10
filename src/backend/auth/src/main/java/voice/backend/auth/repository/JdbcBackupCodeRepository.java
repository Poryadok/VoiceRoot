package voice.backend.auth.repository;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

public class JdbcBackupCodeRepository implements BackupCodeRepository {
  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;

  public JdbcBackupCodeRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
    this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
        Objects.requireNonNull(jdbc.getJdbcTemplate().getDataSource(), "backup code datasource")));
  }

  @Override
  public void replaceCodes(UUID accountId, List<String> codeHashes) {
    transactions.executeWithoutResult(status -> {
      lockAccount(accountId);
      jdbc.update("DELETE FROM backup_codes WHERE account_id = :accountId", Map.of("accountId", accountId));
      for (String hash : codeHashes) {
        jdbc.update("""
            INSERT INTO backup_codes (account_id, code_hash) VALUES (:accountId, :codeHash)
            """, new MapSqlParameterSource().addValue("accountId", accountId).addValue("codeHash", hash));
      }
    });
  }

  @Override
  public boolean consumeCode(UUID accountId, String codeHash) {
    return Boolean.TRUE.equals(transactions.execute(status -> {
      lockAccount(accountId);
      return jdbc.update("""
          UPDATE backup_codes SET used_at = now()
          WHERE account_id = :accountId AND code_hash = :codeHash AND used_at IS NULL
          """, new MapSqlParameterSource().addValue("accountId", accountId).addValue("codeHash", codeHash)) > 0;
    }));
  }

  private void lockAccount(UUID accountId) {
    // All paths, including legacy login and replacement, use account -> backup ordering.
    // REQUIRED joins proof issuance so a later proof failure also rolls back factor use.
    jdbc.query("SELECT id FROM accounts WHERE id=:accountId FOR UPDATE", Map.of("accountId", accountId),
        (rs, row) -> rs.getObject("id", UUID.class));
  }
}
