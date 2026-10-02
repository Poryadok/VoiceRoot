package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.sql.Connection;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.utility.DockerImageName;

class AuthAuthoritySourceMigrationJdbcIntegrationTest {
  private static long revision(JdbcTemplate jdbc) {
    return jdbc.queryForObject("SELECT revision FROM auth_authority_revision WHERE singleton", Long.class);
  }

  @Test
  void owningClockCoversCommittedChangesAndSurvivesRollbackAndBulkRemoval() throws Exception {
    try (var postgres = new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
        .withDatabaseName("auth_source").withUsername("fixture").withPassword("fixture")
        .withReuse(false).withLabel("voice.task", "auth-source-20261002")) {
      postgres.start();
      var dataSource = new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
      var jdbc = new JdbcTemplate(dataSource);
      Flyway.configure().dataSource(dataSource).locations("classpath:db/migration").load().migrate();
      assertThat(revision(jdbc)).isEqualTo(1);
      var account = UUID.randomUUID();
      jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES(?,'fixture','regular','active')", account);
      long created = revision(jdbc);
      assertThat(created).isGreaterThan(1);
      try (var snapshot = dataSource.getConnection()) {
        snapshot.setAutoCommit(false);
        snapshot.setReadOnly(true);
        snapshot.setTransactionIsolation(Connection.TRANSACTION_REPEATABLE_READ);
        try (var statement = snapshot.createStatement(); var rows = statement.executeQuery("SELECT revision FROM auth_authority_revision WHERE singleton")) {
          assertThat(rows.next()).isTrue();
          assertThat(rows.getLong(1)).isEqualTo(created);
        }
        jdbc.update("UPDATE accounts SET status='suspended' WHERE id=?", account);
        assertThat(revision(jdbc)).isGreaterThan(created);
        try (var statement = snapshot.createStatement(); var rows = statement.executeQuery("SELECT revision,(SELECT status FROM accounts WHERE id='"+account+"') FROM auth_authority_revision WHERE singleton")) {
          assertThat(rows.next()).isTrue();
          assertThat(rows.getLong(1)).isEqualTo(created);
          assertThat(rows.getString(2)).isEqualTo("active");
        }
        snapshot.rollback();
      }
      long suspended = revision(jdbc);
      try (var transaction = dataSource.getConnection()) {
        transaction.setAutoCommit(false);
        try (var statement = transaction.prepareStatement("UPDATE accounts SET status='active' WHERE id=?")) {
          statement.setObject(1, account); statement.executeUpdate();
        }
        transaction.rollback();
      }
      assertThat(revision(jdbc)).isEqualTo(suspended);
      var sdk = UUID.randomUUID();
      jdbc.update("INSERT INTO sdk_identities(account_id,actor_id,application_id,environment_id,issuer,provider_subject,created_at) VALUES(?,?,?,?,'fixture','private-subject',now())", sdk, UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID());
      long identity = revision(jdbc);
      assertThat(identity).isGreaterThan(suspended);
      var device = UUID.randomUUID();
      jdbc.update("INSERT INTO sdk_devices(device_id,account_id,thumbprint,public_jwk) VALUES(?,?,'fixture','fixture')", device,sdk);
      long enrolled = revision(jdbc);
      assertThat(enrolled).isGreaterThan(identity);
      jdbc.update("UPDATE sdk_devices SET revoked_at=now() WHERE device_id=?", device);
      assertThat(revision(jdbc)).isGreaterThan(enrolled);
      long revoked = revision(jdbc);
      for (String sql : java.util.List.of("DELETE FROM auth_authority_revision", "TRUNCATE auth_authority_revision", "UPDATE auth_authority_revision SET revision=revision-1")) {
        assertThatThrownBy(() -> jdbc.execute(sql)).isInstanceOf(org.springframework.dao.DataAccessException.class);
        assertThat(revision(jdbc)).isEqualTo(revoked);
      }
      jdbc.execute("TRUNCATE sdk_sessions");
      assertThat(revision(jdbc)).isGreaterThan(revoked);
      assertThat(jdbc.queryForObject("SELECT count(*) FROM pg_trigger WHERE tgname LIKE 'auth_source_%' AND NOT tgisinternal AND tgenabled='O'", Integer.class)).isEqualTo(20);
    }
  }
}
