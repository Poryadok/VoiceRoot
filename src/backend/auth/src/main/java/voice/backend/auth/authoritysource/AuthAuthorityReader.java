package voice.backend.auth.authoritysource;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Context;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import javax.sql.DataSource;

/** Exact, authorization-only owning cut. No remote call runs in its transaction. */
public final class AuthAuthorityReader implements AutoCloseable {
  public record Snapshot(long revision, byte[] canonicalState, long validUntilUnixMillis) {}
  public static final int MAX_SUBJECTS=10000, MAX_STATE_BYTES=4*1024*1024;
  private static final ObjectMapper JSON=new ObjectMapper();
  private final DataSource dataSource;
  private final java.util.concurrent.ExecutorService reads=new java.util.concurrent.ThreadPoolExecutor(
      0,16,30,java.util.concurrent.TimeUnit.SECONDS,new java.util.concurrent.SynchronousQueue<>(),
      Thread.ofVirtual().name("auth-source-read-",0).factory(),new java.util.concurrent.ThreadPoolExecutor.AbortPolicy());
  public AuthAuthorityReader(DataSource dataSource) {
    if(dataSource==null) throw new IllegalArgumentException("Auth source requires owning database");
    this.dataSource=dataSource;
  }

  public void checkSchema() { read(List.of(),false); }
  public long revision(List<String> accounts) { return read(accounts,false).revision(); }
  public Snapshot snapshot(List<String> accounts) { return read(accounts,true); }

  public static void validateAccounts(List<String> accounts) {
    if(accounts==null || accounts.size()>MAX_SUBJECTS) throw new IllegalArgumentException("invalid Auth source scope");
    String previous="";
    for(String account:accounts) {
      if(!id(account) || account.compareTo(previous)<=0) throw new IllegalArgumentException("invalid Auth source scope");
      previous=account;
    }
  }
  static boolean id(String value) {
    try { var parsed=UUID.fromString(value);return !parsed.equals(new UUID(0,0)) && parsed.toString().equals(value); }
    catch(RuntimeException ex){return false;}
  }
  private static void budget(long deadline) {
    if(System.nanoTime()>=deadline || Context.current().isCancelled()) throw unavailable();
  }

  private Snapshot read(List<String> accounts,boolean stateRequired) {
    validateAccounts(accounts);
    java.util.concurrent.Future<Snapshot> pending=null;
    try {
      var context=Context.current();
      pending=reads.submit(context.wrap((java.util.concurrent.Callable<Snapshot>)()->readBounded(accounts,stateRequired)));
      return pending.get(1,java.util.concurrent.TimeUnit.SECONDS);
    } catch(Exception ex) {
      if(pending!=null)pending.cancel(true);
      if(ex instanceof InterruptedException)Thread.currentThread().interrupt();
      throw unavailable();
    }
  }
  @Override public void close(){reads.shutdownNow();}

  private Snapshot readBounded(List<String> accounts,boolean stateRequired) {
    long deadline=System.nanoTime()+1_000_000_000L;
    try(var connection=dataSource.getConnection()) {
      if(!connection.getAutoCommit()) throw unavailable();
      int oldTimeout=connection.getNetworkTimeout();
      connection.setNetworkTimeout(Runnable::run,1000);
      try {
        connection.setReadOnly(true);
        connection.setTransactionIsolation(Connection.TRANSACTION_REPEATABLE_READ);
        connection.setAutoCommit(false);
        try {
          preflight(connection,deadline);
          long revision=number(connection,"SELECT revision FROM public.auth_authority_revision WHERE singleton",deadline);
          if(revision<=0) throw unavailable();
          if(!stateRequired) {budget(deadline);connection.commit();return new Snapshot(revision,new byte[0],0);}
          var state=new LinkedHashMap<String,Object>();
          state.put("schema_version",1);state.put("revision",revision);
          for(var query:AuthSourceCatalog.QUERIES.entrySet()) state.put(query.getKey(),rows(connection,query.getValue(),accounts,deadline));
          AuthSourceState.validate(state,accounts);
          byte[] raw=JSON.writeValueAsBytes(state);
          if(raw.length>MAX_STATE_BYTES) throw unavailable();
          long sampled=number(connection,"SELECT floor(extract(epoch FROM clock_timestamp())*1000)::bigint",deadline);
          long validUntil=AuthSourceState.validUntil(state,sampled);
          budget(deadline);connection.commit();budget(deadline);
          return new Snapshot(revision,raw,validUntil);
        } catch(Exception ex) {
          try {connection.rollback();} catch(SQLException ignored) { /* close the owning connection */ }
          throw unavailable();
        }
      } finally {
        connection.setNetworkTimeout(Runnable::run,oldTimeout);
      }
    } catch(Exception ex) {throw unavailable();}
  }

