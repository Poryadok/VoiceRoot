package voice.backend.auth.sdkidentity;

import java.time.Instant;
import java.util.UUID;

/** Auth's bounded, request-bound private GIS execution-permit calls. */
public interface SdkGameIntegrationExecutionPermitAuthority {
  record Permit(UUID permitId, UUID bindingId, UUID applicationId, UUID environmentId, long bindingRevision,
                UUID assertionJti, UUID operationId, Instant expiresAt) {}
  record Completion(UUID permitId, UUID operationId, String outcome, String status) {}

  Permit issue(UUID bindingId, UUID operationId, String deviceAuthorityAssertion);
  Completion complete(UUID permitId, UUID operationId, String outcome);
}
