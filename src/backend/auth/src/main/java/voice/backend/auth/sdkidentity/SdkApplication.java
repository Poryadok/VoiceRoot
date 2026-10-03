package voice.backend.auth.sdkidentity;

import com.nimbusds.jose.jwk.RSAKey;
import java.util.UUID;

/** Trusted operator admission, never populated from an enrollment request. */
public record SdkApplication(UUID applicationId, UUID environmentId, String clientId, RSAKey gameKey) {
  public SdkApplication {
    if (applicationId == null || environmentId == null || clientId == null || clientId.isBlank()
        || clientId.length() > 255 || gameKey == null || gameKey.isPrivate() || gameKey.size() < 2048) {
      throw new IllegalArgumentException("invalid sdk application admission");
    }
  }
}
