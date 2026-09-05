package voice.backend.auth.repository;

import java.time.Instant;
import java.util.Optional;
import java.util.List;
import java.util.UUID;

/** Durable, idempotent state for completing a soft deletion after the account row changed. */
public interface AccountDeletionOperationRepository {
  AccountDeletionOperation createOrResume(
      UUID operationId, UUID accountId, long sessionEpoch, String restoreTokenHash, Instant now);

  Optional<AccountDeletionOperation> findByAccountAndEpoch(UUID accountId, long sessionEpoch);

  List<AccountDeletionOperation> leaseDue(
      AccountDeletionState state, int batchSize, Instant now, Instant leaseUntil);

  Optional<AccountDeletionOperation> lease(
      UUID operationId, AccountDeletionState state, Instant now, Instant leaseUntil);

  AccountDeletionAdvanceResult markFloorRecorded(
      UUID operationId, Instant expectedLockedUntil, Instant now);

  AccountDeletionAdvanceResult markEventPublished(
      UUID operationId, Instant expectedLockedUntil, Instant now);

  Optional<AccountDeletionOperation> recordFailure(
      UUID operationId, Instant expectedLockedUntil, String errorCode, Instant nextAttemptAt, Instant now);
}
