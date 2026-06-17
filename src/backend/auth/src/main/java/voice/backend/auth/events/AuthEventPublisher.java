package voice.backend.auth.events;

import java.util.UUID;

/** Publishes Auth domain events to NATS (user.events stream). */
public interface AuthEventPublisher {
  String SUBJECT_GUEST_CONVERTED = "user.guest_converted";

  void publishGuestConverted(UUID accountId);
}
