package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.events.v1.JetstreamEvents;

class SubscriptionEventParserTest {
  @Test
  void parsePlanStartedPremium() {
    JetstreamEvents.SubscriptionStreamEvent event =
        JetstreamEvents.SubscriptionStreamEvent.newBuilder()
            .setPlanStarted(
                JetstreamEvents.PlanStarted.newBuilder()
                    .setAccountId("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
                    .setPlan("premium")
                    .build())
            .build();

    var update = SubscriptionEventParser.parseTierUpdate(event.toByteArray());
    assertThat(update).isPresent();
    assertThat(update.get().accountId())
        .isEqualTo(UUID.fromString("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"));
    assertThat(update.get().tier()).isEqualTo("premium");
  }

  @Test
  void parsePlanCancelledResetsFree() {
    JetstreamEvents.SubscriptionStreamEvent event =
        JetstreamEvents.SubscriptionStreamEvent.newBuilder()
            .setPlanCancelled(
                JetstreamEvents.PlanCancelled.newBuilder()
                    .setAccountId("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
                    .setPlan("premium")
                    .build())
            .build();

    var update = SubscriptionEventParser.parseTierUpdate(event.toByteArray());
    assertThat(update).isPresent();
    assertThat(update.get().tier()).isEqualTo("free");
  }

  @Test
  void parsePlanExpiredResetsFree() {
    JetstreamEvents.SubscriptionStreamEvent event =
        JetstreamEvents.SubscriptionStreamEvent.newBuilder()
            .setPlanExpired(
                JetstreamEvents.PlanExpired.newBuilder()
                    .setAccountId("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
                    .setPlan("premium")
                    .build())
            .build();

    var update = SubscriptionEventParser.parseTierUpdate(event.toByteArray());
    assertThat(update).isPresent();
    assertThat(update.get().accountId())
        .isEqualTo(UUID.fromString("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"));
    assertThat(update.get().tier()).isEqualTo("free");
  }

  @Test
  void parseDowngradeResetsFree() {
    JetstreamEvents.SubscriptionStreamEvent event =
        JetstreamEvents.SubscriptionStreamEvent.newBuilder()
            .setDowngrade(
                JetstreamEvents.Downgrade.newBuilder()
                    .setAccountId("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
                    .setPlan("premium")
                    .build())
            .build();

    var update = SubscriptionEventParser.parseTierUpdate(event.toByteArray());
    assertThat(update).isPresent();
    assertThat(update.get().accountId())
        .isEqualTo(UUID.fromString("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"));
    assertThat(update.get().tier()).isEqualTo("free");
  }
}
