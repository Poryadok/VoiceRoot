package voice.backend.auth.repository;

import java.util.List;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

public class JdbcBackupCodeRepository implements BackupCodeRepository {
  private final NamedParameterJdbcTemplate jdbc;

  public JdbcBackupCodeRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @Override
  public void replaceCodes(UUID accountId, List<String> codeHashes) {
    jdbc.update(
        "DELETE FROM backup_codes WHERE account_id = :accountId",
        new MapSqlParameterSource("accountId", accountId));
    for (String hash : codeHashes) {
      jdbc.update(
          """
          INSERT INTO backup_codes (account_id, code_hash)
          VALUES (:accountId, :codeHash)
          """,
          new MapSqlParameterSource()
              .addValue("accountId", accountId)
              .addValue("codeHash", hash));
    }
  }

  @Override
  public boolean consumeCode(UUID accountId, String codeHash) {
    int updated = jdbc.update(
        """
        UPDATE backup_codes
        SET used_at = now()
        WHERE account_id = :accountId
          AND code_hash = :codeHash
          AND used_at IS NULL
        """,
        new MapSqlParameterSource()
            .addValue("accountId", accountId)
            .addValue("codeHash", codeHash));
    return updated > 0;
  }
}