  private static void preflight(Connection connection,long deadline)throws Exception {
    execute(connection,"SET LOCAL row_security=off; SET LOCAL search_path=public,pg_catalog",deadline);
    boolean flyway=number(connection,"SELECT CASE WHEN to_regclass('public.flyway_schema_history') IS NULL THEN 0 ELSE 1 END",deadline)>0;
    boolean golang=number(connection,"SELECT CASE WHEN to_regclass('public.schema_migrations') IS NULL THEN 0 ELSE 1 END",deadline)>0;
    if(flyway==golang) throw unavailable();
    String history=flyway?"flyway_schema_history":"schema_migrations";
    // The serving catalog is constant source, never a request identifier.
    String lock="LOCK TABLE public."+history;
    for(String table:AuthSourceCatalog.TABLES) lock+=",public."+table;
    execute(connection,lock+" IN ACCESS SHARE MODE",deadline);
    String historyCheck=flyway?"SELECT CASE WHEN count(*)=26 AND bool_and(success) AND count(DISTINCT version)=26 AND min(version::int)=1 AND max(version::int)=26 THEN 1 ELSE 0 END FROM public.flyway_schema_history WHERE version IS NOT NULL"
        :"SELECT CASE WHEN count(*)=1 AND bool_and(version=27 AND NOT dirty) THEN 1 ELSE 0 END FROM public.schema_migrations";
    if(number(connection,historyCheck,deadline)!=1) throw unavailable();
    var tables=new ArrayList<>(AuthSourceCatalog.TABLES);tables.add(history);
    try(var query=statement(connection,"SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY(?) AND c.relkind='r' AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity AND NOT EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=c.oid) AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid) AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=c.oid OR inhparent=c.oid)",deadline)) {
      query.setArray(1,connection.createArrayOf("text",tables.toArray()));
      try(var result=query.executeQuery()) {if(!result.next() || result.getInt(1)!=tables.size()) throw unavailable();}
    }
    for(var trigger:AuthSourceCatalog.TRIGGERS) {
      try(var query=statement(connection,"SELECT EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace n ON n.oid=p.pronamespace WHERE t.tgrelid=to_regclass(?) AND t.tgname=? AND t.tgenabled='O' AND NOT t.tgisinternal AND t.tgtype=? AND n.nspname='public' AND p.proname=? AND octet_length(t.tgargs)=0)",deadline)) {
        query.setString(1,"public."+trigger.table());query.setString(2,trigger.name());query.setInt(3,trigger.type());query.setString(4,trigger.function());
        try(var result=query.executeQuery()) {if(!result.next() || !result.getBoolean(1)) throw unavailable();}
      }
    }
    for(var function:AuthSourceCatalog.FUNCTIONS.entrySet()) {
      try(var query=statement(connection,"SELECT p.prosrc FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang WHERE n.nspname='public' AND p.proname=? AND p.pronargs=0 AND NOT p.prosecdef AND p.provolatile='v' AND l.lanname='plpgsql' AND p.prorettype='trigger'::regtype",deadline)) {
        query.setString(1,function.getKey());
        try(var result=query.executeQuery()) {
          if(!result.next()) throw unavailable();
          String digest=HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(result.getString(1).getBytes(StandardCharsets.UTF_8)));
          if(!digest.equals(function.getValue()) || result.next()) throw unavailable();
        }
      }
    }
  }
  private static PreparedStatement statement(Connection connection,String sql,long deadline)throws SQLException {
    budget(deadline);var statement=connection.prepareStatement(sql);statement.setQueryTimeout(1);return statement;
  }
  private static void execute(Connection connection,String sql,long deadline)throws SQLException {
    try(var statement=statement(connection,sql,deadline)){statement.execute();}
  }
  private static long number(Connection connection,String sql,long deadline)throws SQLException {
    try(var statement=statement(connection,sql,deadline);var result=statement.executeQuery()) {
      if(!result.next()) throw unavailable();long number=result.getLong(1);if(result.wasNull() || result.next()) throw unavailable();return number;
    }
  }
  private static List<Map<String,Object>> rows(Connection connection,String sql,List<String> accounts,long deadline)throws SQLException {
    var values=new ArrayList<Map<String,Object>>();
    try(var statement=statement(connection,sql,deadline)) {
      statement.setArray(1,connection.createArrayOf("uuid",accounts.toArray()));
      statement.setMaxRows(MAX_SUBJECTS+1);
      try(var result=statement.executeQuery()) {
        var columns=result.getMetaData();
        while(result.next()) {
          budget(deadline);if(values.size()>=MAX_SUBJECTS) throw unavailable();
          var item=new LinkedHashMap<String,Object>();
          for(int column=1;column<=columns.getColumnCount();column++) {
            Object value=result.getObject(column);
            if(value instanceof java.sql.Array array) {value=List.of((Object[])array.getArray());array.free();}
            if(value==null) throw unavailable();
            item.put(columns.getColumnLabel(column),value);
          }
          values.add(item);
        }
      }
    }
    return values;
  }
  private static IllegalStateException unavailable(){return new IllegalStateException("Auth owning source unavailable");}
}
