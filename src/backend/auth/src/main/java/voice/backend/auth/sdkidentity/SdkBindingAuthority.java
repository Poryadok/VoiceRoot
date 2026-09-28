package voice.backend.auth.sdkidentity;

import java.util.Optional;
import java.util.UUID;

/** T16-owned source of the current player binding; absence must deny assertion issuance. */
@FunctionalInterface
public interface SdkBindingAuthority {
  Optional<Binding> currentBinding(UUID applicationId, UUID environmentId, UUID accountId);

  record Binding(UUID actorId, UUID bindingId) {}
}
