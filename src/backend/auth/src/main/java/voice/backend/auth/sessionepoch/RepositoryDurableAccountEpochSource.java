package voice.backend.auth.sessionepoch;

import java.util.List;
import java.util.Objects;
import java.util.UUID;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.AccountSessionEpoch;

/** Adapts Auth's durable account repository for session epoch floor seeding. */
public final class RepositoryDurableAccountEpochSource implements DurableAccountEpochSource {
  private final AccountRepository accounts;

  public RepositoryDurableAccountEpochSource(AccountRepository accounts) {
    this.accounts = Objects.requireNonNull(accounts, "accounts");
  }

  @Override
  public List<AccountSessionEpoch> pageSessionEpochsAfter(UUID exclusiveAfter, int limit) {
    return accounts.pageSessionEpochsAfter(exclusiveAfter, limit);
  }

  @Override
  public long advanceSessionEpochAtLeast(UUID accountId, long requestedEpoch) {
    return accounts.advanceSessionEpochAtLeast(accountId, requestedEpoch);
  }
}
