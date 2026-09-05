package voice.backend.auth.service;

import java.time.Instant;
import java.util.UUID;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountRepository;

/** Memory-profile equivalent; production correctness relies on the JDBC transaction implementation. */
public final class InMemoryAccountDeletionOperationStarter implements AccountDeletionOperationStarter {
  private final AccountRepository accounts;
  private final AccountDeletionOperationRepository operations;

  public InMemoryAccountDeletionOperationStarter(
      AccountRepository accounts, AccountDeletionOperationRepository operations) {
    this.accounts = accounts;
    this.operations = operations;
  }

  @Override
  public synchronized AccountDeletionStartResult startOrResume(
      Account account, UUID proposedOperationId, String tokenHash, Instant now) {
    try {
      long epoch = accounts.markDeletedAndIncrementSessionEpoch(account.id(), now);
      Account deleted = accounts.findById(account.id().toString()).orElseThrow();
      return new AccountDeletionStartResult(
          deleted, operations.createOrResume(proposedOperationId, account.id(), epoch, tokenHash, now));
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
