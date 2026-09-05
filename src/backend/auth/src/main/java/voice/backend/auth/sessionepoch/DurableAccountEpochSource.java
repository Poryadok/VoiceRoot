package voice.backend.auth.sessionepoch;

import java.util.List;
import java.util.UUID;
import voice.backend.auth.repository.AccountSessionEpoch;

/** Reads the Auth database's durable account epochs for idempotent floor seeding. */
public interface DurableAccountEpochSource {
  List<AccountSessionEpoch> pageSessionEpochsAfter(UUID exclusiveAfter, int limit);

  long advanceSessionEpochAtLeast(UUID accountId, long requestedEpoch);
}
