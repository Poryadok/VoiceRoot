package voice.backend.auth.linkoauth;

import java.time.Clock;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

/** JDBC retention remains available when protocol routes and all registrations are disabled. */
@Configuration(proxyBeanMethods = false)
@ConditionalOnProperty(name = "auth.persistence", havingValue = "jdbc")
public class LinkedOAuthCleanupConfiguration {
  @Bean
  LinkedOAuthCleanup linkedOAuthCleanup(NamedParameterJdbcTemplate jdbc, Clock clock) {
    return new LinkedOAuthCleanup(jdbc, clock);
  }
}
