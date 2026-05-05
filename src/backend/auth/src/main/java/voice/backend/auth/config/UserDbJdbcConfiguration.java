package voice.backend.auth.config;

import javax.sql.DataSource;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Conditional;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

@Configuration
@Conditional(JdbcPersistenceWithUserDbUrlCondition.class)
public class UserDbJdbcConfiguration {

  @Bean(name = "userDataSource")
  DataSource userDataSource(AuthProperties properties) {
    AuthProperties.UserDb u = properties.getUserDb();
    return DataSourceBuilder.create()
        .url(u.getJdbcUrl())
        .username(u.resolveUsername())
        .password(u.resolvePassword())
        .driverClassName("org.postgresql.Driver")
        .build();
  }

  @Bean(name = "userJdbc")
  NamedParameterJdbcTemplate userJdbc(@Qualifier("userDataSource") DataSource userDataSource) {
    return new NamedParameterJdbcTemplate(userDataSource);
  }
}
