package voice.backend.auth.moderationauthority;

import static org.assertj.core.api.Assertions.*;

import app.voice.automod.v1.SubjectState;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.utility.DockerImageName;

class AuthModerationStoreIntegrationTest {
  @Test void acknowledgementCapturesAcceptedPayloadAndLifecycleInOneTransaction() {
    String external=System.getenv("AUTH_AUTOMOD_TEST_JDBC_URL");
    PostgreSQLContainer<?> pg=null;
    try {
      String url=external,user=System.getenv("AUTH_AUTOMOD_TEST_USER"),password=System.getenv("AUTH_AUTOMOD_TEST_PASSWORD");
      if(url==null) {
        pg=new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-bookworm"));
        pg.start();url=pg.getJdbcUrl();user=pg.getUsername();password=pg.getPassword();
      }
      var source=new DriverManagerDataSource(url,user,password);
      Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
      var jdbc=new JdbcTemplate(source);
      var store=new AuthModerationStore(new NamedParameterJdbcTemplate(source),new DataSourceTransactionManager(source));
      UUID account=UUID.randomUUID();
      jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES (?,'fixture','regular','active')",account);
      UUID subject=UUID.fromString(store.resolve(account,null).getSubject().getSubjectId());
      var ban=store.reconcile(account,subject,2,true);
      assertThat(ban.getProtocolVersion()).isEqualTo(1);
      assertThat(ban.getAccountId()).isEqualTo(account.toString());
      assertThat(ban.getSubjectId()).isEqualTo(subject.toString());
      assertThat(ban.getRevision()).isEqualTo(2);
      assertThat(ban.getModerationBanned()).isTrue();
      assertThat(ban.getState()).isEqualTo(SubjectState.SUBJECT_STATE_LIVE);
      var staleClear=store.reconcile(account,subject,1,false);
      assertThat(staleClear).isEqualTo(ban);
      assertThat(store.reconcile(account,subject,2,true)).isEqualTo(ban);
      assertThatThrownBy(()->store.reconcile(account,subject,2,false)).isInstanceOf(RuntimeException.class);
      assertThat(jdbc.queryForObject("SELECT status FROM accounts WHERE id=?",String.class,account)).isEqualTo("suspended");
      var clear=store.reconcile(account,subject,3,false);
      assertThat(clear.getRevision()).isEqualTo(3);
      assertThat(clear.getModerationBanned()).isFalse();
      jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp() WHERE id=?",account);
      assertThat(store.reconcile(account,subject,4,true).getState()).isEqualTo(SubjectState.SUBJECT_STATE_DELETED);
      assertThatThrownBy(()->jdbc.update("UPDATE accounts SET moderation_purge_at=clock_timestamp()-interval '1 second' WHERE id=?",account))
          .isInstanceOf(RuntimeException.class);
      // A recovered authoritative terminal fence dominates the still-present
      // account mapping; a client cannot manufacture/shorten its deadline.
      jdbc.update("INSERT INTO moderation_terminal_subjects(subject_id) VALUES (?)",subject);
      assertThatThrownBy(()->store.reconcile(account,subject,5,false)).isInstanceOf(RuntimeException.class);
      store.purgeDue();
      assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_terminal_subjects WHERE subject_id=?",Integer.class,subject)).isEqualTo(1);
      assertThatThrownBy(()->store.resolve(account,subject)).isInstanceOf(RuntimeException.class);
      UUID overdue=UUID.randomUUID();
      jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES (?,'fixture','regular','active')",overdue);
      UUID overdueSubject=UUID.fromString(store.resolve(overdue,null).getSubject().getSubjectId());
      jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp()-interval '31 days' WHERE id=?",overdue);
      assertThatThrownBy(()->store.reconcile(overdue,overdueSubject,1,false)).isInstanceOf(RuntimeException.class);
      store.purgeDue();
      assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_terminal_subjects WHERE subject_id=?",Integer.class,overdueSubject)).isEqualTo(1);
    } finally {
      if(pg!=null)pg.close();
    }
  }
}
