package voice.backend.auth.repository;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.stream.IntStream;
import java.util.stream.LongStream;
import org.junit.jupiter.api.Test;

class SessionEpochRepositoryTest {
  @Test
  void newInMemoryAccountStartsAtPositiveEpochAndIncrementReturnsNextMonotonicValue() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account account = accounts.create("epoch@example.com", null, "hash", "regular");

    assertThat(account.sessionEpoch()).isEqualTo(1L);
    assertThat(accounts.incrementSessionEpoch(account.id())).isEqualTo(2L);
    assertThat(accounts.incrementSessionEpoch(account.id())).isEqualTo(3L);
    assertThat(accounts.findById(account.id().toString())).get().extracting(Account::sessionEpoch).isEqualTo(3L);
  }

  @Test
  void concurrentIncrementsAreAtomicAndNeverLoseOrReuseEpochValues() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account account = accounts.create("epoch-race@example.com", null, "hash", "regular");
    ExecutorService workers = Executors.newFixedThreadPool(8);
    try {
      List<Future<Long>> increments =
          IntStream.range(0, 16)
              .mapToObj(ignored -> workers.<Long>submit(() -> accounts.incrementSessionEpoch(account.id())))
              .toList();

      assertThat(increments.stream().map(this::await).sorted())
          .containsExactlyElementsOf(LongStream.rangeClosed(2, 17).boxed().toList());
      assertThat(accounts.findById(account.id().toString())).get().extracting(Account::sessionEpoch).isEqualTo(17L);
    } finally {
      workers.shutdownNow();
    }
  }

  private long await(Future<Long> future) {
    try {
      return future.get();
    } catch (Exception ex) {
      throw new AssertionError(ex);
    }
  }
}
