package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class AuthSessionEpochFloorReconcilerTest {
  @Test
  void seedIsIdempotentAndReconcilePreservesRedisAheadOverRevokeAfterDbRollback() {
    UUID first = UUID.fromString("11111111-1111-1111-1111-111111111111");
    UUID second = UUID.fromString("22222222-2222-2222-2222-222222222222");
    MutableDurableEpochSource durable = new MutableDurableEpochSource(Map.of(first, 3L, second, 7L));
    InMemoryFloorStore floors = new InMemoryFloorStore();
    AuthSessionEpochFloorReconciler reconciler = new AuthSessionEpochFloorReconciler(durable, floors);

    reconciler.seedAndReconcile();
    reconciler.seedAndReconcile();
    assertThat(floors.values).containsExactlyInAnyOrderEntriesOf(Map.of(first, 3L, second, 7L));
    assertThat(floors.writeCalls).isEqualTo(4);

    floors.recordAtLeast(first, 9L);
    durable.values.put(first, 1L); // Simulates an Auth DB rollback; Redis must securely over-revoke.
    reconciler.seedAndReconcile();

    assertThat(floors.requireFloor(first)).isEqualTo(9L);
    assertThat(floors.requireFloor(second)).isEqualTo(7L);
  }

  private static final class MutableDurableEpochSource implements DurableAccountEpochSource {
    private final Map<UUID, Long> values = new LinkedHashMap<>();

    private MutableDurableEpochSource(Map<UUID, Long> values) {
      this.values.putAll(values);
    }

    @Override
    public Map<UUID, Long> currentAccountEpochs() {
      return Map.copyOf(values);
    }
  }

  private static final class InMemoryFloorStore implements SessionEpochFloorStore {
    private final Map<UUID, Long> values = new LinkedHashMap<>();
    private int writeCalls;

    @Override
    public long recordAtLeast(UUID accountId, long epoch) {
      writeCalls++;
      return values.merge(accountId, epoch, Math::max);
    }

    @Override
    public long requireFloor(UUID accountId) {
      return values.getOrDefault(accountId, 0L);
    }
  }
}
