package voice.backend.auth.repository;

import java.time.Instant;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

/** Test-only mirror of the durable operation identity and monotonic state machine. */
public final class InMemoryAccountDeletionOperationRepository
    implements AccountDeletionOperationRepository {
  private final Map<String, AccountDeletionOperation> operations = new HashMap<>();
  private final Map<UUID, String> keysByOperation = new HashMap<>();

  @Override
  public synchronized AccountDeletionOperation createOrResume(
      UUID operationId, UUID accountId, long sessionEpoch, String restoreTokenHash, Instant now) {
    String key = key(accountId, sessionEpoch);
    AccountDeletionOperation existing = operations.get(key);
    if (existing != null) {
      return existing;
    }
    AccountDeletionOperation created =
        new AccountDeletionOperation(
            operationId,
            accountId,
            sessionEpoch,
            restoreTokenHash,
            AccountDeletionState.PENDING_FLOOR,
            null,
            null,
            now,
            now);
    operations.put(key, created);
    keysByOperation.put(created.operationId(), key);
    return created;
  }

  @Override
  public synchronized Optional<AccountDeletionOperation> findByAccountAndEpoch(
      UUID accountId, long sessionEpoch) {
    return Optional.ofNullable(operations.get(key(accountId, sessionEpoch)));
  }

  @Override
  public synchronized AccountDeletionOperation markFloorRecorded(UUID operationId, Instant now) {
    AccountDeletionOperation current = require(operationId);
    if (current.state() != AccountDeletionState.PENDING_FLOOR) {
      return current;
    }
    AccountDeletionOperation advanced =
        new AccountDeletionOperation(
            current.operationId(), current.accountId(), current.sessionEpoch(), current.restoreTokenHash(),
            AccountDeletionState.PENDING_EVENT, now, null, current.createdAt(), now);
    operations.put(keysByOperation.get(operationId), advanced);
    return advanced;
  }

  @Override
  public synchronized AccountDeletionOperation markEventPublished(UUID operationId, Instant now) {
    AccountDeletionOperation current = require(operationId);
    if (current.state() == AccountDeletionState.COMPLETED) {
      return current;
    }
    if (current.state() != AccountDeletionState.PENDING_EVENT) {
      throw new IllegalStateException("account deletion event cannot precede the epoch floor");
    }
    AccountDeletionOperation completed =
        new AccountDeletionOperation(
            current.operationId(), current.accountId(), current.sessionEpoch(), current.restoreTokenHash(),
            AccountDeletionState.COMPLETED, current.floorRecordedAt(), now, current.createdAt(), now);
    operations.put(keysByOperation.get(operationId), completed);
    return completed;
  }

  private AccountDeletionOperation require(UUID operationId) {
    String key = keysByOperation.get(operationId);
    if (key == null) {
      throw new IllegalArgumentException("unknown account deletion operation");
    }
    return operations.get(key);
  }

  private static String key(UUID accountId, long epoch) {
    return accountId + ":" + epoch;
  }
}
