package voice.backend.auth.repository;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

/** Durable, idempotent state for completing a soft deletion after the account row changed. */
public interface AccountDeletionOperationRepository {
  AccountDeletionOperation createOrResume(
      UUID operationId, UUID accountId, long sessionEpoch, String restoreTokenHash, Instant now);

  Optional<AccountDeletionOperation> findByAccountAndEpoch(UUID accountId, long sessionEpoch);

  AccountDeletionOperation markFloorRecorded(UUID operationId, Instant now);

  AccountDeletionOperation markEventPublished(UUID operationId, Instant now);
}
