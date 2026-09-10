package voice.backend.auth.ownershipproof;

import java.time.Instant;
import java.util.List;
import java.util.UUID;

public record StoredProof(ProofBinding binding, String proofHash, long securityRevision,
    List<String> factors, Instant expiresAt, UUID receiptId, Instant consumedAt) {
  public StoredProof { factors = List.copyOf(factors); }
  public StoredProof consumed(Instant at) {
    return new StoredProof(binding, proofHash, securityRevision, factors, expiresAt, receiptId, at);
  }
}
