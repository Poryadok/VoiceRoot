package voice.backend.auth.sdkidentity;

import java.time.Clock;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;

@Configuration
@ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
public class SdkAuthorizationConfiguration {
  @Bean
  @ConditionalOnMissingBean(SdkAuthorizationPolicy.class)
  SdkAuthorizationPolicy unavailableSdkAuthorizationPolicy() {
    // Replace only with the registry adapter authenticating dedicated workload identity.
    return (application, environment) -> { throw new SdkIdentityDeniedException(); };
  }

  @Bean
  @ConditionalOnMissingBean(SdkProfileEligibility.class)
  SdkProfileEligibility unavailableSdkProfileEligibility() {
    // User's read-only eligibility RPC is required; SwitchProfile is not a substitute.
    return (account, profile) -> { throw new SdkIdentityDeniedException(); };
  }

  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc", matchIfMissing = true)
  SdkAuthorizationService sdkAuthorizationService(NamedParameterJdbcTemplate jdbc,
      PlatformTransactionManager manager, SdkIdentityService identity, AuthService auth,
      SdkAuthorizationPolicy policies, SdkProfileEligibility profiles, TokenBlacklist blacklist, Clock clock) {
    return new SdkAuthorizationService(jdbc, new TransactionTemplate(manager), identity, auth,
        policies, profiles, blacklist, clock);
  }
}
