package voice.backend.auth.moderationauthority;

import static org.assertj.core.api.Assertions.*;

import java.sql.DriverManager;
import java.sql.Timestamp;
import java.util.Properties;
import java.util.UUID;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.utility.DockerImageName;

class AuthModerationTerminalRestoreRaceTest {
  @Test void terminalSnapshotSettlesLockedRestoreBeforeIssuingAnyPermanentFence() throws Exception {
    String base=System.getenv("AUTH_AUTOMOD_TEST_JDBC_URL"),user=System.getenv("AUTH_AUTOMOD_TEST_USER"),password=System.getenv("AUTH_AUTOMOD_TEST_PASSWORD");
    PostgreSQLContainer<?> pg=null;
    if(base==null){pg=new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-bookworm"));pg.start();base=pg.getJdbcUrl();user=pg.getUsername();password=pg.getPassword();}
    String database="terminal_race_"+UUID.randomUUID().toString().replace("-","");
    var worker=Executors.newSingleThreadExecutor();
    try(var admin=DriverManager.getConnection(base,user,password)){
      try(var statement=admin.createStatement()){statement.execute("CREATE DATABASE \""+database+"\"");}
      int query=base.indexOf('?');String suffix=query<0?"":base.substring(query),plain=query<0?base:base.substring(0,query);
      String url=plain.substring(0,plain.lastIndexOf('/')+1)+database+suffix;
      try {
        var source=new DriverManagerDataSource(url,user,password);
        Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
        var jdbc=new JdbcTemplate(source);UUID account=UUID.randomUUID();
        jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES (?,'fixture','regular','active')",account);
        UUID subject=jdbc.queryForObject("SELECT moderation_subject_id FROM accounts WHERE id=?",UUID.class,account);
        Timestamp deadline=jdbc.queryForObject("UPDATE accounts SET deleted_at=clock_timestamp()-interval '30 days'+interval '3 seconds' WHERE id=? RETURNING moderation_purge_at",Timestamp.class,account);
        String application="terminal-snapshot-"+UUID.randomUUID();
        var snapshotSource=new DriverManagerDataSource(url,user,password);var properties=new Properties();properties.setProperty("ApplicationName",application);snapshotSource.setConnectionProperties(properties);
        var store=new AuthModerationStore(new NamedParameterJdbcTemplate(snapshotSource),new DataSourceTransactionManager(snapshotSource));
        try(var restore=source.getConnection()) {
          restore.setAutoCommit(false);
          try(var statement=restore.prepareStatement("UPDATE accounts SET deleted_at=NULL,status='active' WHERE id=?")){statement.setObject(1,account);assertThat(statement.executeUpdate()).isEqualTo(1);}
          long until=System.nanoTime()+TimeUnit.SECONDS.toNanos(6);
          while(!Boolean.TRUE.equals(jdbc.queryForObject("SELECT clock_timestamp()>=?",Boolean.class,deadline)) && System.nanoTime()<until)Thread.sleep(10);
          assertThat(jdbc.queryForObject("SELECT clock_timestamp()>=?",Boolean.class,deadline)).isTrue();
          var snapshot=worker.submit(store::terminalSnapshot);
          until=System.nanoTime()+TimeUnit.SECONDS.toNanos(3);boolean blocked=false;
          while(!snapshot.isDone() && System.nanoTime()<until){blocked=Boolean.TRUE.equals(jdbc.queryForObject("SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=? AND wait_event_type='Lock')",Boolean.class,application));if(blocked)break;Thread.sleep(10);}
          if(snapshot.isDone())assertThat(snapshot.get(2,TimeUnit.SECONDS).getSubjectIdsList()).as("stale RR state is not irreversible authority").doesNotContain(subject.toString());
          assertThat(blocked).as("complete terminal view must settle the owner account lock").isTrue();
          restore.commit();
          assertThat(snapshot.get(10,TimeUnit.SECONDS).getSubjectIdsList()).doesNotContain(subject.toString());
          assertThat(jdbc.queryForObject("SELECT count(*) FROM moderation_terminal_subjects WHERE subject_id=?",Integer.class,subject)).isZero();
          assertThat(jdbc.queryForObject("SELECT deleted_at IS NULL AND moderation_purge_at IS NULL FROM accounts WHERE id=?",Boolean.class,account)).isTrue();
        }
      } finally {try(var statement=admin.createStatement()){statement.execute("DROP DATABASE \""+database+"\" WITH (FORCE)");}}
    } finally {worker.shutdownNow();worker.awaitTermination(5,TimeUnit.SECONDS);if(pg!=null)pg.close();}
  }
}
