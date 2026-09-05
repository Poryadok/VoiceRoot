package voice.backend.auth.repository;

import java.time.Instant;
import java.util.UUID;

public record GuestConversionOperation(
    UUID operationId,
    UUID accountId,
    UUID otpCodeId,
    GuestConversionState state,
    int attemptCount,
    Instant nextAttemptAt,
    Instant lockedUntil,
    String lastErrorCode,
    Instant userMarkedAt,
    Instant authPromotedAt,
    Instant eventPublishedAt,
    Instant createdAt,
    Instant updatedAt) {}
