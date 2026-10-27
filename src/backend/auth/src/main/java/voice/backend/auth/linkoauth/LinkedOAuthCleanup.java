package voice.backend.auth.linkoauth;

import java.sql.Timestamp;
import java.time.Clock;
import java.util.Map;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.scheduling.annotation.Scheduled;

/** Retention does not depend on provider enablement, registration secrets or decryption keys. */
public final class LinkedOAuthCleanup {
  private final NamedParameterJdbcTemplate jdbc;
  private final Clock clock;

  public LinkedOAuthCleanup(NamedParameterJdbcTemplate jdbc, Clock clock) {
    this.jdbc = jdbc;
    this.clock = clock;
  }

  @Scheduled(fixedDelay = 30000)
  public void cleanup() {
    // Disabled transport also supports a pre-V32 Auth store; never create/repair schema here.
    if (!Boolean.TRUE.equals(jdbc.queryForObject(
        "SELECT to_regclass('public.linked_oauth_attempts') IS NOT NULL", Map.of(), Boolean.class))) return;
    jdbc.update("""
        UPDATE linked_oauth_attempts SET provider_secret=NULL,result_secret=NULL,
          status=CASE WHEN status IN ('pending','processing','ready') THEN 'expired' ELSE status END
        WHERE (provider_secret IS NOT NULL OR result_secret IS NOT NULL OR status IN ('pending','processing','ready'))
          AND (expires_at<=:now OR result_expires_at<=:now OR status IN ('cancelled','superseded','consumed','expired'))
        """, Map.of("now", Timestamp.from(clock.instant())));
  }
}
