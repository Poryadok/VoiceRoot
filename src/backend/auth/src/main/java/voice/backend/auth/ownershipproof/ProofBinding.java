package voice.backend.auth.ownershipproof;

import java.util.UUID;

public record ProofBinding(UUID accountId, UUID profileId, UUID spaceId,
    UUID newOwnerProfileId, UUID operationId, long sessionEpoch) {
  public ProofBinding {
    if (accountId == null || profileId == null || spaceId == null || newOwnerProfileId == null
        || operationId == null || sessionEpoch <= 0) {
      throw new IllegalArgumentException("invalid proof binding");
    }
  }
}
