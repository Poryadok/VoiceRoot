package voice.backend.auth.repository;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;
import static org.mockito.Mockito.verify;

import java.lang.reflect.Method;
import java.lang.reflect.InvocationTargetException;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import java.util.Arrays;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

class AccountRestoreTransitionRepositoryContractTest {
  private static final Instant DELETED_AT = Instant.parse("2026-04-01T10:00:00Z");
  private static final Instant CUTOFF = DELETED_AT.plus(Duration.ofDays(30));
  private static final Instant AFTER_CUTOFF = CUTOFF.plusNanos(1);

  @Test
  void inMemoryRestoreRejectsAfterCutoffAndKeepsAccountDeleted() throws Exception {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account account = accounts.create("repository-expired@example.com", null, "hash", "regular");
    accounts.markDeleted(account.id(), DELETED_AT);

    assertThat(invokeConditionalRestore(accounts, account.id(), AFTER_CUTOFF)).isFalse();
    assertThat(accounts.findById(account.id().toString()).orElseThrow())
        .extracting(Account::status, Account::deletedAt)
        .containsExactly("deleted", DELETED_AT);
  }

  @Test
  void inMemoryRestoreAllowsExactCutoffAndClearsDeletedState() throws Exception {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account account = accounts.create("repository-boundary@example.com", null, "hash", "regular");
    accounts.markDeleted(account.id(), DELETED_AT);

    assertThat(invokeConditionalRestore(accounts, account.id(), CUTOFF)).isTrue();
    assertThat(accounts.findById(account.id().toString()).orElseThrow())
        .extracting(Account::status, Account::deletedAt)
        .containsExactly("active", null);
  }

  @Test
  void jdbcRestoreUpdateCarriesTransitionInstantAndExpiryPredicate() throws Exception {
    NamedParameterJdbcTemplate jdbc = mock();
    when(jdbc.update(anyString(), any(MapSqlParameterSource.class))).thenReturn(0);
    JdbcAccountRepository accounts = new JdbcAccountRepository(jdbc);
    UUID accountId = UUID.randomUUID();

    assertThat(invokeConditionalRestore(accounts, accountId, AFTER_CUTOFF)).isFalse();

    ArgumentCaptor<String> sql = ArgumentCaptor.forClass(String.class);
    ArgumentCaptor<MapSqlParameterSource> parameters =
        ArgumentCaptor.forClass(MapSqlParameterSource.class);
    verify(jdbc).update(sql.capture(), parameters.capture());
    String normalizedSql = sql.getValue().replaceAll("\\s+", " ").toLowerCase();
    assertThat(normalizedSql).contains("interval '30 days'");
    Map<String, Object> values = parameters.getValue().getValues();
    assertThat(values).containsEntry("id", accountId);
    String transitionParameter =
        values.entrySet().stream()
            .filter(entry -> entry.getValue() instanceof Timestamp)
            .map(Map.Entry::getKey)
            .findFirst()
            .orElseThrow(() -> new AssertionError("transition instant parameter is missing"));
    assertThat(normalizedSql).contains("deleted_at");
    assertThat(normalizedSql).contains(":" + transitionParameter);
  }

  private static boolean invokeConditionalRestore(
      AccountRepository accounts, UUID accountId, Instant transitionInstant) throws Exception {
    Method method = conditionalRestoreMethod(accounts.getClass());
    try {
      return (boolean) method.invoke(accounts, accountId, transitionInstant);
    } catch (InvocationTargetException ex) {
      throw new AssertionError("conditional restore invocation failed", ex.getCause());
    }
  }

  private static Method conditionalRestoreMethod(Class<?> repositoryType) {
    Optional<Method> method =
        Arrays.stream(repositoryType.getMethods())
            .filter(candidate -> candidate.getName().equals("restoreDeleted"))
            .filter(
                candidate ->
                    Arrays.equals(
                        candidate.getParameterTypes(), new Class<?>[] {UUID.class, Instant.class}))
            .findFirst();
    assertThat(method)
        .as("restoreDeleted(UUID, Instant) must atomically fence the recovery cutoff")
        .isPresent();
    return method.orElseThrow();
  }
}
