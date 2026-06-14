package voice.backend.auth.repository;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

public class JdbcAccountRepository implements AccountRepository {
  private static final RowMapper<Account> ROW_MAPPER =
      (rs, rowNum) ->
          new Account(
              rs.getObject("id", UUID.class),
              rs.getString("email"),
              rs.getString("phone"),
              rs.getString("password_hash"),
              rs.getString("type"),
              rs.getString("status"),
              rs.getBytes("totp_secret"),
              rs.getBoolean("totp_enabled"),
              rs.getTimestamp("created_at").toInstant());

  private final NamedParameterJdbcTemplate jdbc;

  public JdbcAccountRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @Override
  public Account create(String email, String phone, String passwordHash, String type) {
    MapSqlParameterSource params =
        new MapSqlParameterSource()
            .addValue("email", email)
            .addValue("phone", phone)
            .addValue("passwordHash", passwordHash)
            .addValue("type", type);
    try {
      return jdbc.queryForObject(
          """
          INSERT INTO accounts (email, phone, password_hash, type, status)
          VALUES (:email, :phone, :passwordHash, :type, 'active')
          RETURNING id, email, phone, password_hash, type, status, totp_secret, totp_enabled, created_at
          """,
          params,
          ROW_MAPPER);
    } catch (DuplicateKeyException ex) {
      throw new IllegalArgumentException("duplicate account identifier", ex);
    }
  }

  @Override
  public Optional<Account> findByEmail(String email) {
    if (email == null) {
      return Optional.empty();
    }
    return jdbc.query(
            """
            SELECT id, email, phone, password_hash, type, status, totp_secret, totp_enabled, created_at
            FROM accounts WHERE email = :email LIMIT 1
            """,
            new MapSqlParameterSource("email", email),
            ROW_MAPPER)
        .stream()
        .findFirst();
  }

  @Override
  public Optional<Account> findByPhone(String phone) {
    if (phone == null) {
      return Optional.empty();
    }
    return jdbc.query(
            """
            SELECT id, email, phone, password_hash, type, status, totp_secret, totp_enabled, created_at
            FROM accounts WHERE phone = :phone LIMIT 1
            """,
            new MapSqlParameterSource("phone", phone),
            ROW_MAPPER)
        .stream()
        .findFirst();
  }

  @Override
  public Optional<Account> findById(String id) {
    try {
      UUID uuid = UUID.fromString(id);
      return jdbc.query(
              """
              SELECT id, email, phone, password_hash, type, status, totp_secret, totp_enabled, created_at
              FROM accounts WHERE id = :id LIMIT 1
              """,
              new MapSqlParameterSource("id", uuid),
              ROW_MAPPER)
          .stream()
          .findFirst();
    } catch (IllegalArgumentException ex) {
      return Optional.empty();
    }
  }

  @Override
  public void saveTotpSecret(UUID accountId, byte[] encryptedSecret, boolean enabled) {
    jdbc.update(
        """
        UPDATE accounts
        SET totp_secret = :secret, totp_enabled = :enabled, updated_at = now()
        WHERE id = :id
        """,
        new MapSqlParameterSource()
            .addValue("id", accountId)
            .addValue("secret", encryptedSecret)
            .addValue("enabled", enabled));
  }

  @Override
  public void setTotpEnabled(UUID accountId, boolean enabled) {
    jdbc.update(
        """
        UPDATE accounts
        SET totp_enabled = :enabled, updated_at = now()
        WHERE id = :id
        """,
        new MapSqlParameterSource()
            .addValue("id", accountId)
            .addValue("enabled", enabled));
  }

  @Override
  public void setStatus(UUID accountId, String status) {
    jdbc.update(
        """
        UPDATE accounts
        SET status = :status, updated_at = now()
        WHERE id = :id
        """,
        new MapSqlParameterSource()
            .addValue("id", accountId)
            .addValue("status", status));
  }
}
