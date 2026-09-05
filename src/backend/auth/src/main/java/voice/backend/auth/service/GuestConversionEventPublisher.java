package voice.backend.auth.service;

import voice.events.v1.JetstreamEvents.UserStreamEvent;

/** Publishes a canonical guest-conversion envelope and returns its durable acknowledgement. */
public interface GuestConversionEventPublisher {
  GuestConversionPublishAck publishGuestConverted(
      String subject, UserStreamEvent envelope, String natsMessageId);
}
