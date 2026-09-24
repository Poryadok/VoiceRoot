package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.JetStream;
import io.nats.client.JetStreamManagement;
import io.nats.client.JetStreamSubscription;
import io.nats.client.Message;
import io.nats.client.MessageHandler;
import io.nats.client.PushSubscribeOptions;
import io.nats.client.api.AckPolicy;
import io.nats.client.api.ConsumerConfiguration;
import io.nats.client.api.ConsumerInfo;
import io.nats.client.api.DeliverPolicy;
import java.io.IOException;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import voice.events.v1.JetstreamEvents;

class NatsSubscriptionTierStoreTest {
  private static final String STREAM = "subscription_events";
  private static final String DURABLE = "auth_subscription_tier";
  private static final String DELIVER = "_INBOX.voice.auth.subscription_tier";

  @Test
  void bindsOnlyThePreprovisionedConsumer() throws Exception {
    Fixture fixture = new Fixture();
    fixture.consumer(config("subscription.>", DELIVER, AckPolicy.Explicit, DeliverPolicy.New));

    try (NatsSubscriptionTierStore ignored = new NatsSubscriptionTierStore(fixture.connection)) {
      ArgumentCaptor<PushSubscribeOptions> options = ArgumentCaptor.forClass(PushSubscribeOptions.class);
      verify(fixture.jetStream)
          .subscribe(eq("subscription.>"), eq(fixture.dispatcher), any(MessageHandler.class),
              eq(false), options.capture());
      assertThat(options.getValue().isBind()).isTrue();
      assertThat(options.getValue().getStream()).isEqualTo(STREAM);
      assertThat(options.getValue().getDurable()).isEqualTo(DURABLE);
      assertThat(options.getValue().getDeliverSubject()).isEqualTo(DELIVER);
      verify(fixture.management, never()).addStream(any());
      verify(fixture.management, never()).createConsumer(any(), any());
      verify(fixture.management, never()).addOrUpdateConsumer(any(), any());
    }
  }

  @Test
  void missingConsumerFailsBeforeSubscribeAndClosesConnection() throws Exception {
    Fixture fixture = new Fixture();
    when(fixture.management.getConsumerInfo(STREAM, DURABLE)).thenThrow(new IOException("missing"));

    assertThatThrownBy(() -> new NatsSubscriptionTierStore(fixture.connection))
        .isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("connect subscription.events consumer");
    verify(fixture.jetStream, never())
        .subscribe(any(), any(Dispatcher.class), any(MessageHandler.class), eq(false), any());
    verify(fixture.connection).close();
  }

  @Test
  void rejectsMismatchedConsumerShape() throws Exception {
    ConsumerConfiguration valid = config("subscription.>", DELIVER, AckPolicy.Explicit, DeliverPolicy.New);
    assertRejected("wrong_stream", valid, "stream");
    assertRejected(STREAM, config("user.>", DELIVER, AckPolicy.Explicit, DeliverPolicy.New), "filter");
    assertRejected(STREAM, config("subscription.>", "_INBOX.other", AckPolicy.Explicit, DeliverPolicy.New), "deliver");
    assertRejected(STREAM, config("subscription.>", DELIVER, AckPolicy.None, DeliverPolicy.New), "ack");
    assertRejected(STREAM, config("subscription.>", DELIVER, AckPolicy.Explicit, DeliverPolicy.All), "policy");
    assertRejected(STREAM, ConsumerConfiguration.builder(valid).deliverGroup("unexpected").build(), "group");
    assertRejected(STREAM, ConsumerConfiguration.builder(valid).durable("other").build(), "durable");
  }

  @Test
  void acknowledgesHandledAndIgnoredEventsButTerminatesMalformedPayloads() throws Exception {
    Fixture fixture = new Fixture();
    fixture.consumer(config("subscription.>", DELIVER, AckPolicy.Explicit, DeliverPolicy.New));

    try (NatsSubscriptionTierStore store = new NatsSubscriptionTierStore(fixture.connection)) {
      Message handled = mock(Message.class);
      when(handled.getData())
          .thenReturn(
              JetstreamEvents.SubscriptionStreamEvent.newBuilder()
                  .setPlanStarted(
                      JetstreamEvents.PlanStarted.newBuilder()
                          .setAccountId("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
                          .setPlan("premium"))
                  .build()
                  .toByteArray());
      store.onMessage(handled);
      verify(handled).ack();
      assertThat(
              store.resolveTier(
                  java.util.UUID.fromString("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")))
          .isEqualTo("premium");

      Message ignored = mock(Message.class);
      when(ignored.getData())
          .thenReturn(
              JetstreamEvents.SubscriptionStreamEvent.newBuilder()
                  .setPaymentFailed(JetstreamEvents.PaymentFailed.newBuilder())
                  .build()
                  .toByteArray());
      store.onMessage(ignored);
      verify(ignored).ack();

      Message malformed = mock(Message.class);
      when(malformed.getData()).thenReturn(new byte[] {0x0f});
      store.onMessage(malformed);
      verify(malformed).term();
      verify(malformed, never()).ack();
    }
  }

  private static void assertRejected(String stream, ConsumerConfiguration configuration, String reason)
      throws Exception {
    Fixture fixture = new Fixture();
    fixture.consumer(stream, configuration);
    assertThatThrownBy(() -> new NatsSubscriptionTierStore(fixture.connection))
        .isInstanceOf(IllegalStateException.class)
        .hasStackTraceContaining(reason);
    verify(fixture.jetStream, never())
        .subscribe(any(), any(Dispatcher.class), any(MessageHandler.class), eq(false), any());
    verify(fixture.connection).close();
  }

  private static ConsumerConfiguration config(
      String filter, String deliver, AckPolicy ack, DeliverPolicy policy) {
    return ConsumerConfiguration.builder()
        .durable(DURABLE)
        .filterSubject(filter)
        .deliverSubject(deliver)
        .ackPolicy(ack)
        .deliverPolicy(policy)
        .build();
  }

  private static final class Fixture {
    final Connection connection = mock(Connection.class);
    final JetStreamManagement management = mock(JetStreamManagement.class);
    final JetStream jetStream = mock(JetStream.class);
    final Dispatcher dispatcher = mock(Dispatcher.class);

    Fixture() throws Exception {
      when(connection.jetStreamManagement()).thenReturn(management);
      when(connection.jetStream()).thenReturn(jetStream);
      when(connection.createDispatcher()).thenReturn(dispatcher);
      when(jetStream.subscribe(any(), any(Dispatcher.class), any(MessageHandler.class), eq(false), any()))
          .thenReturn(mock(JetStreamSubscription.class));
    }

    void consumer(ConsumerConfiguration configuration) throws Exception {
      consumer(STREAM, configuration);
    }

    void consumer(String stream, ConsumerConfiguration configuration) throws Exception {
      ConsumerInfo info = mock(ConsumerInfo.class);
      when(info.getStreamName()).thenReturn(stream);
      when(info.getName()).thenReturn(DURABLE);
      when(info.getConsumerConfiguration()).thenReturn(configuration);
      when(management.getConsumerInfo(STREAM, DURABLE)).thenReturn(info);
    }
  }
}
