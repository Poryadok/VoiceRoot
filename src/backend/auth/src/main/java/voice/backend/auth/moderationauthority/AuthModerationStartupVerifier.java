package voice.backend.auth.moderationauthority;

import java.nio.charset.StandardCharsets;
import java.util.*;
import java.util.regex.Pattern;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Finite local integrity for explicitly retained live storage; never restore-tail proof. */
final class AuthModerationStartupVerifier {
  static final class VerifiedAuthority {
    private final UUID bootId;private final String binding;
    private VerifiedAuthority(UUID bootId,String binding){this.bootId=bootId;this.binding=binding;}
    UUID bootId(){return bootId;}
    boolean same(VerifiedAuthority other){return other!=null&&bootId.equals(other.bootId)&&binding.equals(other.binding);}
  }
  private final NamedParameterJdbcTemplate jdbc;private final TransactionTemplate snapshots;
  AuthModerationStartupVerifier(NamedParameterJdbcTemplate jdbc,PlatformTransactionManager manager) {
    this.jdbc=Objects.requireNonNull(jdbc);snapshots=new TransactionTemplate(Objects.requireNonNull(manager));
    snapshots.setIsolationLevel(TransactionDefinition.ISOLATION_REPEATABLE_READ);snapshots.setReadOnly(true);snapshots.setTimeout(5);
  }
  VerifiedAuthority verifyLive(AuthModerationStartContext context,UUID bootId) {
    Objects.requireNonNull(context);Objects.requireNonNull(bootId);
    if(bootId.equals(new UUID(0,0)) || TransactionSynchronizationManager.isActualTransactionActive())throw new IllegalStateException("standalone current-boot authority verification required");
    if(!context.mode().equals("LIVE_CURRENT"))throw new IllegalStateException("independent operation-bound restored domain evidence required");
    return snapshots.execute(status->{
      jdbc.getJdbcTemplate().execute("SET LOCAL lock_timeout='2s'");
      jdbc.getJdbcTemplate().execute("SET LOCAL statement_timeout='3s'");
      jdbc.getJdbcTemplate().execute("""
          LOCK TABLE accounts,moderation_startup_identity,moderation_authority_generation,
          moderation_terminal_subjects,moderation_account_restrictions,moderation_lifecycle_current,
          moderation_lifecycle_delivery,moderation_terminal_delivery IN ACCESS SHARE MODE
          """);
      UUID incarnation=jdbc.queryForObject("SELECT incarnation FROM moderation_startup_identity WHERE singleton",Map.of(),UUID.class);
      UUID generation=jdbc.queryForObject("SELECT generation FROM moderation_authority_generation WHERE singleton",Map.of(),UUID.class);
      if(!context.storeIncarnation().equals(incarnation)||!context.authorityGeneration().equals(generation))throw invalid();
      verifyCatalog();
      int terminal=boundedCount("moderation_terminal_subjects",50000);
      long high=Objects.requireNonNull(jdbc.queryForObject("SELECT terminal_high_water FROM moderation_authority_generation WHERE singleton",Map.of(),Long.class));
      if(high!=terminal)throw invalid();
      boundedCount("moderation_lifecycle_current",50000);
      boundedCount("moderation_lifecycle_delivery",100000);
      boundedCount("moderation_terminal_delivery",100000);
      rejectIf("""
          SELECT EXISTS(SELECT 1 FROM accounts a LEFT JOIN moderation_lifecycle_current c ON c.subject_id=a.moderation_subject_id
            WHERE a.moderation_subject_id IS NOT NULL AND (
              c.subject_id IS NULL OR c.lifecycle_revision IS DISTINCT FROM a.moderation_lifecycle_revision
              OR c.authority_generation IS DISTINCT FROM :generation OR c.protocol_version<>1
              OR c.state IS DISTINCT FROM CASE WHEN a.deleted_at IS NULL THEN 'LIVE' ELSE 'DELETED' END
              OR c.deletion_cycle_id IS DISTINCT FROM a.moderation_deletion_cycle_id OR c.purge_at IS DISTINCT FROM a.moderation_purge_at
              OR (a.deleted_at IS NULL AND (c.deletion_cycle_id IS NOT NULL OR c.purge_at IS NOT NULL))
              OR (a.deleted_at IS NOT NULL AND (c.deletion_cycle_id IS NULL OR c.purge_at IS NULL))
              OR EXISTS(SELECT 1 FROM moderation_terminal_subjects t WHERE t.subject_id=a.moderation_subject_id)))
          """,generation);
      rejectIf("""
          SELECT EXISTS(SELECT 1 FROM moderation_lifecycle_current c WHERE
            NOT EXISTS(SELECT 1 FROM accounts a WHERE a.moderation_subject_id=c.subject_id)
            OR EXISTS(SELECT 1 FROM moderation_terminal_subjects t WHERE t.subject_id=c.subject_id))
          """,generation);
      rejectIf("""
          SELECT EXISTS(SELECT 1 FROM moderation_lifecycle_delivery d LEFT JOIN moderation_lifecycle_current c USING(subject_id)
            WHERE c.subject_id IS NULL OR d.event_id IS DISTINCT FROM c.event_id OR d.recipient NOT IN ('messaging','moderation'))
          """,generation);
      rejectIf("""
          SELECT EXISTS(SELECT 1 FROM moderation_terminal_delivery d LEFT JOIN moderation_terminal_subjects t USING(subject_id)
            WHERE t.subject_id IS NULL OR d.event_id IS DISTINCT FROM t.event_id OR d.authority_generation IS DISTINCT FROM :generation
            OR d.recipient NOT IN ('messaging','moderation'))
          """,generation);
      rejectIf("""
          SELECT EXISTS(SELECT 1 FROM moderation_account_restrictions r LEFT JOIN accounts a ON a.id=r.account_id
            WHERE a.id IS NULL OR r.subject_id IS DISTINCT FROM a.moderation_subject_id OR r.desired_revision<0)
          OR EXISTS(SELECT 1 FROM accounts a WHERE a.moderation_subject_id IS NOT NULL AND
            NOT EXISTS(SELECT 1 FROM moderation_account_restrictions r WHERE r.account_id=a.id AND r.subject_id=a.moderation_subject_id))
          """,generation);
      return new VerifiedAuthority(bootId,context.sha256()+":"+incarnation+":"+generation);
    });
  }
  private int boundedCount(String table,int limit) {
    // Table and bound are fixed internal call sites, never user/configuration input.
    Integer count=jdbc.queryForObject("SELECT count(*) FROM (SELECT 1 FROM "+table+" LIMIT "+(limit+1)+") bounded",Map.of(),Integer.class);
    if(count==null||count>limit)throw new IllegalStateException("complete finite Auth authority inventory exceeds supported bound");
    return count;
  }
  private void rejectIf(String query,UUID generation){if(!Boolean.FALSE.equals(jdbc.queryForObject(query,Map.of("generation",generation),Boolean.class)))throw invalid();}
  private void verifyCatalog() {
    var expected=new LinkedHashMap<String,String>();
    var pattern=Pattern.compile("CREATE FUNCTION\\s+([a-z_]+)\\s*\\(.*?\\).*?AS \\$\\$(.*?)\\$\\$;",Pattern.DOTALL);
    for(String migration:List.of("V27__moderation_subject_restrictions.sql","V29__moderation_bootstrap.sql","V30__moderation_lifecycle_delivery.sql","V32__moderation_startup_identity.sql")) {
      try(var resource=AuthModerationStartupVerifier.class.getResourceAsStream("/db/migration/"+migration)) {
        if(resource==null)throw invalid();String sql=new String(resource.readAllBytes(),StandardCharsets.UTF_8).replace("\r\n","\n");
        var matcher=pattern.matcher(sql);int found=0;while(matcher.find()){if(expected.put(matcher.group(1),matcher.group(2))!=null)throw invalid();found++;}
        if(found==0)throw invalid();
      }catch(java.io.IOException failure){throw new IllegalStateException("Auth native authority definition unavailable",failure);}
    }
    for(var function:expected.entrySet()) {
      var rows=jdbc.query("""
          SELECT p.prosrc,p.pronargs,p.prokind,p.provolatile,p.proparallel,p.prosecdef,p.proisstrict,p.proconfig,l.lanname
          FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang
          WHERE n.nspname='public' AND p.proname=:name
          """,Map.of("name",function.getKey()),(rs,row)->
          function.getValue().equals(rs.getString(1).replace("\r\n","\n")) && rs.getInt(2)==(function.getKey().equals("auth_reconcile_moderation")?4:0)
              && "f".equals(rs.getString(3))&&"v".equals(rs.getString(4))&&"u".equals(rs.getString(5))
              && !rs.getBoolean(6)&&!rs.getBoolean(7)&&rs.getArray(8)==null&&"plpgsql".equals(rs.getString(9)));
      if(rows.size()!=1||!rows.getFirst())throw invalid();
    }
    Map<String,String> triggers=Map.of(
        "auth_moderation_account_guard","accounts:31:auth_moderation_account_guard",
        "auth_moderation_account_created","accounts:5:auth_moderation_account_created",
        "moderation_authority_generation_guard","moderation_authority_generation:27:moderation_authority_generation_guard",
        "moderation_terminal_progress","moderation_terminal_subjects:5:moderation_terminal_progress",
        "moderation_terminal_permanent","moderation_terminal_subjects:27:moderation_terminal_progress",
        "auth_moderation_capture_lifecycle","accounts:29:auth_moderation_capture_lifecycle",
        "auth_moderation_capture_terminal","moderation_terminal_subjects:5:auth_moderation_capture_terminal",
        "moderation_startup_identity_guard","moderation_startup_identity:27:moderation_startup_identity_guard");
    for(var trigger:triggers.entrySet()) {
      var rows=jdbc.query("""
          SELECT c.relname,t.tgtype,p.proname,t.tgenabled,t.tgnargs,t.tgqual,t.tgattr::text,pn.nspname
          FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
          JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace
          WHERE n.nspname='public' AND NOT t.tgisinternal AND t.tgname=:name
          """,Map.of("name",trigger.getKey()),(rs,row)->trigger.getValue().equals(rs.getString(1)+":"+rs.getInt(2)+":"+rs.getString(3))
              &&"O".equals(rs.getString(4))&&rs.getInt(5)==0&&rs.getString(6)==null&&rs.getString(7).isEmpty()&&"public".equals(rs.getString(8)));
      if(rows.size()!=1||!rows.getFirst())throw invalid();
    }
  }
  private static IllegalStateException invalid(){return new IllegalStateException("Auth native moderation authority integrity unavailable");}
}
