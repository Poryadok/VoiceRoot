package voice.backend.auth.service;

import java.time.Instant;
import java.util.UUID;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** Memory-profile equivalent; production correctness relies on the JDBC transaction implementation. */
public final class InMemoryAccountDeletionOperationStarter implements AccountDeletionOperationStarter {
  private final AccountRepository accounts;
  private final AccountDeletionOperationRepository operations;
  private final SessionEpochFloorStore floors;

  public InMemoryAccountDeletionOperationStarter(
      AccountRepository accounts,
      AccountDeletionOperationRepository operations,
      SessionEpochFloorStore floors) {
    this.accounts = accounts;
    this.operations = operations;
    this.floors = floors;
  }

  @Override
  public synchronized AccountDeletionStartResult startOrResume(
      Account account, UUID proposedOperationId, String tokenHash, Instant now) {
    try {
      long expectedEpoch = Math.addExact(account.sessionEpoch(), 1L);
      long floor = floors.recordAtLeast(account.id(), expectedEpoch);
      if (floor < expectedEpoch) {
        throw new IllegalStateException("session epoch floor did not reach durable epoch");
      }
      long epoch = accounts.markDeletedAndIncrementSessionEpoch(account.id(), now);
      Account deleted = accounts.findById(account.id().toString()).orElseThrow();
      AccountDeletionOperation operation =
          operations.createOrResume(proposedOperationId, account.id(), epoch, tokenHash, now);
      AccountDeletionOperation leased =
          operations
              .lease(
                  operation.operationId(),
                  voice.backend.auth.repository.AccountDeletionState.PENDING_FLOOR,
                  now,
                  now.plusSeconds(30))
              .orElseThrow(() -> new IllegalStateException("deletion operation lease missing"));
      operations.markFloorRecorded(operation.operationId(), leased.lockedUntil(), now);
      return new AccountDeletionStartResult(
          deleted,
          operations
              .findByAccountAndEpoch(account.id(), epoch)
              .orElseThrow(() -> new IllegalStateException("deletion operation missing")));
    } catch (IllegalArgumentException raced) {
      Account deleted = accounts.findById(account.id().toString()).orElseThrow(() -> raced);
      if (!"deleted".equals(deleted.status())) {
        throw raced;
      }
      return new AccountDeletionStartResult(
          deleted,
          operations
              .findByAccountAndEpoch(deleted.id(), deleted.sessionEpoch())
              .orElseThrow(() -> new IllegalStateException("deletion operation missing", raced)));
    }
  }
}
