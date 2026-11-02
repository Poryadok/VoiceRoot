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
      // Order issuance at PostgreSQL timestamp precision under the same lock,
      // even when application clocks repeat or move backward. Invalid history
      // must fail the transaction rather than commit predecessor invalidation.
      return jdbc.queryForObject(
        """
        INSERT INTO otp_codes (account_id, code, type, expires_at, created_at)
        SELECT :accountId, decode(:codeHash, 'hex'), :type, :expiresAt,
               GREATEST(:createdAt, COALESCE(MAX(created_at) + INTERVAL '1 microsecond', :createdAt))
        FROM otp_codes WHERE account_id = :accountId AND type = :type
        HAVING MAX(created_at) IS NULL OR isfinite(MAX(created_at))
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
    // Rank all history before checking usability, including rows written before
    // predecessor invalidation existed. A spent/expired newest code is a tombstone.
    var latest = jdbc.query(
            """
            SELECT id, account_id, encode(code, 'hex') AS code_hash, type, expires_at, used_at
            FROM otp_codes
            WHERE account_id = :accountId
              AND type = :type
              AND isfinite(created_at)
              AND created_at = (SELECT MAX(created_at) FROM otp_codes
                                WHERE account_id = :accountId AND type = :type)
            LIMIT 2
            """,
            new MapSqlParameterSource()
                .addValue("accountId", accountId)
                .addValue("type", type),
            ROW_MAPPER);
    // Legacy equal issuance timestamps have no trustworthy ordering. Fail closed.
    return latest.size() == 1 ? Optional.of(latest.getFirst()).filter(r -> r.isUsable(now))
        : Optional.empty();
  }

  @Override
  public void markUsed(UUID id, Instant usedAt) {
    int consumed = jdbc.update(
        "UPDATE otp_codes SET used_at = :usedAt WHERE id = :id AND used_at IS NULL AND expires_at > :usedAt",
        new MapSqlParameterSource().addValue("id", id).addValue("usedAt", Timestamp.from(usedAt)));
    if (consumed != 1) throw new AuthException("invalid_otp");
  }
}
