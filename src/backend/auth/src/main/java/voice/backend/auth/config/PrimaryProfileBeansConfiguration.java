package voice.backend.auth.config;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;
import voice.backend.auth.userdb.JdbcPrimaryProfileProvisioner;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

@Configuration
public class PrimaryProfileBeansConfiguration {

  @Bean
  PrimaryProfileProvisioner primaryProfileProvisioner(
      AuthProperties props,
      @Autowired(required = false) @Qualifier("userJdbc") NamedParameterJdbcTemplate userJdbc) {
    if (props.getPersistence() == AuthProperties.PersistenceMode.MEMORY) {
      return new InMemoryPrimaryProfileProvisioner();
    }
    if (userJdbc == null) {
      throw new IllegalStateException(
          "auth.user-db.jdbc-url is required when auth.persistence=jdbc (see docs/EXEC_PLAN.md)");
    }
    return new JdbcPrimaryProfileProvisioner(userJdbc);
  }
}
