package voice.backend.auth.sdkidentity;

import java.time.Clock;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
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
@Import(AuthUserPrincipalConfiguration.class)
public class SdkAuthorizationConfiguration {
  @Bean
  @ConditionalOnMissingBean(SdkAuthorizationPolicy.class)
  SdkAuthorizationPolicy gameIntegrationSdkAuthorizationPolicy(
      @Value("${auth.sdk-authorization.game-integration-base-url:}") String baseUrl,
      @Value("${GAME_INTEGRATION_AUTH_WORKLOAD_KEY_B64:}") String keyBase64,
      @Value("${auth.sdk-authorization.allow-internal-http:false}") boolean allowInternalHttp,
      Clock clock) {
    return new SdkGameIntegrationPolicyClient(baseUrl, keyBase64, allowInternalHttp, clock);
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
