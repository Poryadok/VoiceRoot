package voice.backend.auth.events;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

import io.nats.client.Connection;
import io.nats.client.JetStream;
import io.nats.client.JetStreamOptions;
import io.nats.client.api.PublishAck;
import io.nats.client.impl.Headers;
import java.time.Duration;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

class NatsAuthObservationPublisherTest {
  @Test
  void requiresPositiveUserStreamAckAndUsesOneSecondTimeoutAndStableDedupId() throws Exception {
    Connection connection = mock(Connection.class);
    JetStream jetStream = mock(JetStream.class);
    when(connection.jetStream(any(JetStreamOptions.class))).thenReturn(jetStream);
    PublishAck ack = mock(PublishAck.class);
    when(ack.getStream()).thenReturn("wrong_stream", "user_events", "user_events");
    when(ack.getSeqno()).thenReturn(0L, 12L);
    byte[] bytes = {1, 2};
    when(jetStream.publish(eq("user.logged_in"), any(Headers.class), same(bytes))).thenReturn(ack);
    var publisher = new NatsAuthEventPublisher(connection);
    assertThat(publisher.publishObservation("user.logged_in", bytes, "stable-id")).isFalse();
    assertThat(publisher.publishObservation("user.logged_in", bytes, "stable-id")).isFalse();
    assertThat(publisher.publishObservation("user.logged_in", bytes, "stable-id")).isTrue();
    ArgumentCaptor<JetStreamOptions> options = ArgumentCaptor.forClass(JetStreamOptions.class);
    verify(connection, times(3)).jetStream(options.capture());
    assertThat(options.getValue().getRequestTimeout()).isEqualTo(Duration.ofSeconds(1));
    ArgumentCaptor<Headers> headers = ArgumentCaptor.forClass(Headers.class);
    verify(jetStream, times(3)).publish(eq("user.logged_in"), headers.capture(), same(bytes));
    assertThat(headers.getAllValues()).allSatisfy(value ->
        assertThat(value.getFirst("Nats-Msg-Id")).isEqualTo("stable-id"));
  }
}
