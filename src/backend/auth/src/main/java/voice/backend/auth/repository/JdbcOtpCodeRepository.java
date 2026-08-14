package voice.backend.auth.repository;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

public class JdbcOtpCodeRepository implements OtpCodeRepository {
  private static final RowMapper<OtpCodeRecord> ROW_MAPPER =
      (rs, rowNum) -> {
        Timestamp usedTs = rs.getTimestamp("used_at");
        return new OtpCodeRecord(
            rs.getObject("id", UUID.class),
            rs.getObject("account_id", UUID.class),
            rs.getString("code_hash"),
            rs.getString("type"),
            rs.getTimestamp("expires_at").toInstant(),
            usedTs == null ? null : usedTs.toInstant());
      };

  private final NamedParameterJdbcTemplate jdbc;

  public JdbcOtpCodeRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @Override
  public OtpCodeRecord create(UUID accountId, String codeHash, String type, Instant expiresAt, Instant now) {
    return jdbc.queryForObject(
        """
        INSERT INTO otp_codes (account_id, code, type, expires_at, created_at)
        VALUES (:accountId, decode(:codeHash, 'hex'), :type, :expiresAt, :createdAt)
        RETURNING id, account_id, encode(code, 'hex') AS code_hash, type, expires_at, used_at
        """,
        new MapSqlParameterSource()
            .addValue("accountId", accountId)
            .addValue("codeHash", codeHash)
            .addValue("type", type)
            .addValue("expiresAt", Timestamp.from(expiresAt))
            .addValue("createdAt", Timestamp.from(now)),
        ROW_MAPPER);
  }

  @Override
  public Optional<OtpCodeRecord> findLatestValid(UUID accountId, String type, Instant now) {
    return jdbc.query(
            """
            SELECT id, account_id, encode(code, 'hex') AS code_hash, type, expires_at, used_at
            FROM otp_codes
            WHERE account_id = :accountId
              AND type = :type
              AND used_at IS NULL
              AND expires_at > :now
            ORDER BY expires_at DESC
            LIMIT 1
            """,
            new MapSqlParameterSource()
                .addValue("accountId", accountId)
                .addValue("type", type)
                .addValue("now", Timestamp.from(now)),
            ROW_MAPPER)
        .stream()
        .findFirst();
  }

  @Override
  public void markUsed(UUID id, Instant usedAt) {
    jdbc.update(
        "UPDATE otp_codes SET used_at = :usedAt WHERE id = :id",
        new MapSqlParameterSource().addValue("id", id).addValue("usedAt", Timestamp.from(usedAt)));
  }
}
