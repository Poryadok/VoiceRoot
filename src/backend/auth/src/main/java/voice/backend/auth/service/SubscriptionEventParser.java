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
      throw new IllegalArgumentException("empty subscription event payload");
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
        // These known events do not change personal Auth tier; do not treat new
        // or unsupported payload arms as successful no-ops.
        case PAYMENT_SUCCESS, PAYMENT_FAILED, SPACE_PRO_STARTED, SPACE_PRO_EXPIRED, GRACE_REMINDER ->
            Optional.empty();
        default -> throw new IllegalArgumentException(
            "unsupported subscription event payload: " + event.getPayloadCase());
      };
    } catch (InvalidProtocolBufferException | IllegalArgumentException ex) {
      throw new IllegalArgumentException("invalid subscription event payload", ex);
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
