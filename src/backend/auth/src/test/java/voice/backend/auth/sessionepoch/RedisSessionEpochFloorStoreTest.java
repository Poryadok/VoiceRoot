package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.Duration;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class RedisSessionEpochFloorStoreTest {
  private static final Duration COMMAND_TIMEOUT = Duration.ofSeconds(2);

  @Test
  void writesExactAuthOwnedKeyWithAtomicMaxWithoutExpiry() {
    UUID accountId = UUID.fromString("11111111-1111-1111-1111-111111111111");
    RecordingCommands commands = new RecordingCommands();
    RedisSessionEpochFloorStore store = new RedisSessionEpochFloorStore(commands, COMMAND_TIMEOUT);

    assertThat(store.keyFor(accountId)).isEqualTo("auth:session:min_epoch:" + accountId);
    assertThat(store.recordAtLeast(accountId, 3L)).isEqualTo(3L);
    assertThat(store.recordAtLeast(accountId, 9L)).isEqualTo(9L);
    assertThat(store.recordAtLeast(accountId, 4L)).isEqualTo(9L);

    assertThat(commands.values).containsEntry(store.keyFor(accountId), 9L);
    assertThat(commands.timeouts).containsOnly(COMMAND_TIMEOUT);
  }

  @Test
  void rejectsNonPositiveEpochAndFailsClosedForCorruptOrUnavailableRedis() {
    UUID accountId = UUID.randomUUID();
    RecordingCommands commands = new RecordingCommands();
    RedisSessionEpochFloorStore store = new RedisSessionEpochFloorStore(commands, COMMAND_TIMEOUT);

    assertThatThrownBy(() -> store.recordAtLeast(accountId, 0L))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("session epoch");
    assertThatThrownBy(() -> store.recordAtLeast(accountId, -1L))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("session epoch");

    commands.values.put(store.keyFor(accountId), 0L);
    assertThatThrownBy(() -> store.requireFloor(accountId))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("invalid");

    commands.failure = new RuntimeException("redis timeout");
    assertThatThrownBy(() -> store.recordAtLeast(accountId, 1L))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasCauseInstanceOf(RuntimeException.class);
    assertThat(commands.timeouts).containsOnly(COMMAND_TIMEOUT);
  }

  @Test
  void commandThatIgnoresTheBoundStillFailsClosedWithinTwoSeconds() {
    RedisSessionEpochFloorStore store = new RedisSessionEpochFloorStore(new BlockingCommands(), COMMAND_TIMEOUT);
    long startedAtNanos = System.nanoTime();

    assertThatThrownBy(() -> store.recordAtLeast(UUID.randomUUID(), 1L))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("timeout");

    assertThat(Duration.ofNanos(System.nanoTime() - startedAtNanos)).isLessThanOrEqualTo(Duration.ofMillis(2500));
  }

  private static final class RecordingCommands implements RedisSessionEpochCommands {
    private final Map<String, Long> values = new HashMap<>();
    private final java.util.List<Duration> timeouts = new java.util.ArrayList<>();
    private RuntimeException failure;

    @Override
    public long atomicMaxWithoutExpiry(String key, long candidate, Duration timeout) {
      timeouts.add(timeout);
      if (failure != null) {
        throw failure;
      }
      return values.merge(key, candidate, Math::max);
    }

    @Override
    public long readRequiredPositive(String key, Duration timeout) {
      timeouts.add(timeout);
      if (failure != null) {
        throw failure;
      }
      return values.getOrDefault(key, 0L);
    }
  }

  private static final class BlockingCommands implements RedisSessionEpochCommands {
    @Override
    public long atomicMaxWithoutExpiry(String key, long candidate, Duration timeout) {
      try {
        Thread.sleep(Duration.ofSeconds(10));
      } catch (InterruptedException ex) {
        Thread.currentThread().interrupt();
        throw new RuntimeException("interrupted", ex);
      }
      throw new AssertionError("bounded command was not cancelled");
    }

    @Override
    public long readRequiredPositive(String key, Duration timeout) {
      throw new UnsupportedOperationException("not used");
    }
  }
}
