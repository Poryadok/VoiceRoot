package voice.backend.auth.userdb;

import java.util.UUID;

public class NoOpUserVerificationSync implements UserVerificationSync {
  @Override
  public void setPersonalVerification(UUID profileId, String badge) {}

  @Override
  public void clearVerification(UUID profileId) {}
}
