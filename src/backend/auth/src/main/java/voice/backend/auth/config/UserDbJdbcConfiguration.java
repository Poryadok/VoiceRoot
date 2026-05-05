package voice.backend.auth.config;

import javax.sql.DataSource;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.autoconfigure.jdbc.DataSourceProperties;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Conditional;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

@Configuration
@Conditional(JdbcPersistenceWithUserDbUrlCondition.class)
public class UserDbJdbcConfiguration {

  /**
   * Second JDBC URL ({@code userDataSource}) makes Spring Boot skip auto-configured {@code DataSource};
   * without an explicit {@code @Primary} bean, {@link NamedParameterJdbcTemplate} would bind only to
   * {@code user_db}.
   */
  @Bean
  @Primary
  DataSource dataSource(DataSourceProperties properties) {
    return properties.initializeDataSourceBuilder().build();
  }

  /**
   * A {@code NamedParameterJdbcTemplate} for {@code userJdbc} alone satisfies
   * {@code @ConditionalOnMissingBean(NamedParameterOperations.class)} and prevents Boot from creating the default
   * template; repositories must use a {@code @Primary} template bound to the auth {@code DataSource}.
   */
  @Bean
  @Primary
  NamedParameterJdbcTemplate namedParameterJdbcTemplate(DataSource dataSource) {
    return new NamedParameterJdbcTemplate(dataSource);
  }

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
