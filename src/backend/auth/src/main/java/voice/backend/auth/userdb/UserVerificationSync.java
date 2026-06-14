package voice.backend.auth.userdb;

import java.util.UUID;

public interface UserVerificationSync {
  void setPersonalVerification(UUID profileId, String badge);

  void clearVerification(UUID profileId);
}
