package voice.backend.auth.repository;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

public class JdbcRefreshTokenRepository implements RefreshTokenRepository {
  private static final RowMapper<RefreshTokenRecord> ROW_MAPPER =
      (rs, rowNum) -> {
        Instant revokedAt = null;
        Timestamp revokedTs = rs.getTimestamp("revoked_at");
        if (revokedTs != null) {
          revokedAt = revokedTs.toInstant();
        }
        return new RefreshTokenRecord(
            rs.getObject("id", UUID.class),
            rs.getObject("account_id", UUID.class),
            rs.getString("token_hash"),
            rs.getString("device_info_json"),
            rs.getString("access_jti"),
            rs.getTimestamp("expires_at").toInstant(),
            rs.getTimestamp("created_at").toInstant(),
            revokedAt);
      };

  private final NamedParameterJdbcTemplate jdbc;

  public JdbcRefreshTokenRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @Override
  public RefreshTokenRecord create(
      UUID accountId, String tokenHash, String deviceInfoJson, String accessJti, Instant expiresAt, Instant now) {
    MapSqlParameterSource params =
        new MapSqlParameterSource()
            .addValue("accountId", accountId)
            .addValue("tokenHash", tokenHash)
            .addValue("deviceInfoJson", deviceInfoJson == null ? "{}" : deviceInfoJson)
            .addValue("accessJti", accessJti == null ? "" : accessJti)
            .addValue("expiresAt", Timestamp.from(expiresAt))
            .addValue("createdAt", Timestamp.from(now));
    return jdbc.queryForObject(
        """
        INSERT INTO refresh_tokens (account_id, token_hash, device_info, access_jti, expires_at, created_at)
        VALUES (:accountId, :tokenHash, CAST(:deviceInfoJson AS jsonb), :accessJti, :expiresAt, :createdAt)
        RETURNING id, account_id, token_hash, device_info::text AS device_info_json, access_jti, expires_at, created_at, revoked_at
        """,
        params,
        ROW_MAPPER);
  }

  @Override
  public Optional<RefreshTokenRecord> findByHash(String tokenHash) {
    return jdbc.query(
            """
            SELECT id, account_id, token_hash, device_info::text AS device_info_json, access_jti, expires_at, created_at, revoked_at
            FROM refresh_tokens WHERE token_hash = :tokenHash LIMIT 1
            """,
            new MapSqlParameterSource("tokenHash", tokenHash),
            ROW_MAPPER)
        .stream()
        .findFirst();
  }

  @Override
  public RefreshTokenRecord revoke(String tokenHash, Instant now) {
    int updated =
        jdbc.update(
            """
            UPDATE refresh_tokens SET revoked_at = :now
            WHERE token_hash = :tokenHash AND revoked_at IS NULL
            """,
            new MapSqlParameterSource()
                .addValue("now", Timestamp.from(now))
                .addValue("tokenHash", tokenHash));
    if (updated == 0) {
      return findByHash(tokenHash).orElse(null);
    }
    return findByHash(tokenHash).orElse(null);
  }
}
