package voice.backend.auth.sessionepoch;

import java.util.Map;
import java.util.UUID;

/** Reads the Auth database's durable account epochs for idempotent floor seeding. */
public interface DurableAccountEpochSource {
  Map<UUID, Long> currentAccountEpochs();
}
