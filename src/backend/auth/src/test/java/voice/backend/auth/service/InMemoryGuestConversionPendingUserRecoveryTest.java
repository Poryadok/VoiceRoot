package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.config.GuestConversionPendingUserRecoveryProperties;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryGuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

class InMemoryGuestConversionPendingUserRecoveryTest {
  private static final Instant NOW = Instant.parse("2026-09-04T10:15:30Z");
  private static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);

  @Test
  void explicitRunnerPromotesTheMemoryGuestAndLeavesOnlyPendingEventWork() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    var guest = accounts.create("memory-guest@example.com", null, "hash", "guest");
    InMemoryGuestConversionOperationRepository operations =
        new InMemoryGuestConversionOperationRepository();
    operations.createOrResume(guest.id(), UUID.randomUUID(), NOW);
    RecordingUser user = new RecordingUser();
    GuestConversionPendingUserRecoveryProperties properties =
        new GuestConversionPendingUserRecoveryProperties();
    properties.setBatchSize(1);
    properties.setLeaseDuration(Duration.ofMinutes(1));
    properties.setInterval(Duration.ofSeconds(10));

    new GuestConversionPendingUserRecoveryRunner(
            new GuestConversionPendingUserWorker(
                operations,
                user,
                new InMemoryGuestConversionLocalPromotion(accounts, operations),
                CLOCK),
            properties)
        .tick();

    assertThat(accounts.findById(guest.id().toString()).orElseThrow().type()).isEqualTo("regular");
    assertThat(user.accountIds).containsExactly(guest.id());
    List<GuestConversionOperation> nextDue =
        operations.leaseDue(1, NOW, NOW.plus(Duration.ofMinutes(2)));
    assertThat(nextDue).extracting(GuestConversionOperation::state).containsExactly(GuestConversionState.PENDING_EVENT);
  }

  private static final class RecordingUser implements PrimaryProfileProvisioner {
    private final java.util.ArrayList<UUID> accountIds = new java.util.ArrayList<>();

    @Override
    public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void clearGuestAccountFlag(UUID accountId) {
      accountIds.add(accountId);
    }
  }
}
