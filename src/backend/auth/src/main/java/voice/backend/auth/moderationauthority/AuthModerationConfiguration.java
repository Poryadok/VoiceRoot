package voice.backend.auth.moderationauthority;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.transaction.PlatformTransactionManager;

@Configuration(proxyBeanMethods=false)
@ConditionalOnProperty(prefix="auth",name="persistence",havingValue="jdbc")
public class AuthModerationConfiguration {
  @Bean AuthModerationStore authModerationStore(NamedParameterJdbcTemplate jdbc,PlatformTransactionManager manager){return new AuthModerationStore(jdbc,manager);}
  @Bean AuthModerationService authModerationService(AuthModerationStore store){return new AuthModerationService(store);}
  @Bean TerminalDeadlineWorker authModerationTerminalDeadlineWorker(AuthModerationStore store){return new TerminalDeadlineWorker(store);}
  public static final class TerminalDeadlineWorker {
    private final AuthModerationStore store;
    TerminalDeadlineWorker(AuthModerationStore store){this.store=store;}
    @Scheduled(fixedDelayString="${AUTH_AUTOMOD_PURGE_INTERVAL_MS:1000}") public void purge(){store.purgeDue();}
  }
}
