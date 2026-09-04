package voice.backend.auth.service;

import io.nats.client.JetStream;
import io.nats.client.api.PublishAck;
import io.nats.client.impl.Headers;
import java.util.Objects;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

/** JetStream adapter which uses the durable operation identity as the de-duplication key. */
public final class JetStreamGuestConversionEventPublisher implements GuestConversionEventPublisher {
  private final JetStream jetStream;

  public JetStreamGuestConversionEventPublisher(JetStream jetStream) {
    this.jetStream = Objects.requireNonNull(jetStream, "jetStream");
  }

  @Override
  public GuestConversionPublishAck publishGuestConverted(
      String subject, UserStreamEvent envelope, String natsMessageId) {
    Objects.requireNonNull(subject, "subject");
    Objects.requireNonNull(envelope, "envelope");
    Objects.requireNonNull(natsMessageId, "natsMessageId");
    if (natsMessageId.isBlank()) {
      throw new IllegalArgumentException("natsMessageId must not be blank");
    }
    try {
      Headers headers = new Headers();
      headers.put("Nats-Msg-Id", natsMessageId);
      PublishAck ack = jetStream.publish(subject, headers, envelope.toByteArray());
      if (ack == null) {
        throw new IllegalStateException("JetStream did not return a PubAck");
      }
      return new GuestConversionPublishAck(ack.getStream(), ack.getSeqno());
    } catch (Exception failure) {
      throw new IllegalStateException("publish durable guest conversion event", failure);
    }
  }
}
