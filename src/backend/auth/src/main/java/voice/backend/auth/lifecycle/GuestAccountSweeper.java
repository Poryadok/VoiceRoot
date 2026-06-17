package voice.backend.auth.lifecycle;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import voice.backend.auth.repository.AccountRepository;

/** Deactivates guest accounts inactive for 30 days (auth-and-contacts.md lifecycle). */
public class GuestAccountSweeper {
  private static final Logger log = LoggerFactory.getLogger(GuestAccountSweeper.class);
  static final Duration GUEST_TTL = Duration.ofDays(30);

  private final AccountRepository accounts;
  private final Clock clock;

  public GuestAccountSweeper(AccountRepository accounts, Clock clock) {
    this.accounts = accounts;
    this.clock = clock;
  }

  @Scheduled(cron = "0 15 3 * * *")
  public void sweep() {
    Instant cutoff = Instant.now(clock).minus(GUEST_TTL);
    int deactivated = accounts.deactivateExpiredGuests(cutoff);
    if (deactivated > 0) {
      log.info("deactivated {} expired guest accounts (last_online before {})", deactivated, cutoff);
    }
  }
}
