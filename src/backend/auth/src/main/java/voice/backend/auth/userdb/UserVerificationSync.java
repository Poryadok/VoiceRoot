package voice.backend.auth.userdb;

import java.util.UUID;

public interface UserVerificationSync {
  void setPersonalVerification(UUID profileId, String badge);

  void clearVerification(UUID profileId);

  default void applySourceState(
      UUID profileId, String source, long revision, boolean verified, String badge) {
    if (verified) {
      setPersonalVerification(profileId, badge);
    } else {
      clearVerification(profileId);
    }
  }
}
