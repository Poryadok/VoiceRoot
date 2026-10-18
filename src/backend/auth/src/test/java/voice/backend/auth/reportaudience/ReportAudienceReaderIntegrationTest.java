package voice.backend.auth.reportaudience;

import static org.assertj.core.api.Assertions.*;
import app.voice.auth.v1.ReportAudienceAccountState;
import java.util.List;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.utility.DockerImageName;

class ReportAudienceReaderIntegrationTest {
  @Test void classifiesEveryAccountAndRetainsGuestsAndSuspensions() {
    try(var pg=new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-bookworm")).withDatabaseName("audience").withUsername("fixture").withPassword("fixture")) {
      pg.start();var source=new DriverManagerDataSource(pg.getJdbcUrl(),pg.getUsername(),pg.getPassword());
      Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
      var jdbc=new JdbcTemplate(source);var live=UUID.randomUUID();var guest=UUID.randomUUID();var suspended=UUID.randomUUID();var deleted=UUID.randomUUID();var missing=UUID.randomUUID();
      jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES (?,'fixture','regular','active'),(?,'fixture','guest','active'),(?,'fixture','regular','suspended'),(?,'fixture','regular','deleted')",live,guest,suspended,deleted);
      jdbc.update("UPDATE accounts SET deleted_at=now() WHERE id=?",deleted);
      var ids=List.of(live.toString(),guest.toString(),suspended.toString(),deleted.toString(),missing.toString());
      try(var reader=new ReportAudienceReader(source)) {
        var first=reader.classify(ids);assertThat(first).hasSize(5);assertThat(first.stream().map(account->account.getAccountId()).toList()).isEqualTo(ids);
        assertThat(first.stream().map(account->account.getState()).toList()).containsExactly(
            ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_LIVE,ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_LIVE,
            ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_LIVE,ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_DELETED,
            ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_MISSING);
        jdbc.update("DELETE FROM accounts WHERE id=?",live);
        assertThat(reader.classify(List.of(live.toString())).getFirst().getState()).isEqualTo(ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_MISSING);
        jdbc.update("UPDATE accounts SET deleted_at=NULL WHERE id=?",deleted);
        assertThatThrownBy(()->reader.classify(ids)).isInstanceOf(IllegalStateException.class);
        assertThatThrownBy(()->reader.classify(List.of(guest.toString(),guest.toString()))).isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(()->reader.classify(List.of(new UUID(0,0).toString()))).isInstanceOf(IllegalArgumentException.class);
        jdbc.execute("DROP TABLE accounts CASCADE");
        assertThatThrownBy(()->reader.classify(List.of(guest.toString()))).isInstanceOf(IllegalStateException.class);
      }
    }
  }
}
