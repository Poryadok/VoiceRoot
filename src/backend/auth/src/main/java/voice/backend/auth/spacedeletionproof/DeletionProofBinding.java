package voice.backend.auth.spacedeletionproof;

import java.util.UUID;

/** User and Space identifiers bound into one deletion authorization. */
public record DeletionProofBinding(
    UUID accountId,
    UUID profileId,
    long sessionEpoch,
    UUID spaceId,
    UUID operationId) {
  public DeletionProofBinding {
    if (accountId == null || profileId == null || sessionEpoch <= 0
        || spaceId == null || operationId == null) {
      throw new IllegalArgumentException("invalid deletion proof binding");
    }
  }
}
