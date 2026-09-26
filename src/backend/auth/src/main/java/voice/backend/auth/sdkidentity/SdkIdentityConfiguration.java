package voice.backend.auth.sdkidentity;

import com.nimbusds.jose.jwk.RSAKey;
import java.time.Clock;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

@Configuration
@ConditionalOnProperty(prefix = "auth.sdk-identity", name = "enabled", havingValue = "true")
@EnableConfigurationProperties(SdkIdentityConfiguration.Settings.class)
public class SdkIdentityConfiguration {
  @ConfigurationProperties(prefix = "auth.sdk-identity")
  public static class Settings {
    private List<Application> applications = List.of();
    public List<Application> getApplications() { return applications; }
    public void setApplications(List<Application> applications) { this.applications = List.copyOf(applications); }
  }

  public record Application(UUID applicationId, UUID environmentId, String clientId, String gamePublicJwk) {}

  @Bean
  @ConditionalOnProperty(prefix = "auth", name = "persistence", havingValue = "jdbc", matchIfMissing = true)
  SdkIdentityService sdkIdentityService(NamedParameterJdbcTemplate jdbc, PlatformTransactionManager manager,
                                      Clock clock, Settings settings) throws java.text.ParseException {
    Map<String, SdkApplication> admitted = new HashMap<>();
    var audiences = new HashSet<String>();
    for (Application app : settings.getApplications()) {
      SdkApplication admission = new SdkApplication(app.applicationId(), app.environmentId(),
          app.clientId(), RSAKey.parse(app.gamePublicJwk()));
      if (!audiences.add(app.clientId())
          || admitted.put(app.applicationId() + "/" + app.environmentId(), admission) != null) {
        throw new IllegalArgumentException("SDK app/env and Google audiences must be unique");
      }
    }
    return new SdkIdentityService(jdbc, new TransactionTemplate(manager),
        new GoogleOidcProofVerifier(clock, new GoogleJwks(clock)), Map.copyOf(admitted), clock);
  }
}
