package voice.backend.auth.moderationauthority;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.core.env.Environment;
import org.springframework.beans.factory.ObjectProvider;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

@Configuration(proxyBeanMethods=false)
@ConditionalOnProperty(prefix="auth",name="persistence",havingValue="jdbc")
public class AuthModerationConfiguration {
  @Bean AuthModerationBootAuthority authModerationBootAuthority(){return new AuthModerationBootAuthority();}
  @Bean AuthModerationStore authModerationStore(NamedParameterJdbcTemplate jdbc,PlatformTransactionManager manager,AuthModerationBootAuthority boot){return new AuthModerationStore(jdbc,manager,boot::acquire);}
  @Bean AuthModerationService authModerationService(AuthModerationStore store){return new AuthModerationService(store);}
  @Bean TerminalDeadlineWorker authModerationTerminalDeadlineWorker(AuthModerationStore store,AuthModerationBootAuthority boot){return new TerminalDeadlineWorker(store,boot);}
  @Bean(destroyMethod="close") AuthModerationStartupRuntime authModerationStartupRuntime(NamedParameterJdbcTemplate jdbc,PlatformTransactionManager manager,
      AuthModerationBootAuthority boot,Environment environment,ObjectProvider<AuthUserPrincipalIssuer> issuer){
    return new AuthModerationStartupRuntime(jdbc,manager,boot,environment,issuer.getIfAvailable());
  }
  public static final class TerminalDeadlineWorker {
    private final AuthModerationStore store;
    private final AuthModerationBootAuthority boot;
    TerminalDeadlineWorker(AuthModerationStore store){this(store,new AuthModerationBootAuthority());}
    TerminalDeadlineWorker(AuthModerationStore store,AuthModerationBootAuthority boot){this.store=store;this.boot=boot;}
    @Scheduled(fixedDelayString="${AUTH_AUTOMOD_PURGE_INTERVAL_MS:1000}") public void purge(){
      try{boot.acquire().run();}catch(io.grpc.StatusRuntimeException closed){return;}
      store.purgeDue();
    }
  }
}
