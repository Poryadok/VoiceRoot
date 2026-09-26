package voice.backend.auth.sdkidentity;

import java.time.Clock;
import org.springframework.boot.autoconfigure.condition.AllNestedConditions;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Conditional;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.RegistrationIntentBinder;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

@Configuration
@Conditional(SdkConversionConfiguration.Enabled.class)
public class SdkConversionConfiguration {
  static class Enabled extends AllNestedConditions {
    Enabled() { super(ConfigurationPhase.REGISTER_BEAN); }

    @ConditionalOnProperty(prefix = "auth.sdk-conversion", name = "enabled", havingValue = "true")
    static class OptIn {}

    @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc", matchIfMissing = true)
    static class JdbcOnly {}
  }

  @Bean
  RegistrationIntentBinder sdkRegistrationIntentBinder(NamedParameterJdbcTemplate jdbc, Clock clock) {
    return new JdbcSdkRegistrationIntentBinder(jdbc, clock);
  }

  @Bean
  SdkConversionService sdkConversionService(NamedParameterJdbcTemplate jdbc, PlatformTransactionManager manager,
      SdkIdentityService identity, SdkAuthorizationService authorization, AuthService auth,
      SdkProfileEligibility profiles, PrimaryProfileProvisioner primaries, TokenBlacklist blacklist, Clock clock) {
    return new SdkConversionService(jdbc, new TransactionTemplate(manager), identity, authorization,
        auth, profiles, primaries, blacklist, clock);
  }
}
