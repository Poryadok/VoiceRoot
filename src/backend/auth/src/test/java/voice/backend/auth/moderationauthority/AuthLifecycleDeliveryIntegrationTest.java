package voice.backend.auth.moderationauthority;

import static org.assertj.core.api.Assertions.*;
import app.voice.automod.v1.*;
import com.google.protobuf.ByteString;
import io.grpc.Status;
import java.sql.DriverManager;
import java.util.Map;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.utility.DockerImageName;

class AuthLifecycleDeliveryIntegrationTest {
  static final Runnable CURRENT_AUTHORITY=()->{}; // Explicit fixture-only bootstrap authority.
  @Test void stableReceiptClearsOnlyItsExactRecipientIntent() throws Exception {
    try(var f=fixture()) {
      f.account();var delivery=f.first("moderation");
      assertThat(f.store.acknowledge(delivery,receipt(delivery),CURRENT_AUTHORITY)).isEqualTo(AuthLifecycleDeliveryStore.Outcome.ACKED);
      assertThat(f.store.pending(100)).extracting(AuthLifecycleDeliveryStore.Delivery::recipient).containsExactly("messaging");
      assertThat(f.jdbc.queryForObject("SELECT count(*) FROM moderation_lifecycle_current",Integer.class)).isEqualTo(1);
    }
  }
  @Test void networkCallHoldsNoTransactionAndOwnerTransitionSupersedesOldAck() throws Exception {
    try(var f=fixture()) {
      UUID account=f.account();boolean[] changed={false};
      var publisher=new AuthLifecyclePublisher(f.store,delivery->{
        assertThat(TransactionSynchronizationManager.isActualTransactionActive()).isFalse();
        if(!changed[0]) {f.jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp() WHERE id=?",account);changed[0]=true;}
        return receipt(delivery);
      },CURRENT_AUTHORITY);
      var result=publisher.runOnce(100);
      assertThat(result.acknowledged()).isZero();assertThat(result.superseded()).isEqualTo(2);
      assertThat(f.store.pending(100)).allSatisfy(d->{assertThat(d.fact().getLifecycleRevision()).isEqualTo(2);assertThat(d.fact().getState()).isEqualTo(SubjectState.SUBJECT_STATE_DELETED);});
      var converged=new AuthLifecyclePublisher(f.store,AuthLifecycleDeliveryIntegrationTest::receipt,CURRENT_AUTHORITY).runOnce(100);
      assertThat(converged.acknowledged()).isEqualTo(2);assertThat(f.store.pending(100)).isEmpty();
    }
  }
  @Test void peerAheadOrUnownedTerminalCannotRewriteOwnerOrClearIntent() throws Exception {
    try(var f=fixture()) {
      UUID account=f.account();var delivery=f.first("moderation");var stable=receipt(delivery);
      var ahead=stable.toBuilder().setLifecycleRevision(2).setCurrentFactSha256(ByteString.copyFrom(new byte[32])).build();
      assertThatThrownBy(()->f.store.acknowledge(delivery,ahead,CURRENT_AUTHORITY)).isInstanceOf(AuthLifecycleDeliveryStore.RestoreDivergence.class);
      var terminal=stable.toBuilder().setTerminal(true).setLifecycleRevision(0).clearCurrentFactSha256().build();
      assertThatThrownBy(()->f.store.acknowledge(delivery,terminal,CURRENT_AUTHORITY)).isInstanceOf(AuthLifecycleDeliveryStore.RestoreDivergence.class);
      var changed=stable.toBuilder().setCurrentFactSha256(ByteString.copyFrom(new byte[32])).build();
      assertThatThrownBy(()->f.store.acknowledge(delivery,changed,CURRENT_AUTHORITY)).isInstanceOf(AuthLifecycleDeliveryStore.RestoreDivergence.class);
      assertThat(f.store.pending(100)).hasSize(2);
      assertThat(f.jdbc.queryForObject("SELECT moderation_lifecycle_revision FROM accounts WHERE id=?",Long.class,account)).isEqualTo(1);
      assertThat(f.jdbc.queryForObject("SELECT count(*) FROM moderation_terminal_subjects",Integer.class)).isZero();
    }
  }
  @Test void newerPeerReceiptIsComparedToCurrentOwnerButNeverConsumesNewIntent() throws Exception {
    try(var f=fixture()) {
      UUID account=f.account();var old=f.first("moderation");
      f.jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp() WHERE id=?",account);
      var current=f.first("moderation");
      var response=receipt(current).toBuilder().setRequestedEventId(old.fact().getEventId()).build();
      assertThat(f.store.acknowledge(old,response,CURRENT_AUTHORITY)).isEqualTo(AuthLifecycleDeliveryStore.Outcome.SUPERSEDED);
      assertThat(f.first("moderation").fact()).isEqualTo(current.fact());
      assertThatThrownBy(()->f.store.acknowledge(old,response.toBuilder().setCurrentFactSha256(ByteString.copyFrom(new byte[32])).build(),CURRENT_AUTHORITY)).isInstanceOf(AuthLifecycleDeliveryStore.RestoreDivergence.class);
    }
  }
  @Test void terminalAckRequiresOwnPermanentLedgerAndPreservesOtherRecipient() throws Exception {
    try(var f=fixture()) {
      UUID account=f.account();f.jdbc.update("DELETE FROM accounts WHERE id=?",account);
      var terminal=f.first("moderation");assertThat(terminal.fact().getState()).isEqualTo(SubjectState.SUBJECT_STATE_PURGED);
      assertThat(f.store.acknowledge(terminal,receipt(terminal),CURRENT_AUTHORITY)).isEqualTo(AuthLifecycleDeliveryStore.Outcome.ACKED);
      assertThat(f.store.pending(100)).extracting(AuthLifecycleDeliveryStore.Delivery::recipient).containsExactly("messaging");
      assertThat(f.jdbc.queryForObject("SELECT count(*) FROM moderation_terminal_subjects",Integer.class)).isEqualTo(1);
    }
  }
  @Test void generationClosureBeforeCommitRollsBackAndUnavailablePeerDoesNotStarveOtherRecipient() throws Exception {
    try(var f=fixture()) {
      f.account();var delivery=f.first("moderation");int[] checks={0};
      Runnable closing=()->{if(++checks[0]==3)throw new IllegalStateException("boot generation closed");};
      assertThatThrownBy(()->f.store.acknowledge(delivery,receipt(delivery),closing)).hasMessageContaining("generation closed");
      assertThat(f.store.pending(100)).hasSize(2);
      var publisher=new AuthLifecyclePublisher(f.store,d->{if(d.recipient().equals("messaging"))throw Status.UNAVAILABLE.asRuntimeException();return receipt(d);},CURRENT_AUTHORITY);
      var result=publisher.runOnce(100);assertThat(result.acknowledged()).isEqualTo(1);assertThat(result.retryable()).isEqualTo(1);
      assertThat(f.store.pending(100)).extracting(AuthLifecycleDeliveryStore.Delivery::recipient).containsExactly("messaging");
    }
  }
  @Test void currentOwnerIsRereadAfterAConcurrentTransitionLockSettles() throws Exception {
    try(var f=fixture();var executor=java.util.concurrent.Executors.newSingleThreadExecutor()) {
      UUID account=f.account();var old=f.first("moderation");
      try(var writer=f.source.getConnection()) {
        writer.setAutoCommit(false);
        try(var statement=writer.prepareStatement("UPDATE accounts SET deleted_at=clock_timestamp() WHERE id=?")){statement.setObject(1,account);statement.executeUpdate();}
        var future=executor.submit(()->f.store.acknowledge(old,receipt(old),CURRENT_AUTHORITY));
        boolean waiting=false;long until=System.nanoTime()+java.util.concurrent.TimeUnit.SECONDS.toNanos(5);
        while(!waiting && System.nanoTime()<until) {
          waiting=Boolean.TRUE.equals(f.jdbc.queryForObject("SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FROM accounts WHERE moderation_subject_id=%')",Boolean.class));
          if(!waiting)Thread.sleep(10);
        }
        assertThat(waiting).as("ACK must acquire the actual owner row before deciding currentness").isTrue();
        writer.commit();
        assertThat(future.get(5,java.util.concurrent.TimeUnit.SECONDS)).isEqualTo(AuthLifecycleDeliveryStore.Outcome.SUPERSEDED);
        assertThat(f.first("moderation").fact().getLifecycleRevision()).isEqualTo(2);
      }
    }
  }
  static ApplyLifecycleFactResponse receipt(AuthLifecycleDeliveryStore.Delivery delivery) {
    var fact=delivery.fact();var response=ApplyLifecycleFactResponse.newBuilder().setProtocolVersion(1)
        .setAuthorityGeneration(fact.getAuthorityGeneration()).setSubjectId(fact.getSubjectId()).setRequestedEventId(fact.getEventId());
    if(fact.getState()==SubjectState.SUBJECT_STATE_PURGED)return response.setTerminal(true).build();
    return response.setLifecycleRevision(fact.getLifecycleRevision()).setCurrentFactSha256(ByteString.copyFrom(AuthLifecycleCodec.hash(fact))).build();
  }
  static Fixture fixture() throws Exception {
    String base=System.getenv("AUTH_AUTOMOD_TEST_JDBC_URL"),user=System.getenv("AUTH_AUTOMOD_TEST_USER"),password=System.getenv("AUTH_AUTOMOD_TEST_PASSWORD");
    PostgreSQLContainer<?> pg=null;
    if(base==null){pg=new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-bookworm"));pg.start();base=pg.getJdbcUrl();user=pg.getUsername();password=pg.getPassword();}
    var admin=DriverManager.getConnection(base,user,password);String database="delivery_"+UUID.randomUUID().toString().replace("-","");
    try(var statement=admin.createStatement()){statement.execute("CREATE DATABASE \""+database+"\"");}
    int query=base.indexOf('?');String suffix=query<0?"":base.substring(query),plain=query<0?base:base.substring(0,query);
    var source=new DriverManagerDataSource(plain.substring(0,plain.lastIndexOf('/')+1)+database+suffix,user,password);
    try {Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();return new Fixture(admin,database,pg,source);}
    catch(Exception error){try(var statement=admin.createStatement()){statement.execute("DROP DATABASE \""+database+"\" WITH (FORCE)");}admin.close();if(pg!=null)pg.close();throw error;}
  }
  static final class Fixture implements AutoCloseable {
    final java.sql.Connection admin;final String database;final PostgreSQLContainer<?> pg;final DriverManagerDataSource source;final JdbcTemplate jdbc;final AuthLifecycleDeliveryStore store;
    Fixture(java.sql.Connection admin,String database,PostgreSQLContainer<?> pg,DriverManagerDataSource source){this.admin=admin;this.database=database;this.pg=pg;this.source=source;this.jdbc=new JdbcTemplate(source);this.store=new AuthLifecycleDeliveryStore(new NamedParameterJdbcTemplate(source),new DataSourceTransactionManager(source));}
    UUID account(){UUID id=UUID.randomUUID();jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES (?,'fixture','regular','active')",id);return id;}
    AuthLifecycleDeliveryStore.Delivery first(String recipient){return store.pending(100).stream().filter(d->d.recipient().equals(recipient)).findFirst().orElseThrow();}
    @Override public void close()throws Exception{try{try(var statement=admin.createStatement()){statement.execute("DROP DATABASE \""+database+"\" WITH (FORCE)");}}finally{admin.close();if(pg!=null)pg.close();}}
  }
}
