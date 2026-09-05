package voice.backend.auth.service;

import java.time.Instant;
import java.util.UUID;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** Couples active→deleted+epoch and the outbox insert in one Auth database transaction. */
public final class TransactionalAccountDeletionOperationStarter implements AccountDeletionOperationStarter {
  private final TransactionTemplate transactions;
  private final AccountRepository accounts;
  private final AccountDeletionOperationRepository operations;
  private final SessionEpochFloorStore floors;

  public TransactionalAccountDeletionOperationStarter(
      TransactionTemplate transactions,
      AccountRepository accounts,
      AccountDeletionOperationRepository operations,
      SessionEpochFloorStore floors) {
    this.transactions = transactions;
    this.accounts = accounts;
    this.operations = operations;
    this.floors = floors;
  }

  @Override
  public AccountDeletionStartResult startOrResume(
      Account account, UUID proposedOperationId, String tokenHash, Instant now) {
    AccountDeletionStartResult result =
        transactions.execute(
            ignored -> {
              try {
                long epoch = accounts.markDeletedAndIncrementSessionEpoch(account.id(), now);
                Account deleted =
                    accounts.findById(account.id().toString()).orElseThrow();
                AccountDeletionOperation operation =
                    operations.createOrResume(proposedOperationId, account.id(), epoch, tokenHash, now);
                long floor = floors.recordAtLeast(account.id(), epoch);
                if (floor < epoch) {
                  throw new IllegalStateException("session epoch floor did not reach durable epoch");
                }
                AccountDeletionOperation leased =
                    operations
                        .lease(
                            operation.operationId(),
                            voice.backend.auth.repository.AccountDeletionState.PENDING_FLOOR,
                            now,
                            now.plusSeconds(30))
                        .orElseThrow(() -> new IllegalStateException("deletion operation lease missing"));
                operations.markFloorRecorded(operation.operationId(), leased.lockedUntil(), now);
                AccountDeletionOperation advanced =
                    operations.findByAccountAndEpoch(account.id(), epoch).orElseThrow();
                return new AccountDeletionStartResult(deleted, advanced);
              } catch (IllegalArgumentException raced) {
                Account deleted =
                    accounts.findById(account.id().toString()).orElseThrow(() -> raced);
                if (!"deleted".equals(deleted.status())) {
                  throw raced;
                }
                AccountDeletionOperation operation =
                    operations
                        .findByAccountAndEpoch(deleted.id(), deleted.sessionEpoch())
                        .orElseThrow(() -> new IllegalStateException("deletion operation missing", raced));
                return new AccountDeletionStartResult(deleted, operation);
              }
            });
    if (result == null) {
      throw new IllegalStateException("account deletion transaction returned no result");
    }
    return result;
  }
}
