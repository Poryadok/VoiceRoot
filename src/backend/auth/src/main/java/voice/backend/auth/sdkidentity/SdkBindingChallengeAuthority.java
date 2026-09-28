package voice.backend.auth.sdkidentity;

import java.time.Instant;
import java.util.UUID;

/** Authenticated GIS-owned challenge facts used before Auth consumes consent. */
public interface SdkBindingChallengeAuthority {
  record Challenge(UUID challengeId, String nonce, UUID applicationId, UUID environmentId, String provider,
      String redirectUriSha256, String pkceChallenge, UUID deviceKeyId, String deviceKeyThumbprint,
      UUID operationId, Instant expiresAt, String status, UUID sourceAccountId, UUID sourceActorId,
      UUID sourceDeviceId, long sourceGeneration, UUID targetAccountId, UUID targetProfileId,
      long profileRevision, long consentRevision, long policyRevision, java.util.List<String> scopes) {
    public Challenge(UUID challengeId, String nonce, UUID applicationId, UUID environmentId, String provider,
        String redirectUriSha256, String pkceChallenge, UUID deviceKeyId, String deviceKeyThumbprint,
        UUID operationId, Instant expiresAt, String status) {
      this(challengeId, nonce, applicationId, environmentId, provider, redirectUriSha256, pkceChallenge,
          deviceKeyId, deviceKeyThumbprint, operationId, expiresAt, status, null, null, null, 0,
          null, null, 0, 0, 0, java.util.List.of());
    }
  }

  record CreateRequest(UUID applicationId, UUID environmentId, String provider, String redirectUriSha256,
      String pkceChallenge, UUID deviceKeyId, String deviceKeyThumbprint, UUID operationId, Instant expiresAt,
      UUID sourceAccountId, UUID sourceActorId, UUID sourceDeviceId, long sourceGeneration,
      UUID targetAccountId, UUID targetProfileId, long profileRevision, long consentRevision,
      long policyRevision, java.util.List<String> scopes) {}

  Challenge resolveBindingChallenge(UUID challengeId);

  default Challenge createBindingChallenge(CreateRequest request) { throw new SdkIdentityDeniedException(); }
}
