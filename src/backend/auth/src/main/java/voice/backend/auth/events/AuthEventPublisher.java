package voice.backend.auth.events;

import java.util.UUID;

/** Publishes Auth domain events to NATS (user.events stream). */
public interface AuthEventPublisher {
  String SUBJECT_GUEST_CONVERTED = "user.guest_converted";
  String SUBJECT_ACCOUNT_DELETED = "user.account_deleted";
  String SUBJECT_ACCOUNT_RESTORED = "user.account_restored";

  void publishGuestConverted(UUID accountId);

  void publishAccountDeleted(UUID accountId);

  void publishAccountRestored(UUID accountId);
}
