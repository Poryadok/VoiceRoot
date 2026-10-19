package voice.backend.auth.moderationauthority;

import app.voice.automod.v1.*;
import com.google.protobuf.Timestamp;
import io.grpc.Status;
import java.time.Instant;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.*;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/** Auth-owned live account/opaque subject projection; never a User profile lookup. */
public final class AuthModerationStore {
  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final TransactionTemplate snapshots;
  private final TransactionTemplate terminalization;
  public AuthModerationStore(NamedParameterJdbcTemplate jdbc, PlatformTransactionManager manager) {
    this.jdbc=Objects.requireNonNull(jdbc);this.transactions=new TransactionTemplate(manager);
    this.transactions.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);
    this.snapshots=new TransactionTemplate(manager);
    this.snapshots.setIsolationLevel(TransactionDefinition.ISOLATION_REPEATABLE_READ);
    this.snapshots.setReadOnly(true);
    this.terminalization=new TransactionTemplate(manager);
    this.terminalization.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);
    this.terminalization.setTimeout(5);
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
      return accountBinding(rs,account);
    });
    if(rows.size()!=1)throw Status.NOT_FOUND.withDescription("moderation subject unavailable").asRuntimeException();
    return rows.getFirst();
  }
  public ListTerminalModerationSubjectsResponse terminalSnapshot() {
    if(TransactionSynchronizationManager.isActualTransactionActive())throw Status.FAILED_PRECONDITION.withDescription("terminal view requires a standalone owner transaction").asRuntimeException();
    Instant cut=terminalization.execute(transaction->{
      Instant through=Objects.requireNonNull(jdbc.queryForObject("SELECT clock_timestamp()",Map.of(),java.sql.Timestamp.class)).toInstant();
      // Never SKIP LOCKED here: a hidden winning restore must settle before
      // the owner can certify that due work through this cut is complete.
      var due=jdbc.query("""
          SELECT id,moderation_subject_id FROM accounts WHERE moderation_subject_id IS NOT NULL
            AND (moderation_purge_at <= :cut OR EXISTS(SELECT 1 FROM moderation_terminal_subjects t WHERE t.subject_id=accounts.moderation_subject_id))
          ORDER BY id LIMIT 50001 FOR UPDATE
          """,Map.of("cut",java.sql.Timestamp.from(through)),(rs,row)->Map.entry(rs.getObject(1,UUID.class),rs.getObject(2,UUID.class)));
      if(due.size()>50000)throw Status.RESOURCE_EXHAUSTED.withDescription("complete due terminal work unavailable").asRuntimeException();
      purgeLocked(due);
      return through;
    });
    return snapshots.execute(transaction->{
      var authority=Objects.requireNonNull(jdbc.queryForObject("SELECT generation,terminal_high_water FROM moderation_authority_generation WHERE singleton",Map.of(),
          (rs,row)->new Authority(rs.getObject(1,UUID.class),rs.getLong(2))));
      var subjects=jdbc.query("""
          SELECT subject_id FROM moderation_terminal_subjects ORDER BY subject_id LIMIT 50001
          """,Map.of(),(rs,row)->rs.getObject(1,UUID.class).toString());
      if(subjects.size()>50000)throw Status.RESOURCE_EXHAUSTED.withDescription("complete terminal authority view unavailable").asRuntimeException();
      if(authority.highWater()!=subjects.size())throw Status.FAILED_PRECONDITION.withDescription("terminal authority integrity unavailable").asRuntimeException();
      var seal=ModerationAuthoritySnapshotSeal.newBuilder().setProtocolVersion(2)
          .setAuthorityGeneration(authority.generation().toString()).setSnapshotId(UUID.randomUUID().toString())
          .setCut(timestamp(cut)).setTerminalHighWater(authority.highWater()).setItemCount(subjects.size())
          .setCanonicalSha256(com.google.protobuf.ByteString.copyFrom(terminalDigest(subjects))).setComplete(true);
      return ListTerminalModerationSubjectsResponse.newBuilder().addAllSubjectIds(subjects).setComplete(true).setSeal(seal).build();
    });
  }
  private record Authority(UUID generation,long highWater) {}
  public ListModerationAccountCensusResponse accountCensus() {
    return snapshots.execute(transaction->{
      Instant cut=Objects.requireNonNull(jdbc.queryForObject("SELECT clock_timestamp()",Map.of(),java.sql.Timestamp.class)).toInstant();
      var authority=Objects.requireNonNull(jdbc.queryForObject("SELECT generation,terminal_high_water FROM moderation_authority_generation WHERE singleton",Map.of(),
          (rs,row)->new Authority(rs.getObject(1,UUID.class),rs.getLong(2))));
      var accounts=jdbc.query("""
          SELECT id,moderation_subject_id,moderation_lifecycle_revision,moderation_deletion_cycle_id,
                 moderation_purge_at,deleted_at,status,session_epoch
          FROM accounts WHERE moderation_subject_id IS NOT NULL
            AND (moderation_purge_at IS NULL OR moderation_purge_at > :cut)
            AND NOT EXISTS(SELECT 1 FROM moderation_terminal_subjects t WHERE t.subject_id=accounts.moderation_subject_id)
          ORDER BY id LIMIT 10001
          """,Map.of("cut",java.sql.Timestamp.from(cut)),(rs,row)->accountBinding(rs,rs.getObject("id",UUID.class)));
      if(accounts.size()>10000)throw Status.RESOURCE_EXHAUSTED.withDescription("complete moderation account census unavailable").asRuntimeException();
      var seal=ModerationAuthoritySnapshotSeal.newBuilder().setProtocolVersion(1)
          .setAuthorityGeneration(authority.generation().toString()).setSnapshotId(UUID.randomUUID().toString())
          .setCut(timestamp(cut)).setTerminalHighWater(authority.highWater()).setItemCount(accounts.size())
          .setCanonicalSha256(com.google.protobuf.ByteString.copyFrom(accountCensusDigest(accounts))).setComplete(true);
      return ListModerationAccountCensusResponse.newBuilder().addAllAccounts(accounts).setComplete(true).setSeal(seal).build();
    });
  }
  private static ResolveModerationSubjectResponse accountBinding(ResultSet rs,UUID account) throws SQLException {
    UUID subject=rs.getObject("moderation_subject_id",UUID.class);
    var binding=SubjectBinding.newBuilder().setProtocolVersion(1).setAccountId(account.toString()).setSubjectId(subject.toString())
        .setLifecycleRevision(rs.getLong("moderation_lifecycle_revision"))
        .setState(rs.getTimestamp("deleted_at")==null?SubjectState.SUBJECT_STATE_LIVE:SubjectState.SUBJECT_STATE_DELETED);
    UUID cycle=rs.getObject("moderation_deletion_cycle_id",UUID.class);
    if(cycle!=null)binding.setDeletionCycleId(cycle.toString());
    if(rs.getTimestamp("moderation_purge_at")!=null)binding.setPurgeAt(timestamp(rs.getTimestamp("moderation_purge_at").toInstant()));
    return ResolveModerationSubjectResponse.newBuilder().setSubject(binding)
        .setAuthRestricted(!"active".equals(rs.getString("status")) || rs.getTimestamp("deleted_at")!=null)
        .setSessionEpoch(rs.getLong("session_epoch")).build();
  }
  static byte[] accountCensusDigest(List<ResolveModerationSubjectResponse> accounts) {
    try {
      var digest=MessageDigest.getInstance("SHA-256");
      digest.update("voice.automod.account-census.v1\u0000".getBytes(StandardCharsets.UTF_8));
      digest.update(ByteBuffer.allocate(Long.BYTES).putLong(accounts.size()).array());
      for(var account:accounts){byte[] body=account.toByteArray();digest.update(ByteBuffer.allocate(Integer.BYTES).putInt(body.length).array());digest.update(body);}
      return digest.digest();
    }catch(NoSuchAlgorithmException ex){throw new IllegalStateException("SHA-256 unavailable",ex);}
  }
  static byte[] terminalDigest(List<String> subjects) {
    try {
      var digest=MessageDigest.getInstance("SHA-256");
      digest.update("voice.automod.terminal-ledger.v2\u0000".getBytes(StandardCharsets.UTF_8));
      digest.update(ByteBuffer.allocate(Long.BYTES).putLong(subjects.size()).array());
      for(String raw:subjects){UUID id=UUID.fromString(raw);digest.update(ByteBuffer.allocate(16).putLong(id.getMostSignificantBits()).putLong(id.getLeastSignificantBits()).array());}
      return digest.digest();
    }catch(NoSuchAlgorithmException ex){throw new IllegalStateException("SHA-256 unavailable",ex);}
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
            AND (moderation_purge_at <= clock_timestamp() OR EXISTS(SELECT 1 FROM moderation_terminal_subjects t WHERE t.subject_id=accounts.moderation_subject_id)) ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED
          """,Map.of(),(rs,row)->Map.entry(rs.getObject(1,UUID.class),rs.getObject(2,UUID.class)));
      purgeLocked(due);
    });
  }
  private void purgeLocked(List<Map.Entry<UUID,UUID>> due) {
    for(var entry:due) {
      var parameters=Map.of("account",entry.getKey(),"subject",entry.getValue());
      // Re-read current owner state/time AFTER its account lock. A stale
      // scheduled DELETED observation never becomes a permanent fact itself.
      jdbc.update("""
          INSERT INTO moderation_terminal_subjects(subject_id)
          SELECT moderation_subject_id FROM accounts WHERE id=:account AND moderation_subject_id=:subject
            AND (moderation_purge_at<=clock_timestamp() OR EXISTS(SELECT 1 FROM moderation_terminal_subjects t WHERE t.subject_id=:subject))
          ON CONFLICT DO NOTHING
          """,parameters);
      jdbc.update("DELETE FROM moderation_account_restrictions WHERE account_id=:account AND EXISTS(SELECT 1 FROM moderation_terminal_subjects WHERE subject_id=:subject)",parameters);
      jdbc.update("UPDATE accounts SET moderation_subject_id=NULL WHERE id=:account AND moderation_subject_id=:subject AND EXISTS(SELECT 1 FROM moderation_terminal_subjects WHERE subject_id=:subject)",parameters);
    }
  }
  private static Timestamp timestamp(Instant instant){return Timestamp.newBuilder().setSeconds(instant.getEpochSecond()).setNanos(instant.getNano()).build();}
}
