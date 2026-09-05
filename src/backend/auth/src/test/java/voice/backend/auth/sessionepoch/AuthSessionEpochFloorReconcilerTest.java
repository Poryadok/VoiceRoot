package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.repository.AccountSessionEpoch;

class AuthSessionEpochFloorReconcilerTest {
  private static final int PAGE_SIZE = 2;

  @Test
  void exhaustsPagesInCursorOrderAndWritesEveryDurableEpochOnce() {
    UUID first = UUID.fromString("11111111-1111-1111-1111-111111111111");
    UUID second = UUID.fromString("22222222-2222-2222-2222-222222222222");
    UUID third = UUID.fromString("33333333-3333-3333-3333-333333333333");
    ScriptedDurableEpochSource durable =
        new ScriptedDurableEpochSource(
            List.of(
                List.of(new AccountSessionEpoch(first, 3L), new AccountSessionEpoch(second, 7L)),
                List.of(new AccountSessionEpoch(third, 9L)),
                List.of()));
    RecordingFloorStore floors = new RecordingFloorStore();

    reconciler(durable, floors).seedAndReconcile();

    assertThat(durable.pageRequests)
        .containsExactly(
            new PageRequest(null, PAGE_SIZE),
            new PageRequest(second, PAGE_SIZE),
            new PageRequest(third, PAGE_SIZE));
    assertThat(floors.writes)
        .containsExactly(
            new FloorWrite(first, 3L), new FloorWrite(second, 7L), new FloorWrite(third, 9L));
    assertThat(floors.values).isEqualTo(Map.of(first, 3L, second, 7L, third, 9L));
    assertThat(durable.advanceCalls).isEmpty();
  }

  @Test
  void redisAheadRepairsTheDurableEpochExactlyOnceWithoutLoweringTheFloor() {
    UUID accountId = UUID.fromString("44444444-4444-4444-4444-444444444444");
    ScriptedDurableEpochSource durable =
        new ScriptedDurableEpochSource(List.of(List.of(new AccountSessionEpoch(accountId, 4L)), List.of()));
    durable.advanceResults.put(accountId, 9L);
    RecordingFloorStore floors = new RecordingFloorStore();
    floors.recordedResults.put(accountId, 9L);

    reconciler(durable, floors).seedAndReconcile();

    assertThat(floors.writes).containsExactly(new FloorWrite(accountId, 4L));
    assertThat(durable.advanceCalls).containsExactly(new AdvanceCall(accountId, 9L));
    assertThat(durable.pageRequests)
        .containsExactly(new PageRequest(null, PAGE_SIZE), new PageRequest(accountId, PAGE_SIZE));
  }

