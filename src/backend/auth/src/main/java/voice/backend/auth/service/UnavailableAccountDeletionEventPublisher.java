package voice.backend.auth.service;

import voice.events.v1.JetstreamEvents.UserStreamEvent;

/** Fails closed: a deletion outbox cannot be finalized without a durable event publisher. */
public final class UnavailableAccountDeletionEventPublisher implements AccountDeletionEventPublisher {
  @Override
  public GuestConversionPublishAck publishAccountDeleted(
      String subject, UserStreamEvent envelope, String natsMessageId) {
    throw new IllegalStateException("JetStream account deletion publisher is not configured");
  }
}
