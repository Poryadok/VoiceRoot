package voice.backend.auth.moderationauthority;

import static org.assertj.core.api.Assertions.*;

import java.sql.DriverManager;
import java.util.Map;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.utility.DockerImageName;

class AuthModerationLifecycleProducerIntegrationTest {
  @Test void everyCommittedOwnerLifecycleHasAnAtomicOpaqueProjectionAndBothRecipientIntents() throws Exception {
    String base=System.getenv("AUTH_AUTOMOD_TEST_JDBC_URL"),user=System.getenv("AUTH_AUTOMOD_TEST_USER"),password=System.getenv("AUTH_AUTOMOD_TEST_PASSWORD");
    PostgreSQLContainer<?> pg=null;
    if(base==null){pg=new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-bookworm"));pg.start();base=pg.getJdbcUrl();user=pg.getUsername();password=pg.getPassword();}
    String database="lifecycle_"+UUID.randomUUID().toString().replace("-","");
    try(var admin=DriverManager.getConnection(base,user,password)) {
      try(var statement=admin.createStatement()){statement.execute("CREATE DATABASE \""+database+"\"");}
      int query=base.indexOf('?');String suffix=query<0?"":base.substring(query),plain=query<0?base:base.substring(0,query);
      String url=plain.substring(0,plain.lastIndexOf('/')+1)+database+suffix;
      try {
        var source=new DriverManagerDataSource(url,user,password);
        Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
        var jdbc=new JdbcTemplate(source);
        UUID rollbackAccount=UUID.randomUUID();
        try(var tx=source.getConnection()) {
          tx.setAutoCommit(false);
          try(var statement=tx.prepareStatement("INSERT INTO accounts(id,password_hash,type,status) VALUES (?,'fixture','regular','active')")) {
            statement.setObject(1,rollbackAccount);statement.executeUpdate();
          }
          tx.rollback();
        }
        assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_lifecycle_current",Integer.class)).isZero();
        assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_lifecycle_delivery",Integer.class)).isZero();

        UUID account=UUID.randomUUID();
        jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES (?,'fixture','regular','active')",account);
        UUID subject=jdbc.queryForObject("SELECT moderation_subject_id FROM accounts WHERE id=?",UUID.class,account);
        Map<String,Object> live=jdbc.queryForMap("SELECT * FROM moderation_lifecycle_current WHERE subject_id=?",subject);
        assertThat(live).containsEntry("state","LIVE").containsEntry("lifecycle_revision",1L)
            .containsEntry("deletion_cycle_id",null).containsEntry("purge_at",null);
        assertIntents(jdbc,subject,live.get("event_id"));
        assertThat(jdbc.queryForObject("SELECT authority_generation=(SELECT generation FROM moderation_authority_generation WHERE singleton) FROM moderation_lifecycle_current WHERE subject_id=?",Boolean.class,subject)).isTrue();
        jdbc.update("UPDATE accounts SET session_epoch=session_epoch+1 WHERE id=?",account);
        assertThat(jdbc.queryForObject("SELECT event_id FROM moderation_lifecycle_current WHERE subject_id=?",UUID.class,subject)).isEqualTo(live.get("event_id"));

        jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp() WHERE id=?",account);
        Map<String,Object> deleted=jdbc.queryForMap("SELECT * FROM moderation_lifecycle_current WHERE subject_id=?",subject);
        assertThat(deleted).containsEntry("state","DELETED").containsEntry("lifecycle_revision",2L);
        assertThat(deleted.get("event_id")).isNotEqualTo(live.get("event_id"));
        assertThat(deleted.get("deletion_cycle_id")).isNotNull();assertThat(deleted.get("purge_at")).isNotNull();
        assertThat(jdbc.queryForObject("SELECT c.deletion_cycle_id=a.moderation_deletion_cycle_id AND c.purge_at=a.moderation_purge_at FROM moderation_lifecycle_current c JOIN accounts a ON a.moderation_subject_id=c.subject_id WHERE a.id=?",Boolean.class,account)).isTrue();
        assertIntents(jdbc,subject,deleted.get("event_id"));
        assertThatThrownBy(()->jdbc.update("UPDATE accounts SET deleted_at=deleted_at+interval '1 second' WHERE id=?",account)).isInstanceOf(RuntimeException.class);
        assertThat(jdbc.queryForMap("SELECT * FROM moderation_lifecycle_current WHERE subject_id=?",subject)).isEqualTo(deleted);

        // An actual producer write failure must roll back the owner restore too.
        jdbc.execute("CREATE FUNCTION reject_lifecycle_intent() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected producer write failure'; END $$");
        jdbc.execute("CREATE TRIGGER reject_lifecycle_intent BEFORE INSERT OR UPDATE ON moderation_lifecycle_delivery FOR EACH ROW EXECUTE FUNCTION reject_lifecycle_intent()");
        assertThatThrownBy(()->jdbc.update("UPDATE accounts SET deleted_at=NULL,status='active' WHERE id=?",account)).isInstanceOf(RuntimeException.class);
        assertThat(jdbc.queryForMap("SELECT * FROM moderation_lifecycle_current WHERE subject_id=?",subject)).isEqualTo(deleted);
        assertThat(jdbc.queryForObject("SELECT deleted_at IS NOT NULL AND moderation_lifecycle_revision=2 FROM accounts WHERE id=?",Boolean.class,account)).isTrue();
        jdbc.execute("DROP TRIGGER reject_lifecycle_intent ON moderation_lifecycle_delivery");jdbc.execute("DROP FUNCTION reject_lifecycle_intent()");
        jdbc.update("UPDATE accounts SET deleted_at=NULL,status='active' WHERE id=?",account);
        Map<String,Object> restored=jdbc.queryForMap("SELECT * FROM moderation_lifecycle_current WHERE subject_id=?",subject);
        assertThat(restored).containsEntry("state","LIVE").containsEntry("lifecycle_revision",3L)
            .containsEntry("deletion_cycle_id",null).containsEntry("purge_at",null);
        assertIntents(jdbc,subject,restored.get("event_id"));
        jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp() WHERE id=?",account);
        assertThat(jdbc.queryForObject("SELECT deletion_cycle_id FROM moderation_lifecycle_current WHERE subject_id=?",UUID.class,subject)).isNotEqualTo(deleted.get("deletion_cycle_id"));

        // Direct physical owner removal also leaves positive terminal delivery.
        jdbc.update("DELETE FROM accounts WHERE id=?",account);
        assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_lifecycle_current WHERE subject_id=?",Integer.class,subject)).isZero();
        assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_lifecycle_delivery WHERE subject_id=?",Integer.class,subject)).isZero();
        assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_terminal_delivery WHERE subject_id=?",Integer.class,subject)).isEqualTo(2);
        assertThat(jdbc.queryForList("SELECT recipient FROM moderation_terminal_delivery WHERE subject_id=? ORDER BY recipient",String.class,subject)).containsExactly("messaging","moderation");
        assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_terminal_subjects WHERE subject_id=?",Integer.class,subject)).isEqualTo(1);
        String terminal=jdbc.queryForObject("SELECT row_to_json(t)::text FROM moderation_terminal_subjects t WHERE subject_id=?",String.class,subject);
        assertThat(terminal).doesNotContain(account.toString());
        assertThat(jdbc.queryForList("SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name IN ('moderation_lifecycle_current','moderation_lifecycle_delivery','moderation_terminal_delivery')",String.class))
            .doesNotContain("account_id","profile_id","message_id","content","original_hash");
      } finally {try(var statement=admin.createStatement()){statement.execute("DROP DATABASE \""+database+"\" WITH (FORCE)");}}
    } finally {if(pg!=null)pg.close();}
  }
  private static void assertIntents(JdbcTemplate jdbc,UUID subject,Object event) {
    assertThat(jdbc.queryForList("SELECT recipient FROM moderation_lifecycle_delivery WHERE subject_id=? AND event_id=? ORDER BY recipient",String.class,subject,event)).containsExactly("messaging","moderation");
    assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_lifecycle_delivery WHERE subject_id=?",Integer.class,subject)).isEqualTo(2);
  }
}
