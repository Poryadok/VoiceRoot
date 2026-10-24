package voice.backend.auth.roleidentityauthority;

import javax.sql.DataSource;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.sql.init.dependency.DependsOnDatabaseInitialization;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;

/** Local key/capture wiring; it does not open a listener or call User. */
@Configuration(proxyBeanMethods = false)
public class RoleIdentityOwnerConfiguration {
  @Bean
  AuthIdentitySelectorSettings authIdentitySelectorSettings(Environment environment) {
    return AuthIdentitySelectorSettings.load(environment);
  }

  @Bean(initMethod = "initialize")
  @DependsOnDatabaseInitialization
  AuthIdentityKeysetBootstrap authIdentityKeysetBootstrap(Environment environment,
      ObjectProvider<DataSource> sources, AuthIdentitySelectorSettings settings) {
    if (!"jdbc".equals(environment.getProperty("auth.persistence", "jdbc"))) {
      if (settings.selectors() != null) throw new IllegalArgumentException("identity owner requires JDBC persistence");
      return new AuthIdentityKeysetBootstrap(null, null);
    }
    return new AuthIdentityKeysetBootstrap(sources.getIfAvailable(), settings.selectors(),
        settings.selectors() == null ? null : AuthIdentityDatabasePolicy.load(environment));
  }

  @Bean
  @ConditionalOnProperty(name = "AUTH_ROLE_IDENTITY_SELECTOR_KEYS_DIR")
  AuthIdentityOwnerStore authIdentityOwnerStore(Environment environment, DataSource source, AuthIdentitySelectorSettings settings,
      AuthIdentityKeysetBootstrap initializedKeyset) {
    return new AuthIdentityOwnerStore(source, settings.selectors(), AuthIdentityDatabasePolicy.load(environment));
  }
}
