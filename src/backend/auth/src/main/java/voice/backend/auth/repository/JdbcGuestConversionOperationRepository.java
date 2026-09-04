package voice.backend.auth.repository;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.Objects;
import java.util.UUID;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

public class JdbcGuestConversionOperationRepository implements GuestConversionOperationRepository {
  private static final RowMapper<GuestConversionOperation> ROW_MAPPER =
      (rs, rowNum) ->
          new GuestConversionOperation(
              rs.getObject("operation_id", UUID.class),
              rs.getObject("account_id", UUID.class),
              rs.getObject("otp_code_id", UUID.class),
              GuestConversionState.valueOf(rs.getString("state")),
              rs.getInt("attempt_count"),
              timestamp(rs.getTimestamp("next_attempt_at")),
              timestamp(rs.getTimestamp("locked_until")),
              rs.getString("last_error_code"),
              timestamp(rs.getTimestamp("user_marked_at")),
              timestamp(rs.getTimestamp("auth_promoted_at")),
              timestamp(rs.getTimestamp("event_published_at")),
              timestamp(rs.getTimestamp("created_at")),
              timestamp(rs.getTimestamp("updated_at")));

  private final NamedParameterJdbcTemplate jdbc;

  public JdbcGuestConversionOperationRepository(NamedParameterJdbcTemplate jdbc) {
    this.jdbc = Objects.requireNonNull(jdbc, "jdbc");
  }

  @Override
  public GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now) {
    Objects.requireNonNull(accountId, "accountId");
    Objects.requireNonNull(otpCodeId, "otpCodeId");
    Objects.requireNonNull(now, "now");

    MapSqlParameterSource parameters =
        new MapSqlParameterSource()
            .addValue("operationId", UUID.randomUUID())
            .addValue("accountId", accountId)
            .addValue("otpCodeId", otpCodeId)
            .addValue("now", Timestamp.from(now));
    jdbc.update(
        """
        INSERT INTO guest_conversion_operations (
            operation_id, account_id, otp_code_id, state, attempt_count,
            next_attempt_at, created_at, updated_at)
        VALUES (
            :operationId, :accountId, :otpCodeId, 'PENDING_USER', 0,
            :now, :now, :now)
        ON CONFLICT DO NOTHING
        """,
        parameters);

    java.util.Optional<GuestConversionOperation> existingByAccount = findByAccountId(accountId);
    if (existingByAccount.isPresent()) {
      return existingByAccount.get();
    }
    boolean otpAlreadyBound =
        jdbc
            .query(
                """
                SELECT operation_id, account_id, otp_code_id, state, attempt_count,
                       next_attempt_at, locked_until, last_error_code, user_marked_at,
                       auth_promoted_at, event_published_at, created_at, updated_at
                FROM guest_conversion_operations
                WHERE otp_code_id = :otpCodeId
                """,
                new MapSqlParameterSource("otpCodeId", otpCodeId),
                ROW_MAPPER)
            .stream()
            .findFirst()
            .isPresent();
    if (otpAlreadyBound) {
      throw new IllegalArgumentException("OTP code is already bound to another account");
    }
    throw new IllegalStateException(
        "guest conversion operation was not visible after a successful insert");
  }

  private java.util.Optional<GuestConversionOperation> findByAccountId(UUID accountId) {
    return jdbc
        .query(
            """
            SELECT operation_id, account_id, otp_code_id, state, attempt_count,
                   next_attempt_at, locked_until, last_error_code, user_marked_at,
                   auth_promoted_at, event_published_at, created_at, updated_at
            FROM guest_conversion_operations
            WHERE account_id = :accountId
            """,
            new MapSqlParameterSource("accountId", accountId),
            ROW_MAPPER)
        .stream()
        .findFirst();
  }

  private static Instant timestamp(Timestamp value) {
    return value == null ? null : value.toInstant();
  }
}
