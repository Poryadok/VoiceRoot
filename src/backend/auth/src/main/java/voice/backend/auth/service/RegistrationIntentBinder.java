package voice.backend.auth.service;

import java.util.UUID;

/** Binds a one-use registration intent to the new account inside the caller's Auth transaction. */
@FunctionalInterface
public interface RegistrationIntentBinder {
  void bind(UUID registrationIntentId, UUID accountId);
}
