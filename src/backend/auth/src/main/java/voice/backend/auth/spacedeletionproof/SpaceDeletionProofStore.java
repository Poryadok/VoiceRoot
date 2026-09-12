package voice.backend.auth.spacedeletionproof;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import java.util.function.Function;
import voice.backend.auth.ownershipproof.ProofAccount;

/** Atomic deletion-proof persistence with account security mutations serialized against it. */
public interface SpaceDeletionProofStore {
  Optional<StoredDeletionProof> findConsumed(UUID operationId);

  default Optional<StoredDeletionProof> findConsumed(DeletionProofLookup lookup) {
    return findConsumed(lookup.operationId());
  }

  Instant acknowledge(DeletionProofAcknowledgement acknowledgement, Instant at);
  int deleteRetainedReceipts(Instant at);
  int pseudonymizeAccount(UUID accountId);
  <T> T withAccount(UUID accountId, Function<Session, T> action);

  interface Session {
    ProofAccount account();
    Optional<StoredDeletionProof> find(UUID operationId);
    void insert(StoredDeletionProof proof);
    void save(StoredDeletionProof proof);
  }
}
