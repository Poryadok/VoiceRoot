package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;

class InMemoryRefreshTokenRepositoryClockTest {
  @Test
  void activeSessionsUseFixtureTimeAndExpireAtDeadlineEquality() {
    Instant expiry = Instant.parse("2026-10-04T12:00:00Z");
    UUID accountId = UUID.randomUUID();
    for (Instant now : new Instant[] {expiry.minusNanos(1), expiry, expiry.plusNanos(1)}) {
      Clock clock = Clock.fixed(now, ZoneOffset.UTC);
      var repository = new InMemoryRefreshTokenRepository(clock);
      var active = repository.create(accountId, "active", "{}", "jti", expiry, now.minusSeconds(60));
      repository.create(accountId, "revoked", "{}", "jti", expiry, now.minusSeconds(60));
      repository.revoke("revoked", now.minusSeconds(1));
      repository.create(UUID.randomUUID(), "other-account", "{}", "jti", expiry, now.minusSeconds(60));

      if (clock.instant().isBefore(expiry)) {
        assertThat(repository.listActiveByAccount(accountId)).containsExactly(active);
      } else {
        assertThat(repository.listActiveByAccount(accountId)).isEmpty();
      }
    }
  }

  @Test
  void defaultConstructorStillUsesCurrentTime() {
    var repository = new InMemoryRefreshTokenRepository();
    UUID accountId = UUID.randomUUID();
    Instant now = Instant.now();
    repository.create(accountId, "expired", "{}", "jti", Instant.EPOCH, now);
    var active = repository.create(accountId, "active", "{}", "jti", Instant.parse("9999-12-31T00:00:00Z"), now);

    assertThat(repository.listActiveByAccount(accountId)).containsExactly(active);
  }
}
