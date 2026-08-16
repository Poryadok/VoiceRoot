package voice.backend.auth.service;

import com.google.protobuf.InvalidProtocolBufferException;
import java.util.Locale;
import java.util.Optional;
import java.util.UUID;
import voice.events.v1.JetstreamEvents;

/** Parses subscription.events protobuf payloads for JWT tier sync. */
final class SubscriptionEventParser {
  private SubscriptionEventParser() {}

  record TierUpdate(UUID accountId, String tier) {}

  static Optional<TierUpdate> parseTierUpdate(byte[] data) {
    if (data == null || data.length == 0) {
      return Optional.empty();
    }
    try {
      JetstreamEvents.SubscriptionStreamEvent event =
          JetstreamEvents.SubscriptionStreamEvent.parseFrom(data);
      return switch (event.getPayloadCase()) {
        case PLAN_STARTED -> {
          JetstreamEvents.PlanStarted started = event.getPlanStarted();
          UUID accountId = UUID.fromString(started.getAccountId());
          yield Optional.of(new TierUpdate(accountId, tierFromPlan(started.getPlan())));
        }
        case PLAN_CANCELLED, PLAN_EXPIRED, DOWNGRADE -> {
          String accountId =
              switch (event.getPayloadCase()) {
                case PLAN_CANCELLED -> event.getPlanCancelled().getAccountId();
                case PLAN_EXPIRED -> event.getPlanExpired().getAccountId();
                case DOWNGRADE -> event.getDowngrade().getAccountId();
                default -> throw new IllegalStateException("unexpected payload: " + event.getPayloadCase());
              };
          yield Optional.of(new TierUpdate(UUID.fromString(accountId), "free"));
        }
        // payment_failed → grace_period: entitlements stay active (subscription.md)
        default -> Optional.empty();
      };
    } catch (InvalidProtocolBufferException | IllegalArgumentException ex) {
      return Optional.empty();
    }
  }

  private static String tierFromPlan(String plan) {
    if (plan == null) {
      return "free";
    }
    return switch (plan.trim().toLowerCase(Locale.ROOT)) {
      case "premium", "space_pro" -> "premium";
      default -> "free";
    };
  }
}
