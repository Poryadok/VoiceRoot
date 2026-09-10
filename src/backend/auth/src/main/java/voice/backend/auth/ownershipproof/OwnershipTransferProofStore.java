package voice.backend.auth.ownershipproof;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import java.util.function.Function;

/** The entire callback runs atomically with account security mutations serialized against it. */
public interface OwnershipTransferProofStore {
  /** Read-only committed receipt lookup; independent of current account/security state. */
  Optional<StoredProof> findConsumed(UUID operationId);
  <T> T withAccount(UUID accountId, Function<Session, T> action);
  interface Session {
    /** Reloads state so a backup-code factor consumed in this transaction is reflected. */
    ProofAccount account();
    Optional<StoredProof> find(UUID operationId);
    void insert(StoredProof proof);
    void markConsumed(UUID operationId, Instant at);
  }
}
