package voice.backend.auth.reportaudience;

import app.voice.auth.v1.ReportAudienceAccount;
import app.voice.auth.v1.ReportAudienceAccountState;
import io.grpc.Context;
import java.sql.Connection;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.*;
import javax.sql.DataSource;

/** Positive classification of every requested account from Auth's own database. */
final class ReportAudienceReader implements AutoCloseable {
  static final int MAX_ACCOUNTS=1000;
  private final DataSource dataSource;
  private final ExecutorService reads=new ThreadPoolExecutor(0,16,30,TimeUnit.SECONDS,
      new SynchronousQueue<>(),Thread.ofVirtual().name("report-audience-read-",0).factory(),new ThreadPoolExecutor.AbortPolicy());
  ReportAudienceReader(DataSource dataSource){this.dataSource=java.util.Objects.requireNonNull(dataSource);}

  static UUID id(String value) {
    try {var id=UUID.fromString(value);if(id.equals(new UUID(0,0)) || !id.toString().equals(value))throw new IllegalArgumentException();return id;}
    catch(RuntimeException invalid){throw new IllegalArgumentException("invalid report audience identifier");}
  }
  static void validate(List<String> accounts) {
    if(accounts==null || accounts.isEmpty() || accounts.size()>MAX_ACCOUNTS)throw new IllegalArgumentException("invalid report audience batch");
    var unique=new HashSet<UUID>();for(var value:accounts)if(!unique.add(id(value)))throw new IllegalArgumentException("duplicate report audience account");
  }
  List<ReportAudienceAccount> classify(List<String> accounts) {
    validate(accounts);Future<List<ReportAudienceAccount>> pending=null;
    try {
      pending=reads.submit(Context.current().wrap((Callable<List<ReportAudienceAccount>>)()->read(accounts)));
      return pending.get(2,TimeUnit.SECONDS);
    } catch(Exception unavailable) {
      if(pending!=null)pending.cancel(true);if(unavailable instanceof InterruptedException)Thread.currentThread().interrupt();
      throw new IllegalStateException("Auth report audience unavailable");
    }
  }
  private List<ReportAudienceAccount> read(List<String> accounts)throws Exception {
    if(Context.current().isCancelled())throw new IllegalStateException();
    try(var connection=dataSource.getConnection()) {
      if(!connection.getAutoCommit())throw new IllegalStateException();
      int oldTimeout=connection.getNetworkTimeout();connection.setNetworkTimeout(Runnable::run,1500);
      try {
        connection.setReadOnly(true);connection.setTransactionIsolation(Connection.TRANSACTION_REPEATABLE_READ);connection.setAutoCommit(false);
        try {
          var classified=new HashMap<UUID,ReportAudienceAccountState>();
          try(var query=connection.prepareStatement("SELECT id,deleted_at IS NOT NULL AS deleted,status FROM public.accounts WHERE id=ANY(?)")) {
            query.setQueryTimeout(1);
            var array=connection.createArrayOf("uuid",accounts.stream().map(ReportAudienceReader::id).toArray());
            try {
              query.setArray(1,array);
              try(var rows=query.executeQuery()) {
                while(rows.next()) {
                  var account=rows.getObject("id",UUID.class);boolean deleted=rows.getBoolean("deleted");String status=rows.getString("status");
                  // Suspensions do not shrink potential reach; an inconsistent deletion
                  // projection cannot be silently treated as either LIVE or MISSING.
                  if(status==null || (!deleted && "deleted".equals(status)))throw new IllegalStateException();
                  var state=deleted?ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_DELETED:ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_LIVE;
                  if(classified.put(account,state)!=null)throw new IllegalStateException();
                }
              }
            } finally {array.free();}
          }
          var result=new ArrayList<ReportAudienceAccount>(accounts.size());
          for(var account:accounts)result.add(ReportAudienceAccount.newBuilder().setAccountId(account).setState(classified.getOrDefault(id(account),ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_MISSING)).build());
          if(Context.current().isCancelled() || Thread.currentThread().isInterrupted())throw new IllegalStateException();
          connection.commit();return List.copyOf(result);
        } catch(Exception error){try{connection.rollback();}catch(Exception ignored){/* discard owning connection */}throw error;}
      } finally {connection.setNetworkTimeout(Runnable::run,oldTimeout);}
    }
  }
  @Override public void close(){reads.shutdownNow();}
}
