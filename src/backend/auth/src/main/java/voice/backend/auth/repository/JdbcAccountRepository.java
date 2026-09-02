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
              rs.getTimestamp("created_at").toInstant(),
              rs.getTimestamp("deleted_at") == null
                  ? null
                  : rs.getTimestamp("deleted_at").toInstant());

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
          RETURNING id, email, phone, password_hash, type, status, totp_secret, totp_enabled, created_at, deleted_at
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
            SELECT id, email, phone, password_hash, type, status, totp_secret, totp_enabled, created_at, deleted_at
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
            SELECT id, email, phone, password_hash, type, status, totp_secret, totp_enabled, created_at, deleted_at
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
              SELECT id, email, phone, password_hash, type, status, totp_secret, totp_enabled, created_at, deleted_at
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

  @Override
  public Account convertGuest(UUID accountId, String email, String phone, String passwordHash) {
    MapSqlParameterSource params =
        new MapSqlParameterSource()
            .addValue("id", accountId)
            .addValue("email", email)
            .addValue("phone", phone)
            .addValue("passwordHash", passwordHash);
    try {
      return jdbc.queryForObject(
          """
          UPDATE accounts
          SET email = :email, phone = :phone, password_hash = :passwordHash, type = 'regular', updated_at = now()
          WHERE id = :id AND type = 'guest'
          RETURNING id, email, phone, password_hash, type, status, totp_secret, totp_enabled, created_at, deleted_at
          """,
          params,
          ROW_MAPPER);
    } catch (org.springframework.dao.EmptyResultDataAccessException ex) {
      throw new IllegalArgumentException("not a guest account", ex);
    } catch (DuplicateKeyException ex) {
      throw new IllegalArgumentException("duplicate account identifier", ex);
    }
  }

  @Override
  public void updatePasswordHash(UUID accountId, String passwordHash) {
    int updated =
        jdbc.update(
            """
            UPDATE accounts
            SET password_hash = :passwordHash, updated_at = now()
            WHERE id = :id
            """,
            new MapSqlParameterSource()
                .addValue("id", accountId)
                .addValue("passwordHash", passwordHash));
    if (updated == 0) {
      throw new IllegalArgumentException("account not found");
    }
  }

  @Override
  public void touchLastOnlineAt(UUID accountId, Instant at) {
    jdbc.update(
        """
        UPDATE accounts
        SET last_online_at = :at, updated_at = now()
        WHERE id = :id
        """,
        new MapSqlParameterSource().addValue("id", accountId).addValue("at", java.sql.Timestamp.from(at)));
  }

  @Override
  public int deactivateExpiredGuests(Instant lastOnlineBefore) {
    return jdbc.update(
        """
        UPDATE accounts
        SET status = 'deleted', deleted_at = now(), updated_at = now()
        WHERE type = 'guest'
          AND status = 'active'
          AND last_online_at IS NOT NULL
          AND last_online_at < :cutoff
        """,
        new MapSqlParameterSource("cutoff", java.sql.Timestamp.from(lastOnlineBefore)));
  }

  @Override
  public void markDeleted(UUID accountId, Instant deletedAt) {
    jdbc.update(
        """
        UPDATE accounts
        SET status = 'deleted', deleted_at = :deletedAt, updated_at = now()
        WHERE id = :id
        """,
        new MapSqlParameterSource()
            .addValue("id", accountId)
            .addValue("deletedAt", java.sql.Timestamp.from(deletedAt)));
  }

  @Override
  public void restoreDeleted(UUID accountId) {
    jdbc.update(
        """
        UPDATE accounts
        SET status = 'active', deleted_at = NULL, updated_at = now()
        WHERE id = :id
        """,
        new MapSqlParameterSource("id", accountId));
  }

  @Override
  public Optional<Instant> getGuestReminderLastShownAt(UUID accountId) {
    java.util.List<Instant> rows =
        jdbc.query(
            """
            SELECT guest_reminder_last_shown_at
            FROM accounts WHERE id = :id LIMIT 1
            """,
            new MapSqlParameterSource("id", accountId),
            (rs, rowNum) -> {
              java.sql.Timestamp ts = rs.getTimestamp("guest_reminder_last_shown_at");
              return ts == null ? null : ts.toInstant();
            });
    if (rows.isEmpty()) {
      return Optional.empty();
    }
    return Optional.ofNullable(rows.get(0));
  }

  @Override
  public void markGuestReminderShown(UUID accountId, Instant shownAt) {
    jdbc.update(
        """
        UPDATE accounts
        SET guest_reminder_last_shown_at = :shownAt, updated_at = now()
        WHERE id = :id
        """,
        new MapSqlParameterSource()
            .addValue("id", accountId)
            .addValue("shownAt", java.sql.Timestamp.from(shownAt)));
  }

  @Override
  public java.util.Set<UUID> findDeletedAmong(java.util.Collection<UUID> accountIds) {
    if (accountIds == null || accountIds.isEmpty()) {
      return java.util.Set.of();
    }
    java.util.List<UUID> ids = accountIds.stream().filter(java.util.Objects::nonNull).distinct().toList();
    if (ids.isEmpty()) {
      return java.util.Set.of();
    }
    return new java.util.HashSet<>(
        jdbc.query(
            """
            SELECT id FROM accounts
            WHERE id IN (:ids) AND deleted_at IS NOT NULL
            """,
            new MapSqlParameterSource("ids", ids),
            (rs, rowNum) -> rs.getObject("id", UUID.class)));
  }
}
