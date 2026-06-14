package voice.backend.auth.repository;

import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

public class JdbcE2EKeyBackupRepository implements E2EKeyBackupRepository {
  private final NamedParameterJdbcTemplate jdbc;

  public JdbcE2EKeyBackupRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @Override
  public void put(UUID accountId, String encryptedBlob, String passwordHint) {
    jdbc.update(
        """
        INSERT INTO e2e_key_backups (account_id, encrypted_blob, password_hint, updated_at)
        VALUES (:accountId, :encryptedBlob, :passwordHint, now())
        ON CONFLICT (account_id) DO UPDATE
        SET encrypted_blob = EXCLUDED.encrypted_blob,
            password_hint = EXCLUDED.password_hint,
            updated_at = now()
        """,
        new MapSqlParameterSource()
            .addValue("accountId", accountId)
            .addValue("encryptedBlob", encryptedBlob)
            .addValue("passwordHint", passwordHint));
  }

  @Override
  public Optional<E2EKeyBackupRecord> get(UUID accountId) {
    return jdbc.query(
            """
            SELECT encrypted_blob, password_hint
            FROM e2e_key_backups
            WHERE account_id = :accountId
            """,
            new MapSqlParameterSource("accountId", accountId),
            (rs, rowNum) ->
                new E2EKeyBackupRecord(
                    rs.getString("encrypted_blob"),
                    rs.getString("password_hint")))
        .stream()
        .findFirst();
  }
}
