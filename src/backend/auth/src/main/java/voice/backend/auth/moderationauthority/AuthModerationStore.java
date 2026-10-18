package voice.backend.auth.moderationauthority;

import app.voice.automod.v1.*;
import com.google.protobuf.Timestamp;
import io.grpc.Status;
import java.time.Instant;
import java.util.*;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Auth-owned live account/opaque subject projection; never a User profile lookup. */
public final class AuthModerationStore {
  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  public AuthModerationStore(NamedParameterJdbcTemplate jdbc, PlatformTransactionManager manager) {
    this.jdbc=Objects.requireNonNull(jdbc);this.transactions=new TransactionTemplate(manager);
  }
  public ResolveModerationSubjectResponse resolve(UUID account,UUID expectedSubject) {
    var rows=jdbc.query("""
        SELECT moderation_subject_id,moderation_lifecycle_revision,moderation_deletion_cycle_id,
               moderation_purge_at,deleted_at,status,session_epoch,
               (moderation_purge_at <= clock_timestamp() OR EXISTS(SELECT 1 FROM moderation_terminal_subjects t WHERE t.subject_id=accounts.moderation_subject_id)) AS terminal
        FROM accounts WHERE id=:account
        """,Map.of("account",account),(rs,row)-> {
      UUID subject=rs.getObject("moderation_subject_id",UUID.class);
      if(subject==null || rs.getBoolean("terminal")) throw Status.FAILED_PRECONDITION.withDescription("terminal moderation subject").asRuntimeException();
      if(expectedSubject!=null && !subject.equals(expectedSubject)) throw Status.FAILED_PRECONDITION.withDescription("moderation subject mismatch").asRuntimeException();
      var binding=SubjectBinding.newBuilder().setProtocolVersion(1).setAccountId(account.toString()).setSubjectId(subject.toString())
          .setLifecycleRevision(rs.getLong("moderation_lifecycle_revision"))
          .setState(rs.getTimestamp("deleted_at")==null?SubjectState.SUBJECT_STATE_LIVE:SubjectState.SUBJECT_STATE_DELETED);
      UUID cycle=rs.getObject("moderation_deletion_cycle_id",UUID.class);
      if(cycle!=null)binding.setDeletionCycleId(cycle.toString());
      if(rs.getTimestamp("moderation_purge_at")!=null)binding.setPurgeAt(timestamp(rs.getTimestamp("moderation_purge_at").toInstant()));
      return ResolveModerationSubjectResponse.newBuilder().setSubject(binding)
          .setAuthRestricted(!"active".equals(rs.getString("status")) || rs.getTimestamp("deleted_at")!=null)
          .setSessionEpoch(rs.getLong("session_epoch")).build();
    });
    if(rows.size()!=1)throw Status.NOT_FOUND.withDescription("moderation subject unavailable").asRuntimeException();
    return rows.getFirst();
  }
  public ListTerminalModerationSubjectsResponse terminalSnapshot() {
    var subjects=jdbc.query("""
        SELECT subject_id FROM moderation_terminal_subjects
        UNION SELECT moderation_subject_id FROM accounts WHERE moderation_subject_id IS NOT NULL AND moderation_purge_at <= clock_timestamp()
        ORDER BY subject_id
        """,Map.of(),(rs,row)->rs.getObject(1,UUID.class).toString());
    return ListTerminalModerationSubjectsResponse.newBuilder().addAllSubjectIds(subjects).setComplete(true).build();
  }
  public ReconcileModerationRestrictionResponse reconcile(UUID account,UUID subject,long revision,boolean banned) {
    if(revision<=0)throw Status.INVALID_ARGUMENT.withDescription("positive moderation revision required").asRuntimeException();
    return transactions.execute(transaction->{
      var parameters=Map.of("account",account,"subject",subject,"revision",revision,"banned",banned);
      Long accepted=jdbc.queryForObject("SELECT auth_reconcile_moderation(:account,:subject,:revision,:banned)",parameters,Long.class);
      // The SQL function's account lock remains held until this exact accepted
      // payload and lifecycle have been captured. Never echo the stale request.
      Boolean acceptedBanned=jdbc.queryForObject("SELECT desired_banned FROM moderation_account_restrictions WHERE account_id=:account AND subject_id=:subject",parameters,Boolean.class);
      return ReconcileModerationRestrictionResponse.newBuilder().setProtocolVersion(1)
          .setAccountId(account.toString()).setSubjectId(subject.toString())
          .setRevision(Objects.requireNonNull(accepted)).setModerationBanned(Objects.requireNonNull(acceptedBanned))
          .setState(resolve(account,subject).getSubject().getState()).build();
    });
  }
  /** No ACK or consumer availability can extend the canonical Auth deletion deadline. */
  public void purgeDue() {
    transactions.executeWithoutResult(status->{
      var due=jdbc.query("""
          SELECT id,moderation_subject_id FROM accounts WHERE moderation_subject_id IS NOT NULL
            AND (moderation_purge_at <= clock_timestamp() OR EXISTS(SELECT 1 FROM moderation_terminal_subjects t WHERE t.subject_id=accounts.moderation_subject_id)) ORDER BY moderation_purge_at,id LIMIT 100 FOR UPDATE SKIP LOCKED
          """,Map.of(),(rs,row)->Map.entry(rs.getObject(1,UUID.class),rs.getObject(2,UUID.class)));
      for(var entry:due) {
        jdbc.update("INSERT INTO moderation_terminal_subjects(subject_id) VALUES(:subject) ON CONFLICT DO NOTHING",Map.of("subject",entry.getValue()));
        jdbc.update("DELETE FROM moderation_account_restrictions WHERE account_id=:account",Map.of("account",entry.getKey()));
        jdbc.update("UPDATE accounts SET moderation_subject_id=NULL WHERE id=:account",Map.of("account",entry.getKey()));
      }
    });
  }
  private static Timestamp timestamp(Instant instant){return Timestamp.newBuilder().setSeconds(instant.getEpochSecond()).setNanos(instant.getNano()).build();}
}
