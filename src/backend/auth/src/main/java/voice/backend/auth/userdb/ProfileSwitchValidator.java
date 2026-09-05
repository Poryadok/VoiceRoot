package voice.backend.auth.userdb;

import java.util.UUID;

public interface ProfileSwitchValidator {
  void validateOwnedSwitchable(UUID accountId, UUID profileId);

  default void validateOwnedSwitchable(
      UUID accountId, UUID currentProfileId, UUID profileId, String subscriptionTier) {
    validateOwnedSwitchable(accountId, profileId);
  }
}
