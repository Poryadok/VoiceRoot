package voice.backend.auth.repository;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.service.AuthException;

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
  private final TransactionTemplate transactions;

  public JdbcOtpCodeRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
    this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
        java.util.Objects.requireNonNull(jdbc.getJdbcTemplate().getDataSource())));
  }

  @Override
  public OtpCodeRecord create(UUID accountId, String codeHash, String type, Instant expiresAt, Instant now) {
    return transactions.execute(status -> {
      // Also serialize first-ever sends, when no predecessor row exists to lock.
      jdbc.queryForList("SELECT pg_advisory_xact_lock(hashtextextended(:key, 0))",
          new MapSqlParameterSource("key", "auth-otp:" + accountId + ":" + type));
      jdbc.update("""
          UPDATE otp_codes SET used_at = :now
          WHERE account_id = :accountId AND type = :type AND used_at IS NULL
          """, new MapSqlParameterSource("accountId", accountId)
              .addValue("type", type).addValue("now", Timestamp.from(now)));
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
    });
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
    int consumed = jdbc.update(
        "UPDATE otp_codes SET used_at = :usedAt WHERE id = :id AND used_at IS NULL AND expires_at > :usedAt",
        new MapSqlParameterSource().addValue("id", id).addValue("usedAt", Timestamp.from(usedAt)));
    if (consumed != 1) throw new AuthException("invalid_otp");
  }
}
