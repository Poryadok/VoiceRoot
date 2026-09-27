package voice.backend.auth.sdkidentity;

import java.time.Clock;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Import;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;

@Configuration
@ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
@Import({AuthUserPrincipalConfiguration.class, SdkGameIntegrationPolicyConfiguration.class})
public class SdkAuthorizationConfiguration {
  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc", matchIfMissing = true)
  SdkAuthorizationService sdkAuthorizationService(NamedParameterJdbcTemplate jdbc,
      PlatformTransactionManager manager, SdkIdentityService identity, AuthService auth,
      SdkAuthorizationPolicy policies, SdkProfileEligibility profiles, TokenBlacklist blacklist, Clock clock) {
    return new SdkAuthorizationService(jdbc, new TransactionTemplate(manager), identity, auth,
        policies, profiles, blacklist, clock);
  }
}
