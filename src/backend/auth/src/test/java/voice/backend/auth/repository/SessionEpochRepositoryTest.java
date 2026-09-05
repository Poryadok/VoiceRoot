package voice.backend.auth.repository;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatIllegalArgumentException;

import java.lang.reflect.Field;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.stream.IntStream;
import java.util.stream.LongStream;
import org.junit.jupiter.api.Test;

class SessionEpochRepositoryTest {
  @Test
  void advanceAtLeastPersistsOnlyTheHigherPositiveEpochAndPreservesAccountFields() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account created = accounts.create("max-epoch@example.com", null, "hash", "regular");
    accounts.saveTotpSecret(created.id(), new byte[] {1, 2, 3}, true);
    Instant deletedAt = Instant.parse("2026-09-06T08:00:00Z");
    accounts.markDeleted(created.id(), deletedAt);
    Account before = accounts.findById(created.id().toString()).orElseThrow();

    assertThat(accounts.advanceSessionEpochAtLeast(created.id(), 7L)).isEqualTo(7L);
    assertThat(accounts.advanceSessionEpochAtLeast(created.id(), 3L)).isEqualTo(7L);
    assertThat(accounts.findById(created.id().toString()).orElseThrow())
        .extracting(
            Account::id,
            Account::email,
            Account::phone,
            Account::passwordHash,
            Account::type,
            Account::status,
            Account::totpEnabled,
            Account::createdAt,
            Account::deletedAt,
            Account::sessionEpoch)
        .containsExactly(
            before.id(),
            before.email(),
            before.phone(),
            before.passwordHash(),
            before.type(),
            before.status(),
            before.totpEnabled(),
            before.createdAt(),
            before.deletedAt(),
            7L);
    assertThat(accounts.findById(created.id().toString()).orElseThrow().totpSecret())
        .containsExactly(before.totpSecret());
  }

  @Test
  void advanceAtLeastRejectsNonpositiveEpochsAndMissingAccounts() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account account = accounts.create("invalid-epoch@example.com", null, "hash", "regular");

    assertThatIllegalArgumentException().isThrownBy(() -> accounts.advanceSessionEpochAtLeast(account.id(), 0));
    assertThatIllegalArgumentException().isThrownBy(() -> accounts.advanceSessionEpochAtLeast(account.id(), -1));
    assertThatIllegalArgumentException()
        .isThrownBy(() -> accounts.advanceSessionEpochAtLeast(UUID.randomUUID(), 2));
  }

  @Test
  void keysetPagesUseUnsignedPostgresUuidOrderAndIncludeDeletedAccounts() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    UUID zero = UUID.fromString("00000000-0000-0000-0000-000000000001");
    UUID lowerHighBit = UUID.fromString("7fffffff-ffff-ffff-ffff-ffffffffffff");
    UUID upperHighBitLowerLsb = UUID.fromString("80000000-0000-0000-0000-000000000001");
    UUID upperHighBit = UUID.fromString("80000000-0000-0000-8000-000000000000");
    UUID allBits = UUID.fromString("ffffffff-ffff-ffff-ffff-ffffffffffff");
    for (UUID id : List.of(zero, lowerHighBit, upperHighBitLowerLsb, upperHighBit, allBits)) {
      replaceCreatedAccountId(accounts, id);
      assertThat(accounts.advanceSessionEpochAtLeast(id, id.equals(allBits) ? 9L : 2L))
          .isEqualTo(id.equals(allBits) ? 9L : 2L);
    }
    accounts.markDeleted(upperHighBit, Instant.parse("2026-09-06T08:01:00Z"));

    List<AccountSessionEpoch> first = accounts.pageSessionEpochsAfter(null, 2);
    List<AccountSessionEpoch> second = accounts.pageSessionEpochsAfter(first.getLast().accountId(), 2);
    List<AccountSessionEpoch> third = accounts.pageSessionEpochsAfter(second.getLast().accountId(), 2);
    List<AccountSessionEpoch> terminal = accounts.pageSessionEpochsAfter(third.getLast().accountId(), 2);

    assertThat(first).extracting(AccountSessionEpoch::accountId).containsExactly(zero, lowerHighBit);
    assertThat(second)
        .extracting(AccountSessionEpoch::accountId)
        .containsExactly(upperHighBitLowerLsb, upperHighBit);
    assertThat(third).extracting(AccountSessionEpoch::accountId).containsExactly(allBits);
    assertThat(first).extracting(AccountSessionEpoch::sessionEpoch).containsExactly(2L, 2L);
    assertThat(second).extracting(AccountSessionEpoch::sessionEpoch).containsExactly(2L, 2L);
    assertThat(third).extracting(AccountSessionEpoch::sessionEpoch).containsExactly(9L);
    assertThat(terminal).isEmpty();
    assertThatIllegalArgumentException().isThrownBy(() -> accounts.pageSessionEpochsAfter(null, 0));
    assertThatIllegalArgumentException().isThrownBy(() -> accounts.pageSessionEpochsAfter(null, -1));
  }

  @Test
  void deletionAdvancesTheEpochInTheSameDurableTransition() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account account = accounts.create("delete-epoch@example.com", null, "hash", "regular");
    Instant deletedAt = Instant.parse("2026-09-05T11:00:00Z");

    long epoch = accounts.markDeletedAndIncrementSessionEpoch(account.id(), deletedAt);

    assertThat(epoch).isEqualTo(2L);
    assertThat(accounts.findById(account.id().toString()))
        .get()
        .satisfies(
            deleted -> {
              assertThat(deleted.status()).isEqualTo("deleted");
              assertThat(deleted.deletedAt()).isEqualTo(deletedAt);
              assertThat(deleted.sessionEpoch()).isEqualTo(epoch);
            });
  }

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

  @SuppressWarnings("unchecked")
  private static void replaceCreatedAccountId(InMemoryAccountRepository accounts, UUID replacementId) {
    Account created = accounts.create(replacementId + "@example.com", null, "hash", "regular");
    try {
      Field field = InMemoryAccountRepository.class.getDeclaredField("byId");
      field.setAccessible(true);
      Map<UUID, Account> byId = (Map<UUID, Account>) field.get(accounts);
      byId.remove(created.id());
      byId.put(
          replacementId,
          new Account(
              replacementId,
              created.email(),
              created.phone(),
              created.passwordHash(),
              created.type(),
              created.status(),
              created.totpSecret(),
              created.totpEnabled(),
              created.sessionEpoch(),
              created.createdAt(),
              created.deletedAt()));
    } catch (ReflectiveOperationException exception) {
      throw new AssertionError("failed to seed fixed UUID fixture", exception);
    }
  }
}
