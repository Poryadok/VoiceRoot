package voice.backend.auth.userdb;

import java.util.UUID;

/** Allows any profile switch; used in unit tests with in-memory provisioner. */
public class NoOpProfileSwitchValidator implements ProfileSwitchValidator {
  @Override
  public void validateOwnedSwitchable(UUID accountId, UUID profileId) {}
}
