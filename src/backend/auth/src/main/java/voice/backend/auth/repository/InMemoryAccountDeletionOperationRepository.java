package voice.backend.auth.repository;

import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

/** Test-profile mirror of the JDBC outbox lease/fencing semantics. */
public final class InMemoryAccountDeletionOperationRepository
    implements AccountDeletionOperationRepository {
  private final Map<String, AccountDeletionOperation> operations = new HashMap<>();
  private final Map<UUID, String> keysByOperation = new HashMap<>();

  @Override
  public synchronized AccountDeletionOperation createOrResume(
      UUID operationId, UUID accountId, long sessionEpoch, String restoreTokenHash, Instant now) {
    String key = key(accountId, sessionEpoch);
    AccountDeletionOperation existing = operations.get(key);
    if (existing != null) return existing;
    AccountDeletionOperation created =
        new AccountDeletionOperation(operationId, accountId, sessionEpoch, restoreTokenHash,
            AccountDeletionState.PENDING_FLOOR, 0, now, null, null, null, null, now, now);
    operations.put(key, created);
    keysByOperation.put(operationId, key);
    return created;
  }

  @Override
  public synchronized Optional<AccountDeletionOperation> findByAccountAndEpoch(UUID accountId, long epoch) {
    return Optional.ofNullable(operations.get(key(accountId, epoch)));
  }

  @Override
  public synchronized List<AccountDeletionOperation> leaseDue(
      AccountDeletionState state, int batchSize, Instant now, Instant leaseUntil) {
    if (batchSize <= 0 || !leaseUntil.isAfter(now)) throw new IllegalArgumentException("invalid lease");
    List<AccountDeletionOperation> due = operations.values().stream()
        .filter(op -> op.state() == state && !op.nextAttemptAt().isAfter(now))
        .filter(op -> op.lockedUntil() == null || !op.lockedUntil().isAfter(now))
        .sorted(Comparator.comparing(AccountDeletionOperation::nextAttemptAt)
            .thenComparing(AccountDeletionOperation::createdAt)).limit(batchSize).toList();
    List<AccountDeletionOperation> leased = new ArrayList<>();
    for (AccountDeletionOperation op : due) {
      AccountDeletionOperation copy = copy(op, op.state(), op.attemptCount(), op.nextAttemptAt(), leaseUntil,
          op.lastErrorCode(), op.floorRecordedAt(), op.eventPublishedAt(), now);
      operations.put(keysByOperation.get(op.operationId()), copy);
      leased.add(copy);
    }
    return List.copyOf(leased);
  }

  @Override
  public synchronized Optional<AccountDeletionOperation> lease(
      UUID operationId, AccountDeletionState state, Instant now, Instant leaseUntil) {
    AccountDeletionOperation operation = require(operationId);
    if (operation.state() != state || operation.nextAttemptAt().isAfter(now)
        || (operation.lockedUntil() != null && operation.lockedUntil().isAfter(now))) {
      return Optional.empty();
    }
    AccountDeletionOperation leased = copy(operation, operation.state(), operation.attemptCount(),
        operation.nextAttemptAt(), leaseUntil, operation.lastErrorCode(), operation.floorRecordedAt(),
        operation.eventPublishedAt(), now);
    operations.put(keysByOperation.get(operationId), leased);
    return Optional.of(leased);
  }

  @Override
  public synchronized AccountDeletionAdvanceResult markFloorRecorded(
      UUID operationId, Instant lease, Instant now) {
    return advance(operationId, AccountDeletionState.PENDING_FLOOR, lease, now);
  }

  @Override
  public synchronized AccountDeletionAdvanceResult markEventPublished(
      UUID operationId, Instant lease, Instant now) {
    return advance(operationId, AccountDeletionState.PENDING_EVENT, lease, now);
  }

  @Override
  public synchronized Optional<AccountDeletionOperation> recordFailure(
      UUID operationId, Instant lease, String code, Instant next, Instant now) {
    AccountDeletionOperation op = require(operationId);
    if (!lease.equals(op.lockedUntil()) || !lease.isAfter(now)
        || op.state() == AccountDeletionState.COMPLETED) return Optional.empty();
    AccountDeletionOperation copy = copy(op, op.state(), op.attemptCount() + 1, next, null, code,
        op.floorRecordedAt(), op.eventPublishedAt(), now);
    operations.put(keysByOperation.get(operationId), copy);
    return Optional.of(copy);
  }

  private AccountDeletionAdvanceResult advance(
      UUID operationId, AccountDeletionState expected, Instant lease, Instant now) {
    String key = keysByOperation.get(operationId);
    if (key == null) return AccountDeletionAdvanceResult.NOT_FOUND;
    AccountDeletionOperation op = operations.get(key);
    AccountDeletionState next = expected == AccountDeletionState.PENDING_FLOOR
        ? AccountDeletionState.PENDING_EVENT : AccountDeletionState.COMPLETED;
    if (op.state() == next || op.state() == AccountDeletionState.COMPLETED)
      return AccountDeletionAdvanceResult.ALREADY_APPLIED;
    if (op.state() != expected || !lease.equals(op.lockedUntil()) || !lease.isAfter(now))
      return AccountDeletionAdvanceResult.LEASE_LOST;
    AccountDeletionOperation copy = copy(op, next, op.attemptCount(), op.nextAttemptAt(), null,
        op.lastErrorCode(), expected == AccountDeletionState.PENDING_FLOOR ? now : op.floorRecordedAt(),
        expected == AccountDeletionState.PENDING_EVENT ? now : op.eventPublishedAt(), now);
    operations.put(key, copy);
    return AccountDeletionAdvanceResult.APPLIED;
  }

  private AccountDeletionOperation require(UUID id) {
    String key = keysByOperation.get(id);
    if (key == null) throw new IllegalArgumentException("unknown account deletion operation");
    return operations.get(key);
  }

  private static AccountDeletionOperation copy(AccountDeletionOperation op, AccountDeletionState state,
      int attempts, Instant next, Instant locked, String error, Instant floor, Instant event, Instant updated) {
    return new AccountDeletionOperation(op.operationId(), op.accountId(), op.sessionEpoch(), op.restoreTokenHash(),
        state, attempts, next, locked, error, floor, event, op.createdAt(), updated);
  }

  private static String key(UUID accountId, long epoch) { return accountId + ":" + epoch; }
}
