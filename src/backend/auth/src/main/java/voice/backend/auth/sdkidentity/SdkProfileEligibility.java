package voice.backend.auth.sdkidentity;

import java.util.UUID;

/** Read-only User authority. Missing or unverifiable answers must fail closed. */
public interface SdkProfileEligibility {
  Profile inspect(UUID accountId, UUID profileId);

  /** Short-deadline variant used while serializing message-permit issue with revoke. */
  default Profile inspectForExecutionPermit(UUID accountId, UUID profileId) {
    return inspect(accountId, profileId);
  }

  record Profile(UUID accountId, UUID profileId, long revision, boolean deleted, boolean frozen) {}
}
