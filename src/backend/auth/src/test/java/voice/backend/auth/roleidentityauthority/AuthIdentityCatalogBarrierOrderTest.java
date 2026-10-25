package voice.backend.auth.roleidentityauthority;

import java.util.ArrayList;
import java.util.List;
import java.sql.Connection;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataAccessResourceFailureException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/** Narrow JDBC admission tests; native event-trigger exclusion is a separate required proof. */
final class AuthIdentityCatalogBarrierOrderTest {
  private static final String BARRIER =
      "LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS SHARE MODE";

  @Test void barrierIsTheFirstLockBeforeCatalogOrDomainAccess() {
    var jdbc = new StoppingJdbc();
    TransactionSynchronizationManager.setActualTransactionActive(true);
    TransactionSynchronizationManager.setCurrentTransactionIsolationLevel(Connection.TRANSACTION_READ_COMMITTED);
    try {
      expectStop(jdbc);
      if (!jdbc.statements.getLast().equals(BARRIER)) {
        throw new AssertionError("first lock must be the direct identity DDL barrier: " + jdbc.statements);
      }
    } finally { TransactionSynchronizationManager.clear(); }
  }

  @Test void missingOrBlockedBarrierPropagatesBeforeAnyCatalogRead() {
    var jdbc = new StoppingJdbc();
    TransactionSynchronizationManager.setActualTransactionActive(true);
    TransactionSynchronizationManager.setCurrentTransactionIsolationLevel(Connection.TRANSACTION_READ_COMMITTED);
    try {
      expectStop(jdbc);
      if (jdbc.statements.stream().anyMatch(sql -> sql.contains("public.accounts"))) {
        throw new AssertionError("domain relation was touched before barrier admission");
      }
    } finally { TransactionSynchronizationManager.clear(); }
  }

  @Test void missingTransactionPerformsNoSql() {
    TransactionSynchronizationManager.clear();
    var jdbc = new StoppingJdbc();
    try {
      new AuthIdentityCatalog().validate(jdbc);
      throw new AssertionError("transaction must be required");
    } catch (IllegalStateException expected) {
      if (!jdbc.statements.isEmpty()) throw new AssertionError("SQL outside transaction");
    }
  }

  @Test void unknownOrNonReadCommittedIsolationPerformsNoSql() {
    Integer[] levels = {null, Connection.TRANSACTION_READ_UNCOMMITTED,
        Connection.TRANSACTION_REPEATABLE_READ, Connection.TRANSACTION_SERIALIZABLE};
    for (Integer level : levels) {
      var jdbc = new StoppingJdbc();
      TransactionSynchronizationManager.setActualTransactionActive(true);
      TransactionSynchronizationManager.setCurrentTransactionIsolationLevel(level);
      try {
        new AuthIdentityCatalog().validate(jdbc);
        throw new AssertionError("isolation must be explicitly READ_COMMITTED: " + level);
      } catch (IllegalStateException expected) {
        if (!jdbc.statements.isEmpty()) throw new AssertionError("SQL before isolation admission");
      } finally { TransactionSynchronizationManager.clear(); }
    }
  }

  private static void expectStop(StoppingJdbc jdbc) {
    try {
      new AuthIdentityCatalog().validate(jdbc);
      throw new AssertionError("barrier admission failure must propagate");
    } catch (DataAccessResourceFailureException failure) {
      if (failure != jdbc.failure) throw new AssertionError("failure replaced", failure);
    }
  }

  private static final class StoppingJdbc extends JdbcTemplate {
    final List<String> statements = new ArrayList<>();
    final DataAccessResourceFailureException failure =
        new DataAccessResourceFailureException("fixture barrier unavailable");
    @Override public void execute(String sql) {
      statements.add(sql);
      if (sql.startsWith("LOCK TABLE")) throw failure;
    }
    @Override public <T> T queryForObject(String sql, Class<T> type) {
      throw new AssertionError("catalog read before barrier admission");
    }
  }

  /** Permits the same scoped checks in an offline Linux javac runner without Maven regeneration. */
  public static void main(String[] args) {
    if (!System.getProperty("os.name").equals("Linux")) throw new AssertionError("Linux required");
    var suite = new AuthIdentityCatalogBarrierOrderTest();
    suite.barrierIsTheFirstLockBeforeCatalogOrDomainAccess();
    suite.missingOrBlockedBarrierPropagatesBeforeAnyCatalogRead();
    suite.missingTransactionPerformsNoSql();
    suite.unknownOrNonReadCommittedIsolationPerformsNoSql();
    System.out.println("PASS 4 barrier-admission unit checks (4 isolation negatives); no PostgreSQL concurrency claim");
  }
}