  @Test
  void failsClosedForNullPageResponseAndDoesNotRequestAnotherPage() {
    ScriptedDurableEpochSource durable =
        new ScriptedDurableEpochSource(Collections.singletonList(null));
    RecordingFloorStore floors = new RecordingFloorStore();

    assertThatThrownBy(() -> reconciler(durable, floors).seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(durable.pageRequests).containsExactly(new PageRequest(null, PAGE_SIZE));
    assertThat(floors.writes).isEmpty();
  }

  @Test
  void failsClosedForInvalidDurableRows() {
    UUID accountId = UUID.fromString("55555555-5555-5555-5555-555555555555");

    assertThatThrownBy(
            () ->
                reconciler(
                        new ScriptedDurableEpochSource(
                            List.of(List.of(new AccountSessionEpoch(accountId, 0L)))),
                        new RecordingFloorStore())
                    .seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
    assertThatThrownBy(
            () ->
                reconciler(
                        new ScriptedDurableEpochSource(
                            List.of(List.of(new AccountSessionEpoch(null, 1L)))),
                        new RecordingFloorStore())
                    .seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
  }

  @Test
  void failsClosedForNullDurableRowWithoutWritingOrRequestingAnotherPage() {
    ScriptedDurableEpochSource durable =
        new ScriptedDurableEpochSource(
            Collections.singletonList(Collections.singletonList((AccountSessionEpoch) null)));
    RecordingFloorStore floors = new RecordingFloorStore();

    assertThatThrownBy(() -> reconciler(durable, floors).seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(durable.pageRequests).containsExactly(new PageRequest(null, PAGE_SIZE));
    assertThat(floors.writes).isEmpty();
  }

  @Test
  void failsClosedWhenStoreReturnsAnInvalidOrLowerFloor() {
    UUID accountId = UUID.fromString("66666666-6666-6666-6666-666666666666");
    ScriptedDurableEpochSource durable =
        new ScriptedDurableEpochSource(List.of(List.of(new AccountSessionEpoch(accountId, 4L))));
    RecordingFloorStore floors = new RecordingFloorStore();
    floors.recordedResults.put(accountId, 3L);

    assertThatThrownBy(() -> reconciler(durable, floors).seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(floors.writes).containsExactly(new FloorWrite(accountId, 4L));
    assertThat(durable.advanceCalls).isEmpty();
  }

  @Test
  void failsClosedWhenRedisAheadRepairDoesNotReachTheRecordedFloor() {
    UUID accountId = UUID.fromString("77777777-7777-7777-7777-777777777777");
    ScriptedDurableEpochSource durable =
        new ScriptedDurableEpochSource(List.of(List.of(new AccountSessionEpoch(accountId, 4L))));
    durable.advanceResults.put(accountId, 8L);
    RecordingFloorStore floors = new RecordingFloorStore();
    floors.recordedResults.put(accountId, 9L);

    assertThatThrownBy(() -> reconciler(durable, floors).seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(durable.advanceCalls).containsExactly(new AdvanceCall(accountId, 9L));
  }

  @Test
  void sourceAndStoreFailuresStopLaterRowsAndPagesWithoutRetries() {
    UUID first = UUID.fromString("88888888-8888-8888-8888-888888888888");
    UUID second = UUID.fromString("99999999-9999-9999-9999-999999999999");
    ScriptedDurableEpochSource sourceFailure =
        new ScriptedDurableEpochSource(
            List.of(List.of(new AccountSessionEpoch(first, 2L)), List.of(new AccountSessionEpoch(second, 3L))));
    sourceFailure.failPageRequest = 1;
    RecordingFloorStore sourceFailureFloors = new RecordingFloorStore();

    assertThatThrownBy(() -> reconciler(sourceFailure, sourceFailureFloors).seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(sourceFailure.pageRequests)
        .containsExactly(new PageRequest(null, PAGE_SIZE), new PageRequest(first, PAGE_SIZE));
    assertThat(sourceFailureFloors.writes).containsExactly(new FloorWrite(first, 2L));

    ScriptedDurableEpochSource storeFailure =
        new ScriptedDurableEpochSource(
            List.of(List.of(new AccountSessionEpoch(first, 2L), new AccountSessionEpoch(second, 3L))));
    RecordingFloorStore failingFloors = new RecordingFloorStore();
    failingFloors.failAccountId = first;

    assertThatThrownBy(() -> reconciler(storeFailure, failingFloors).seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(storeFailure.pageRequests).containsExactly(new PageRequest(null, PAGE_SIZE));
    assertThat(failingFloors.writes).containsExactly(new FloorWrite(first, 2L));
  }

  @Test
  void failsClosedWhenNextPageRepeatsThePreviousCursorBeforeWritingIt() {
    UUID accountId = UUID.fromString("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa");
    ScriptedDurableEpochSource durable =
        new ScriptedDurableEpochSource(
            List.of(
                List.of(new AccountSessionEpoch(accountId, 2L)),
                List.of(new AccountSessionEpoch(accountId, 3L))));
    RecordingFloorStore floors = new RecordingFloorStore();

    assertThatThrownBy(() -> reconciler(durable, floors).seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(durable.pageRequests)
        .containsExactly(new PageRequest(null, PAGE_SIZE), new PageRequest(accountId, PAGE_SIZE));
    assertThat(floors.writes).containsExactly(new FloorWrite(accountId, 2L));
  }

  @Test
  void failsClosedForDuplicateIdsWithinOnePageBeforeWritingTheDuplicate() {
    UUID accountId = UUID.fromString("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb");
    ScriptedDurableEpochSource durable =
        new ScriptedDurableEpochSource(
            List.of(List.of(new AccountSessionEpoch(accountId, 2L), new AccountSessionEpoch(accountId, 3L))));
    RecordingFloorStore floors = new RecordingFloorStore();

    assertThatThrownBy(() -> reconciler(durable, floors).seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(durable.pageRequests).containsExactly(new PageRequest(null, PAGE_SIZE));
    assertThat(floors.writes).containsExactly(new FloorWrite(accountId, 2L));
  }

  @Test
  void failsClosedForDescendingUuidRowBeforeWritingTheOutOfOrderRow() {
    UUID first = UUID.fromString("cccccccc-cccc-cccc-cccc-cccccccccccc");
    UUID second = UUID.fromString("dddddddd-dddd-dddd-dddd-dddddddddddd");
    ScriptedDurableEpochSource durable =
        new ScriptedDurableEpochSource(
            List.of(List.of(new AccountSessionEpoch(second, 2L), new AccountSessionEpoch(first, 3L))));
    RecordingFloorStore floors = new RecordingFloorStore();

    assertThatThrownBy(() -> reconciler(durable, floors).seedAndReconcile())
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(durable.pageRequests).containsExactly(new PageRequest(null, PAGE_SIZE));
    assertThat(floors.writes).containsExactly(new FloorWrite(second, 2L));
  }

  @Test
  void rejectsNonpositivePageSizeBeforeAnyDurableOrFloorOperation() {
    ScriptedDurableEpochSource durable = new ScriptedDurableEpochSource(List.of());
    RecordingFloorStore floors = new RecordingFloorStore();

    assertThatThrownBy(() -> new AuthSessionEpochFloorReconciler(durable, floors, 0))
        .isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new AuthSessionEpochFloorReconciler(durable, floors, -1))
        .isInstanceOf(IllegalArgumentException.class);

    assertThat(durable.pageRequests).isEmpty();
    assertThat(floors.writes).isEmpty();
  }

  private static AuthSessionEpochFloorReconciler reconciler(
      DurableAccountEpochSource durable, SessionEpochFloorStore floors) {
    return new AuthSessionEpochFloorReconciler(durable, floors, PAGE_SIZE);
  }

  private record PageRequest(UUID exclusiveAfter, int limit) {}

  private record FloorWrite(UUID accountId, long epoch) {}

  private record AdvanceCall(UUID accountId, long requestedEpoch) {}

  private static final class ScriptedDurableEpochSource implements DurableAccountEpochSource {
    private final List<List<AccountSessionEpoch>> pages;
    private final List<PageRequest> pageRequests = new ArrayList<>();
    private final List<AdvanceCall> advanceCalls = new ArrayList<>();
    private final Map<UUID, Long> advanceResults = new LinkedHashMap<>();
    private int nextPage;
    private int failPageRequest = -1;

    private ScriptedDurableEpochSource(List<List<AccountSessionEpoch>> pages) {
      this.pages = new ArrayList<>(pages);
    }

    @Override
    public List<AccountSessionEpoch> pageSessionEpochsAfter(UUID exclusiveAfter, int limit) {
      pageRequests.add(new PageRequest(exclusiveAfter, limit));
      if (nextPage == failPageRequest) {
        throw new IllegalStateException("durable source unavailable");
      }
      if (nextPage >= pages.size()) {
        throw new AssertionError("unexpected durable page request");
      }
      return pages.get(nextPage++);
    }

    @Override
    public long advanceSessionEpochAtLeast(UUID accountId, long requestedEpoch) {
      advanceCalls.add(new AdvanceCall(accountId, requestedEpoch));
      Long result = advanceResults.get(accountId);
      if (result == null) {
        throw new AssertionError("unexpected durable max advance");
      }
      return result;
    }
  }

  private static final class RecordingFloorStore implements SessionEpochFloorStore {
    private final Map<UUID, Long> values = new LinkedHashMap<>();
    private final List<FloorWrite> writes = new ArrayList<>();
    private final Map<UUID, Long> recordedResults = new LinkedHashMap<>();
    private UUID failAccountId;

    @Override
    public long recordAtLeast(UUID accountId, long epoch) {
      writes.add(new FloorWrite(accountId, epoch));
      if (accountId.equals(failAccountId)) {
        throw new IllegalStateException("floor store unavailable");
      }
      Long result = recordedResults.get(accountId);
      if (result != null) {
        return result;
      }
      return values.merge(accountId, epoch, Math::max);
    }

    @Override
    public long requireFloor(UUID accountId) {
      return values.getOrDefault(accountId, 0L);
    }
  }
}
