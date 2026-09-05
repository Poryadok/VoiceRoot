package voice.backend.auth.service;

import voice.events.v1.JetstreamEvents.UserStreamEvent;

/**
 * Explicitly fails durable event delivery while JetStream is unconfigured, so the worker fences
 * and retries the operation rather than silently completing or abandoning it.
 */
public final class UnavailableGuestConversionEventPublisher implements GuestConversionEventPublisher {
  @Override
  public GuestConversionPublishAck publishGuestConverted(
      String subject, UserStreamEvent envelope, String natsMessageId) {
    throw new IllegalStateException("JetStream guest conversion publisher is not configured");
  }
}
