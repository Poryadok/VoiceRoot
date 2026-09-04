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

  @Override
  public java.util.List<GuestConversionOperation> leaseDue(
      int batchSize, Instant now, Instant leaseUntil) {
    if (batchSize <= 0) {
      throw new IllegalArgumentException("batchSize must be positive");
    }
    Objects.requireNonNull(now, "now");
    Objects.requireNonNull(leaseUntil, "leaseUntil");
    if (!leaseUntil.isAfter(now)) {
      throw new IllegalArgumentException("leaseUntil must be after now");
    }

    return jdbc.query(
        """
        WITH eligible AS (
            SELECT operation_id, next_attempt_at, created_at
            FROM guest_conversion_operations
            WHERE state <> 'COMPLETED'
              AND next_attempt_at <= :now
              AND (locked_until IS NULL OR locked_until <= :now)
            ORDER BY next_attempt_at, created_at, operation_id
            LIMIT :batchSize
            FOR UPDATE SKIP LOCKED
        ), leased AS (
            UPDATE guest_conversion_operations operation
            SET locked_until = :leaseUntil
            FROM eligible
            WHERE operation.operation_id = eligible.operation_id
            RETURNING operation.*
        )
        SELECT leased.operation_id, leased.account_id, leased.otp_code_id, leased.state,
               leased.attempt_count, leased.next_attempt_at, leased.locked_until,
               leased.last_error_code, leased.user_marked_at, leased.auth_promoted_at,
               leased.event_published_at, leased.created_at, leased.updated_at
        FROM leased
        JOIN eligible ON eligible.operation_id = leased.operation_id
        ORDER BY eligible.next_attempt_at, eligible.created_at, eligible.operation_id
        """,
        new MapSqlParameterSource()
            .addValue("batchSize", batchSize)
            .addValue("now", Timestamp.from(now))
            .addValue("leaseUntil", Timestamp.from(leaseUntil)),
        ROW_MAPPER);
  }

  @Override
  public GuestConversionAdvanceResult advance(
      UUID operationId, GuestConversionState expectedState, Instant expectedLockedUntil, Instant now) {
    Objects.requireNonNull(operationId, "operationId");
    Objects.requireNonNull(expectedState, "expectedState");
    Objects.requireNonNull(expectedLockedUntil, "expectedLockedUntil");
    Objects.requireNonNull(now, "now");
    if (expectedState == GuestConversionState.COMPLETED) {
      throw new IllegalArgumentException("COMPLETED cannot be advanced");
    }

    MapSqlParameterSource parameters =
        new MapSqlParameterSource()
            .addValue("operationId", operationId)
            .addValue("expectedState", expectedState.name())
            .addValue("expectedLockedUntil", Timestamp.from(expectedLockedUntil))
            .addValue("now", Timestamp.from(now));
    String transition =
        expectedState == GuestConversionState.PENDING_USER
            ? """
              state = 'PENDING_EVENT',
              user_marked_at = :now,
              auth_promoted_at = :now,
              locked_until = NULL,
              updated_at = :now
              """
            : """
              state = 'COMPLETED',
              event_published_at = :now,
              locked_until = NULL,
              updated_at = :now
              """;
    String markerPreconditions =
        expectedState == GuestConversionState.PENDING_USER
            ? "user_marked_at IS NULL AND auth_promoted_at IS NULL AND event_published_at IS NULL"
            : "user_marked_at IS NOT NULL AND auth_promoted_at IS NOT NULL AND event_published_at IS NULL";

    boolean applied =
        !jdbc
            .query(
                """
                UPDATE guest_conversion_operations
                SET %s
                WHERE operation_id = :operationId
                  AND state = :expectedState
                  AND locked_until = :expectedLockedUntil
                  AND locked_until > :now
                  AND %s
                RETURNING operation_id
                """.formatted(transition, markerPreconditions),
                parameters,
                (rs, rowNum) -> rs.getObject("operation_id", UUID.class))
            .isEmpty();
    if (applied) {
      return GuestConversionAdvanceResult.APPLIED;
    }
    return classifyAdvanceMiss(operationId, expectedState);
  }

  private GuestConversionAdvanceResult classifyAdvanceMiss(
      UUID operationId, GuestConversionState expectedState) {
    java.util.Optional<GuestConversionOperation> operation =
        jdbc
            .query(
                """
                SELECT operation_id, account_id, otp_code_id, state, attempt_count,
                       next_attempt_at, locked_until, last_error_code, user_marked_at,
                       auth_promoted_at, event_published_at, created_at, updated_at
                FROM guest_conversion_operations
                WHERE operation_id = :operationId
                """,
                new MapSqlParameterSource("operationId", operationId),
                ROW_MAPPER)
            .stream()
            .findFirst();
    if (operation.isEmpty()) {
      return GuestConversionAdvanceResult.NOT_FOUND;
    }
    GuestConversionOperation current = operation.get();
    if (isTargetOrLater(current, expectedState)) {
      return GuestConversionAdvanceResult.ALREADY_APPLIED;
    }
    if (current.state() == expectedState && hasValidMarkers(current)) {
      return GuestConversionAdvanceResult.LEASE_LOST;
    }
    throw new IllegalStateException("guest conversion operation has invalid state markers");
  }

  private static boolean isTargetOrLater(
      GuestConversionOperation operation, GuestConversionState expectedState) {
    return switch (expectedState) {
      case PENDING_USER ->
          (operation.state() == GuestConversionState.PENDING_EVENT && hasValidMarkers(operation))
              || (operation.state() == GuestConversionState.COMPLETED && hasValidMarkers(operation));
      case PENDING_EVENT ->
          operation.state() == GuestConversionState.COMPLETED && hasValidMarkers(operation);
      case COMPLETED -> false;
    };
  }

  private static boolean hasValidMarkers(GuestConversionOperation operation) {
    return switch (operation.state()) {
      case PENDING_USER ->
          operation.userMarkedAt() == null
              && operation.authPromotedAt() == null
              && operation.eventPublishedAt() == null;
      case PENDING_EVENT ->
          operation.userMarkedAt() != null
              && operation.authPromotedAt() != null
              && operation.eventPublishedAt() == null;
      case COMPLETED ->
          operation.userMarkedAt() != null
              && operation.authPromotedAt() != null
              && operation.eventPublishedAt() != null;
    };
  }

  @Override
  public java.util.Optional<GuestConversionOperation> recordFailure(
      UUID operationId,
      Instant expectedLockedUntil,
      String errorCode,
      Instant nextAttemptAt,
      Instant now) {
    Objects.requireNonNull(operationId, "operationId");
    Objects.requireNonNull(expectedLockedUntil, "expectedLockedUntil");
    Objects.requireNonNull(errorCode, "errorCode");
    Objects.requireNonNull(nextAttemptAt, "nextAttemptAt");
    Objects.requireNonNull(now, "now");
    if (errorCode.isBlank()) {
      throw new IllegalArgumentException("errorCode must not be blank");
    }
    if (nextAttemptAt.isBefore(now)) {
      throw new IllegalArgumentException("nextAttemptAt must not be before now");
    }

    return jdbc
        .query(
            """
            UPDATE guest_conversion_operations
            SET attempt_count = attempt_count + 1,
                last_error_code = :errorCode,
                next_attempt_at = :nextAttemptAt,
                locked_until = NULL,
                updated_at = :now
            WHERE operation_id = :operationId
              AND state <> 'COMPLETED'
              AND locked_until = :expectedLockedUntil
              AND locked_until > :now
            RETURNING operation_id, account_id, otp_code_id, state, attempt_count,
                      next_attempt_at, locked_until, last_error_code, user_marked_at,
                      auth_promoted_at, event_published_at, created_at, updated_at
            """,
            new MapSqlParameterSource()
                .addValue("operationId", operationId)
                .addValue("expectedLockedUntil", Timestamp.from(expectedLockedUntil))
                .addValue("errorCode", errorCode)
                .addValue("nextAttemptAt", Timestamp.from(nextAttemptAt))
                .addValue("now", Timestamp.from(now)),
            ROW_MAPPER)
        .stream()
        .findFirst();
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
