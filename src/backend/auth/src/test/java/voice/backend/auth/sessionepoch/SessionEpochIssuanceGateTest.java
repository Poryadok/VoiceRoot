package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.repository.InMemoryAccountRepository;

class SessionEpochIssuanceGateTest {
  @Test
  void equalFloorReturnsExplicitPreparedAccountAndEpochAfterOneFloorWrite() {
    UUID accountId = UUID.fromString("11111111-1111-1111-1111-111111111111");
    RecordingAccounts accounts = new RecordingAccounts();
    RecordingFloors floors = new RecordingFloors(7L);

    PreparedSessionEpoch prepared = new SessionEpochIssuanceGate(accounts, floors).prepare(accountId, 7L);

    assertThat(prepared.accountId()).isEqualTo(accountId);
    assertThat(prepared.sessionEpoch()).isEqualTo(7L);
    assertThat(floors.writeCalls).isEqualTo(1);
    assertThat(floors.accountId).isEqualTo(accountId);
    assertThat(floors.epoch).isEqualTo(7L);
    assertThat(accounts.advanceCalls).isZero();
  }

  @Test
  void redisAheadAdvancesDurableEpochOnceAndUsesItsEvenHigherResultWithoutRetry() {
    UUID accountId = UUID.fromString("22222222-2222-2222-2222-222222222222");
    RecordingAccounts accounts = new RecordingAccounts();
    accounts.advanceResult = 12L;
    RecordingFloors floors = new RecordingFloors(9L);

    PreparedSessionEpoch prepared = new SessionEpochIssuanceGate(accounts, floors).prepare(accountId, 7L);

    assertThat(prepared).isEqualTo(new PreparedSessionEpoch(accountId, 12L));
    assertThat(floors.writeCalls).isEqualTo(1);
    assertThat(floors.epoch).isEqualTo(7L);
    assertThat(accounts.advanceCalls).isEqualTo(1);
    assertThat(accounts.accountId).isEqualTo(accountId);
    assertThat(accounts.requestedEpoch).isEqualTo(9L);
  }

  @Test
  void rejectsNullOrNonpositiveInputBeforeCallingCollaborators() {
    RecordingAccounts accounts = new RecordingAccounts();
    RecordingFloors floors = new RecordingFloors(1L);
    SessionEpochIssuanceGate gate = new SessionEpochIssuanceGate(accounts, floors);

    assertThatThrownBy(() -> gate.prepare(null, 1L)).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> gate.prepare(UUID.randomUUID(), 0L)).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> gate.prepare(UUID.randomUUID(), -1L)).isInstanceOf(IllegalArgumentException.class);

    assertThat(floors.writeCalls).isZero();
    assertThat(accounts.advanceCalls).isZero();
  }

  @Test
  void lowerOrNonpositiveFloorFailsClosedWithoutDurableAdvance() {
    UUID accountId = UUID.randomUUID();
    for (long floor : new long[] {6L, 0L, -1L}) {
      RecordingAccounts accounts = new RecordingAccounts();
      RecordingFloors floors = new RecordingFloors(floor);

      assertThatThrownBy(() -> new SessionEpochIssuanceGate(accounts, floors).prepare(accountId, 7L))
          .isInstanceOf(SessionEpochFloorUnavailableException.class);

      assertThat(floors.writeCalls).isEqualTo(1);
      assertThat(accounts.advanceCalls).isZero();
    }
  }

  @Test
  void floorFailureFailsClosedWithoutDurableAdvanceOrRetry() {
    RecordingAccounts accounts = new RecordingAccounts();
    RecordingFloors floors = new RecordingFloors(7L);
    floors.failure = new IllegalStateException("redis unavailable");

    assertThatThrownBy(() -> new SessionEpochIssuanceGate(accounts, floors).prepare(UUID.randomUUID(), 7L))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasCause(floors.failure);

    assertThat(floors.writeCalls).isEqualTo(1);
    assertThat(accounts.advanceCalls).isZero();
  }

  @Test
  void durableMaxFailureOrResultBelowRedisFloorFailsClosedWithoutRetry() {
    UUID accountId = UUID.randomUUID();
    RecordingFloors floors = new RecordingFloors(9L);
    RecordingAccounts failingAccounts = new RecordingAccounts();
    failingAccounts.failure = new IllegalStateException("database unavailable");

    assertThatThrownBy(
            () -> new SessionEpochIssuanceGate(failingAccounts, floors).prepare(accountId, 7L))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasCause(failingAccounts.failure);
    assertThat(failingAccounts.advanceCalls).isEqualTo(1);
    assertThat(floors.writeCalls).isEqualTo(1);

    RecordingFloors secondFloors = new RecordingFloors(9L);
    RecordingAccounts inadequateAccounts = new RecordingAccounts();
    inadequateAccounts.advanceResult = 8L;

    assertThatThrownBy(
            () -> new SessionEpochIssuanceGate(inadequateAccounts, secondFloors).prepare(accountId, 7L))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
    assertThat(inadequateAccounts.advanceCalls).isEqualTo(1);
    assertThat(secondFloors.writeCalls).isEqualTo(1);
  }

  private static final class RecordingAccounts extends InMemoryAccountRepository {
    private int advanceCalls;
    private UUID accountId;
    private long requestedEpoch;
    private long advanceResult = 9L;
    private RuntimeException failure;

    @Override
    public java.util.Optional<voice.backend.auth.repository.Account> findById(String id) {
      throw new AssertionError("issuance preparation must not reload Account");
    }

    @Override
    public synchronized long advanceSessionEpochAtLeast(UUID accountId, long requestedEpoch) {
      advanceCalls++;
      this.accountId = accountId;
      this.requestedEpoch = requestedEpoch;
      if (failure != null) {
        throw failure;
      }
      return advanceResult;
    }
  }

  private static final class RecordingFloors implements SessionEpochFloorStore {
    private final long result;
    private int writeCalls;
    private UUID accountId;
    private long epoch;
    private RuntimeException failure;

    private RecordingFloors(long result) {
      this.result = result;
    }

    @Override
    public long recordAtLeast(UUID accountId, long epoch) {
      writeCalls++;
      this.accountId = accountId;
      this.epoch = epoch;
      if (failure != null) {
        throw failure;
      }
      return result;
    }

    @Override
    public long requireFloor(UUID accountId) {
      throw new UnsupportedOperationException("not used by issuance preparation");
    }
  }
}
