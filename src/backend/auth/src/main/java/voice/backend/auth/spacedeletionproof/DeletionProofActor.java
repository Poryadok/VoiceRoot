package voice.backend.auth.spacedeletionproof;

import java.util.UUID;

/** Account/profile/session epoch established by an ordinary user access credential. */
public record DeletionProofActor(UUID accountId, UUID profileId, long sessionEpoch) {
  public DeletionProofActor {
    if (accountId == null || profileId == null || sessionEpoch <= 0) {
      throw new IllegalArgumentException("invalid deletion proof actor");
    }
  }
}
