package voice.backend.auth.moderationauthority;

import app.voice.automod.v1.*;
import com.google.protobuf.Timestamp;
import java.security.MessageDigest;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.UUID;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Private recipient intents; transport is deliberately outside these transactions. */
public final class AuthLifecycleDeliveryStore {
  public enum Outcome { ACKED, SUPERSEDED }
  public static final class RestoreDivergence extends IllegalStateException {
    RestoreDivergence(String message){super(message);}
    RestoreDivergence(String message,Throwable cause){super(message,cause);}
  }
  public record Delivery(String recipient,AccountLifecycleFact fact) {
    public Delivery {
      if(!"messaging".equals(recipient) && !"moderation".equals(recipient))throw new IllegalArgumentException("invalid lifecycle recipient");
      AuthLifecycleCodec.canonical(fact);
    }
  }
  private record Owner(long revision,String state,UUID cycle,Instant deadline) {}
  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  public AuthLifecycleDeliveryStore(NamedParameterJdbcTemplate jdbc,PlatformTransactionManager manager) {
    this.jdbc=Objects.requireNonNull(jdbc);this.transactions=new TransactionTemplate(Objects.requireNonNull(manager));
    this.transactions.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);this.transactions.setTimeout(5);
  }
  public List<Delivery> pending(int limit) {
    standalone();if(limit<1 || limit>1000)throw new IllegalArgumentException("bounded lifecycle batch required");
    return List.copyOf(jdbc.query("""
        SELECT * FROM (
          SELECT false AS terminal,d.recipient,d.requested_at,d.event_id AS requested_event_id,
                 c.subject_id,c.event_id,c.authority_generation,c.lifecycle_revision,c.state,c.deletion_cycle_id,c.purge_at,
                 g.generation AS current_generation
          FROM moderation_lifecycle_delivery d JOIN moderation_lifecycle_current c USING(subject_id)
          CROSS JOIN moderation_authority_generation g WHERE g.singleton
          UNION ALL
          SELECT true,d.recipient,d.requested_at,d.event_id,t.subject_id,t.event_id,d.authority_generation,
                 0::bigint,'PURGED',NULL::uuid,NULL::timestamptz,g.generation
          FROM moderation_terminal_delivery d JOIN moderation_terminal_subjects t USING(subject_id)
          CROSS JOIN moderation_authority_generation g WHERE g.singleton
        ) current_intents ORDER BY terminal DESC,requested_at,subject_id,recipient LIMIT :limit
        """,Map.of("limit",limit),(rs,row)-> {
      if(!rs.getObject("requested_event_id",UUID.class).equals(rs.getObject("event_id",UUID.class))
          || !rs.getObject("authority_generation",UUID.class).equals(rs.getObject("current_generation",UUID.class)))
        throw new RestoreDivergence("lifecycle intent differs from current owner projection");
      return new Delivery(rs.getString("recipient"),fact(rs));
    }));
  }
  public Outcome acknowledge(Delivery requested,ApplyLifecycleFactResponse receipt,Runnable authorityCheck) {
    standalone();Objects.requireNonNull(requested);Objects.requireNonNull(authorityCheck);
    try {AuthLifecycleCodec.validateReceipt(requested.fact(),receipt);}
    catch(IllegalArgumentException invalid){throw new RestoreDivergence("invalid lifecycle receipt",invalid);}
    authorityCheck.run();
    return transactions.execute(status->{
      jdbc.getJdbcTemplate().execute("SET LOCAL lock_timeout='2s'");
      jdbc.getJdbcTemplate().execute("SET LOCAL statement_timeout='3s'");
      var parameters=Map.of("subject",UUID.fromString(requested.fact().getSubjectId()));
      // This owner lock precedes every projection/intent lock. A restore/delete
      // which committed during the network call is observed AFTER this lock.
      var owners=jdbc.query("""
          SELECT moderation_lifecycle_revision,deleted_at,moderation_deletion_cycle_id,moderation_purge_at
          FROM accounts WHERE moderation_subject_id=:subject FOR UPDATE
          """,parameters,(rs,row)->new Owner(rs.getLong(1),rs.getTimestamp(2)==null?"LIVE":"DELETED",
              rs.getObject(3,UUID.class),rs.getTimestamp(4)==null?null:rs.getTimestamp(4).toInstant()));
      var terminals=jdbc.query("SELECT event_id FROM moderation_terminal_subjects WHERE subject_id=:subject FOR SHARE",parameters,(rs,row)->rs.getObject(1,UUID.class));
      UUID generation=jdbc.queryForObject("SELECT generation FROM moderation_authority_generation WHERE singleton",Map.of(),UUID.class);
      if(!Objects.requireNonNull(generation).toString().equals(requested.fact().getAuthorityGeneration()))
        throw new RestoreDivergence("lifecycle authority generation changed");
      authorityCheck.run();
      Outcome outcome=compareAndClear(requested,receipt,owners,terminals,generation);
      authorityCheck.run(); // Same captured boot check; failure rolls the CAS back.
      return outcome;
    });
  }
  private Outcome compareAndClear(Delivery requested,ApplyLifecycleFactResponse receipt,List<Owner> owners,List<UUID> terminals,UUID generation) {
    var fact=requested.fact();UUID subject=UUID.fromString(fact.getSubjectId());
    var identity=Map.of("subject",subject,"recipient",requested.recipient(),"event",UUID.fromString(fact.getEventId()),"generation",generation);
    if(receipt.getTerminal()) {
      if(terminals.size()!=1)throw new RestoreDivergence("peer terminal receipt has no permanent owner authority");
      if(fact.getState()!=SubjectState.SUBJECT_STATE_PURGED)return Outcome.SUPERSEDED;
      if(!terminals.getFirst().toString().equals(fact.getEventId()))throw new RestoreDivergence("terminal event identity changed");
      int changed=jdbc.update("DELETE FROM moderation_terminal_delivery WHERE subject_id=:subject AND recipient=:recipient AND event_id=:event AND authority_generation=:generation",identity);
      return changed==1?Outcome.ACKED:Outcome.SUPERSEDED;
    }
    if(!terminals.isEmpty())return Outcome.SUPERSEDED; // Never clear the newer terminal intent with an old nonterminal reply.
    if(owners.size()!=1)throw new RestoreDivergence("nonterminal lifecycle owner is unavailable");
    var rows=jdbc.query("SELECT * FROM moderation_lifecycle_current WHERE subject_id=:subject FOR UPDATE",Map.of("subject",subject),(rs,row)->fact(rs));
    if(rows.size()!=1)throw new RestoreDivergence("current owner lifecycle projection is unavailable");
    var current=rows.getFirst();var owner=owners.getFirst();
    UUID cycle=current.hasDeletionCycleId()?UUID.fromString(current.getDeletionCycleId()):null;
    Instant deadline=current.hasPurgeAt()?Instant.ofEpochSecond(current.getPurgeAt().getSeconds(),current.getPurgeAt().getNanos()):null;
    if(owner.revision()!=current.getLifecycleRevision() || !owner.state().equals(current.getState()==SubjectState.SUBJECT_STATE_LIVE?"LIVE":"DELETED")
        || !Objects.equals(owner.cycle(),cycle) || !Objects.equals(owner.deadline(),deadline)
        || !current.getAuthorityGeneration().equals(generation.toString()))throw new RestoreDivergence("lifecycle projection differs from locked owner");
    if(receipt.getLifecycleRevision()>current.getLifecycleRevision() || fact.getLifecycleRevision()>current.getLifecycleRevision())
      throw new RestoreDivergence("peer or requested lifecycle is ahead of current owner");
    if(receipt.getLifecycleRevision()==current.getLifecycleRevision()
        && !MessageDigest.isEqual(AuthLifecycleCodec.hash(current),receipt.getCurrentFactSha256().toByteArray()))
      throw new RestoreDivergence("peer hash differs from current owner lifecycle");
    if(current.getLifecycleRevision()!=fact.getLifecycleRevision())return Outcome.SUPERSEDED;
    if(!current.equals(fact))throw new RestoreDivergence("same-revision owner lifecycle identity changed");
    int changed=jdbc.update("DELETE FROM moderation_lifecycle_delivery WHERE subject_id=:subject AND recipient=:recipient AND event_id=:event",identity);
    return changed==1?Outcome.ACKED:Outcome.SUPERSEDED;
  }
  private static AccountLifecycleFact fact(ResultSet rs)throws SQLException {
    var fact=AccountLifecycleFact.newBuilder().setProtocolVersion(1).setSubjectId(rs.getObject("subject_id",UUID.class).toString())
        .setEventId(rs.getObject("event_id",UUID.class).toString()).setAuthorityGeneration(rs.getObject("authority_generation",UUID.class).toString())
        .setLifecycleRevision(rs.getLong("lifecycle_revision"));
    fact.setState(switch(rs.getString("state")){case "LIVE"->SubjectState.SUBJECT_STATE_LIVE;case "DELETED"->SubjectState.SUBJECT_STATE_DELETED;case "PURGED"->SubjectState.SUBJECT_STATE_PURGED;default->throw new RestoreDivergence("invalid owner lifecycle state");});
    UUID cycle=rs.getObject("deletion_cycle_id",UUID.class);if(cycle!=null)fact.setDeletionCycleId(cycle.toString());
    var time=rs.getTimestamp("purge_at");if(time!=null){Instant at=time.toInstant();fact.setPurgeAt(Timestamp.newBuilder().setSeconds(at.getEpochSecond()).setNanos(at.getNano()));}
    var value=fact.build();AuthLifecycleCodec.canonical(value);return value;
  }
  private static void standalone(){if(TransactionSynchronizationManager.isActualTransactionActive())throw new IllegalStateException("lifecycle delivery requires a standalone owner transaction");}
}
