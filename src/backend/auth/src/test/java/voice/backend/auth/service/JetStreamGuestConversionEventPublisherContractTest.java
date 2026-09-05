package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.nats.client.JetStream;
import io.nats.client.api.PublishAck;
import io.nats.client.impl.Headers;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import voice.events.v1.JetstreamEvents.UserGuestConverted;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

class JetStreamGuestConversionEventPublisherContractTest {
  @Test
  void realJetStreamAdapterSerializesCanonicalEnvelopeAndReturnsTheDurablePubAck() throws Exception {
    JetStream jetStream = mock(JetStream.class);
    PublishAck pubAck = mock(PublishAck.class);
    when(pubAck.getStream()).thenReturn("user.events");
    when(pubAck.getSeqno()).thenReturn(42L);
    when(jetStream.publish(eq("user.guest_converted"), any(Headers.class), any(byte[].class)))
        .thenReturn(pubAck);
    UUID operationId = UUID.fromString("00000000-0000-0000-0000-000000000041");
    UUID accountId = UUID.fromString("00000000-0000-0000-0000-000000000042");
    UserStreamEvent envelope =
        UserStreamEvent.newBuilder()
            .setEventId(operationId.toString())
            .setUserGuestConverted(
                UserGuestConverted.newBuilder().setAccountId(accountId.toString()).build())
            .build();
    ArgumentCaptor<Headers> headers = ArgumentCaptor.forClass(Headers.class);
    ArgumentCaptor<byte[]> payload = ArgumentCaptor.forClass(byte[].class);

    GuestConversionPublishAck acknowledged =
        new JetStreamGuestConversionEventPublisher(jetStream)
            .publishGuestConverted("user.guest_converted", envelope, operationId.toString());

    verify(jetStream).publish(eq("user.guest_converted"), headers.capture(), payload.capture());
    assertThat(headers.getValue().getFirst("Nats-Msg-Id")).isEqualTo(operationId.toString());
    UserStreamEvent decoded = UserStreamEvent.parseFrom(payload.getValue());
    assertThat(decoded.getEventId()).isEqualTo(operationId.toString());
    assertThat(decoded.getUserGuestConverted().getAccountId()).isEqualTo(accountId.toString());
    assertThat(acknowledged.stream()).isEqualTo("user.events");
    assertThat(acknowledged.sequence()).isEqualTo(42L);
  }
}
