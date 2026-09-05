package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class AccountRestoreTokenStorePeekTest {
  @Test
  void memoryPeekIsRepeatableAndFinalConsumeRemainsOneTime() {
    InMemoryAccountRestoreTokenStore store = new InMemoryAccountRestoreTokenStore();
    UUID accountId = UUID.randomUUID();
    store.store("restore-memory-valid", accountId, Duration.ofMinutes(1));

    assertThat(store.peek("restore-memory-valid")).contains(accountId);
    assertThat(store.peek("restore-memory-valid")).contains(accountId);
    assertThat(store.consume("restore-memory-valid")).contains(accountId);
    assertThat(store.consume("restore-memory-valid")).isEmpty();
  }

  @Test
  void memoryPeekAndConsumeRejectExpiredOrMissingTokens() {
    InMemoryAccountRestoreTokenStore store = new InMemoryAccountRestoreTokenStore();
    store.store("restore-memory-expired", UUID.randomUUID(), Duration.ofSeconds(-1));

    assertThat(store.peek("restore-memory-expired")).isEmpty();
    assertThat(store.consume("restore-memory-expired")).isEmpty();
    assertThat(store.peek("missing")).isEmpty();
  }
}
