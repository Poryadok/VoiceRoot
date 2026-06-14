package voice.backend.auth.userdb;

import java.util.UUID;

public interface ProfileSwitchValidator {
  void validateOwnedSwitchable(UUID accountId, UUID profileId);
}
