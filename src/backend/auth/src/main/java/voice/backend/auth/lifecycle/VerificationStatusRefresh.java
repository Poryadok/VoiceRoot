package voice.backend.auth.lifecycle;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import voice.backend.auth.service.LinkedAccountsService;

/**
 * Periodic re-check of Twitch/YouTube partner status (docs/features/verification.md). Clears badge
 * when the linked platform no longer qualifies.
 */
public class VerificationStatusRefresh {
  private static final Logger log = LoggerFactory.getLogger(VerificationStatusRefresh.class);

  private final LinkedAccountsService linkedAccounts;

  public VerificationStatusRefresh(LinkedAccountsService linkedAccounts) {
    this.linkedAccounts = linkedAccounts;
  }

  @Scheduled(cron = "${auth.verification.refresh-cron:0 30 4 * * *}")
  public void refresh() {
    int cleared = linkedAccounts.refreshVerificationStatuses();
    if (cleared > 0) {
      log.info("verification status refresh cleared {} badges", cleared);
    }
  }
}
