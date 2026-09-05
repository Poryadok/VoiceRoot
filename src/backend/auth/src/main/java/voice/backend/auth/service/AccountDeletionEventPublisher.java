package voice.backend.auth.service;

import voice.events.v1.JetstreamEvents.UserStreamEvent;

/** Publishes the account-deletion outbox envelope and waits for JetStream acknowledgement. */
public interface AccountDeletionEventPublisher {
  GuestConversionPublishAck publishAccountDeleted(
      String subject, UserStreamEvent envelope, String natsMessageId);
}
