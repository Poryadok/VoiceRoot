package voice.backend.auth.moderationauthority;

import java.nio.file.Path;
import java.util.UUID;
import java.util.concurrent.*;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.core.env.Environment;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.transaction.PlatformTransactionManager;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

/** P0 listeners start first. Only locally verified P1 may run lifecycle/terminal work. */
final class AuthModerationStartupRuntime implements AutoCloseable {
  private final AuthModerationBootAuthority boot;private final AuthModerationStartupVerifier verifier;
  private final AuthLifecycleDeliveryStore deliveries;private final AuthLifecycleClientSettings.Runtime transport;
  private final Path contextFile;private final String environment;private final UUID deployment;
  private final int perRecipientLimit;
  private final ThreadPoolExecutor dispatch=new ThreadPoolExecutor(1,1,0,TimeUnit.SECONDS,new SynchronousQueue<>(),task->{
    var thread=new Thread(task,"auth-moderation-lifecycle");thread.setDaemon(true);return thread;
  },new ThreadPoolExecutor.AbortPolicy());
  private volatile boolean started;private volatile boolean stopped;private volatile String contextHash;
  private volatile AuthLifecyclePublisher publisher;private volatile String state="AUTHORITY_RECOVERY";
  AuthModerationStartupRuntime(NamedParameterJdbcTemplate jdbc,PlatformTransactionManager manager,AuthModerationBootAuthority boot,
      Environment settings,AuthUserPrincipalIssuer issuer) {
    this.boot=boot;verifier=new AuthModerationStartupVerifier(jdbc,manager);deliveries=new AuthLifecycleDeliveryStore(jdbc,manager);
    String file=settings.getProperty("AUTH_AUTOMOD_START_CONTEXT_FILE");
    String env=settings.getProperty("AUTH_AUTOMOD_ENVIRONMENT");String generation=settings.getProperty("AUTH_AUTOMOD_DEPLOYMENT_GENERATION");
    if(file==null&&env==null&&generation==null){contextFile=null;environment=null;deployment=null;}
    else {
      if(file==null||env==null||generation==null||file.isBlank()||!env.matches("[a-z][a-z0-9_-]{0,63}"))throw new IllegalArgumentException("complete Auth moderation startup bindings required");
      contextFile=Path.of(file);environment=env;deployment=UUID.fromString(generation);
      if(!contextFile.isAbsolute()||!deployment.toString().equals(generation)||deployment.equals(new UUID(0,0)))throw new IllegalArgumentException("invalid Auth moderation startup binding");
    }
    var endpoints=AuthLifecycleClientSettings.load(settings);
    if(endpoints.isPresent()&&issuer==null)throw new IllegalArgumentException("canonical Auth service issuer required for lifecycle delivery");
    if(endpoints.isPresent()&&contextFile==null)throw new IllegalArgumentException("protected startup context required for lifecycle delivery");
    perRecipientLimit=settings.getProperty("AUTH_AUTOMOD_LIFECYCLE_BATCH_SIZE",Integer.class,100);
    if(perRecipientLimit<1||perRecipientLimit>1000)throw new IllegalArgumentException("bounded per-recipient lifecycle batch required");
    transport=endpoints.isPresent()?endpoints.get().connect(issuer):null;
  }
  @EventListener(ApplicationReadyEvent.class) public void localListenersStarted(){started=true;verify();}
  @Scheduled(fixedDelayString="${AUTH_AUTOMOD_BOOT_RETRY_MS:1000}") public synchronized void verify() {
    if(!started||stopped||contextFile==null)return;
    try {
      var context=AuthModerationStartContext.read(contextFile);
      if(!environment.equals(context.environment())||!deployment.equals(context.deploymentGeneration()))throw new IllegalStateException("startup context workload binding differs");
      if(contextHash!=null){if(!contextHash.equals(context.sha256()))throw new IllegalStateException("startup operation changed");boot.acquire().run();return;}
      var verified=verifier.verifyLive(context,boot.bootId());boot.install(verified);
      contextHash=context.sha256();if(transport!=null)publisher=new AuthLifecyclePublisher(deliveries,transport.client(),boot.acquire());
      state="AUTHORITY_P1";
    }catch(RuntimeException incomplete){if(contextHash!=null)boot.close();if(!state.equals("RESTORE_DIVERGENCE"))state="AUTHORITY_RECOVERY";}
  }
  @Scheduled(fixedDelayString="${AUTH_AUTOMOD_LIFECYCLE_RETRY_MS:1000}") public void publish() {
    if(!started||stopped||publisher==null)return;
    try{boot.acquire().run();}catch(io.grpc.StatusRuntimeException closed){return;}
    try {dispatch.execute(()->{
      try {publisher.runOnce(perRecipientLimit);}
      catch(AuthLifecycleDeliveryStore.RestoreDivergence divergence){boot.close();state="RESTORE_DIVERGENCE";}
      catch(RuntimeException retryable){state="LIFECYCLE_RETRY";}
    });}catch(RejectedExecutionException busy){/* At most one bounded pass; no unbounded retry queue. */}
  }
  String state(){return state;}
  @Override public void close(){stopped=true;boot.close();dispatch.shutdownNow();if(transport!=null)transport.close();try{dispatch.awaitTermination(5,TimeUnit.SECONDS);}catch(InterruptedException interrupted){Thread.currentThread().interrupt();}}
}
