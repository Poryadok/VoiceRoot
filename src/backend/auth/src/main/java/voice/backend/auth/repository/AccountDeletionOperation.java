package voice.backend.auth.repository;

import java.time.Instant;
import java.util.UUID;

/** Auth-owned outbox record for one deletion generation. Restore-token plaintext is never stored. */
public record AccountDeletionOperation(
    UUID operationId,
    UUID accountId,
    long sessionEpoch,
    String restoreTokenHash,
    AccountDeletionState state,
    int attemptCount,
    Instant nextAttemptAt,
    Instant lockedUntil,
    String lastErrorCode,
    Instant floorRecordedAt,
    Instant eventPublishedAt,
    Instant createdAt,
    Instant updatedAt) {}
