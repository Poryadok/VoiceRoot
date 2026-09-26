package voice.backend.auth.sdkidentity;

import java.util.Collections;
import java.util.HashSet;
import java.util.Set;
import java.util.UUID;

/** Trusted registry lookup; request data cannot supply an admission policy. */
@FunctionalInterface
public interface SdkAuthorizationPolicy {
  Policy resolve(UUID applicationId, UUID environmentId);

  record Policy(UUID applicationId, UUID environmentId, long revision, String displayName,
                Set<String> redirectUris, Set<String> playerScopes) {
    public Policy {
      // Keep malformed values representable so the protocol validator denies them uniformly.
      redirectUris = immutable(redirectUris);
      playerScopes = immutable(playerScopes);
    }

    private static Set<String> immutable(Set<String> values) {
      return values == null ? null : Collections.unmodifiableSet(new HashSet<>(values));
    }
  }
}
