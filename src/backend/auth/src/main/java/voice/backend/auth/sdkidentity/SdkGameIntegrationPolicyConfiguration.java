package voice.backend.auth.sdkidentity;

import java.time.Clock;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnExpression;

/** Shared trusted GIS policy client for optional Auth-owned SDK identity surfaces. */
@Configuration
@ConditionalOnExpression("${auth.sdk-identity.enabled:false} or ${auth.sdk-authorization.enabled:false}")
public class SdkGameIntegrationPolicyConfiguration {
  @Bean
  @ConditionalOnMissingBean(SdkAuthorizationPolicy.class)
  SdkGameIntegrationPolicyClient gameIntegrationSdkAuthorizationPolicy(
      @Value("${auth.sdk-authorization.game-integration-base-url:}") String baseUrl,
      @Value("${GAME_INTEGRATION_AUTH_WORKLOAD_KEY_B64:}") String keyBase64,
      @Value("${auth.sdk-authorization.allow-internal-http:false}") boolean allowInternalHttp,
      Clock clock) {
    return new SdkGameIntegrationPolicyClient(baseUrl, keyBase64, allowInternalHttp, clock);
  }

  @Bean
  @ConditionalOnMissingBean(SdkBindingAuthority.class)
  SdkBindingAuthority sdkBindingAuthority(NamedParameterJdbcTemplate jdbc,
      @Value("${auth.sdk-authorization.game-integration-base-url:}") String baseUrl,
      @Value("${GAME_INTEGRATION_AUTH_WORKLOAD_KEY_B64:}") String keyBase64,
      @Value("${auth.sdk-authorization.allow-internal-http:false}") boolean allowInternalHttp,
      Clock clock) {
    var gis = new SdkGameIntegrationPolicyClient(baseUrl, keyBase64, allowInternalHttp, clock);
    return new JdbcSdkBindingAuthority(jdbc, gis);
  }
}
