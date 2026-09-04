package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Clock;
import java.time.Duration;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import voice.backend.auth.service.GuestConversionPendingUserRecoveryRunner;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

class GuestConversionPendingUserRecoveryWiringTest {
  private final ApplicationContextRunner jdbcRuntime =
      new ApplicationContextRunner()
          .withConfiguration(
              AutoConfigurations.of(
                  JdbcPersistenceConfiguration.class, GuestLifecycleConfiguration.class))
          .withUserConfiguration(JdbcSupport.class)
          .withPropertyValues(
              "auth.persistence=jdbc",
              "auth.guest-conversion.pending-user.enabled=true",
              "auth.guest-conversion.pending-user.batch-size=9",
              "auth.guest-conversion.pending-user.lease-duration=PT45S",
              "auth.guest-conversion.pending-user.interval=PT7S");

  @Test
  void jdbcRuntimeWiresAnEnabledRecoveryRunnerWithConfiguredBounds() {
    jdbcRuntime.run(
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(context).hasSingleBean(GuestConversionPendingUserRecoveryRunner.class);
          GuestConversionPendingUserRecoveryProperties properties =
              context.getBean(GuestConversionPendingUserRecoveryProperties.class);
          assertThat(properties.isEnabled()).isTrue();
          assertThat(properties.getBatchSize()).isEqualTo(9);
          assertThat(properties.getLeaseDuration()).isEqualTo(Duration.ofSeconds(45));
          assertThat(properties.getInterval()).isEqualTo(Duration.ofSeconds(7));
        });
  }

  @Test
  void disabledRecoveryDoesNotRegisterAnAccidentalSchedulerFallback() {
    jdbcRuntime
        .withPropertyValues("auth.guest-conversion.pending-user.enabled=false")
        .run(
            context -> {
              assertThat(context).hasNotFailed();
              assertThat(context).doesNotHaveBean(GuestConversionPendingUserRecoveryRunner.class);
            });
  }

  @Configuration(proxyBeanMethods = false)
  static class JdbcSupport {
    @Bean
    DriverManagerDataSource dataSource() {
      return new DriverManagerDataSource("jdbc:postgresql://localhost:5432/auth_db", "voice", "voice");
    }

    @Bean
    NamedParameterJdbcTemplate jdbc(DriverManagerDataSource dataSource) {
      return new NamedParameterJdbcTemplate(dataSource);
    }

    @Bean
    DataSourceTransactionManager transactions(DriverManagerDataSource dataSource) {
      return new DataSourceTransactionManager(dataSource);
    }

    @Bean
    Clock clock() {
      return Clock.systemUTC();
    }

    @Bean
    PrimaryProfileProvisioner primaryProfiles() {
      return new PrimaryProfileProvisioner() {
        @Override
        public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
          throw new UnsupportedOperationException();
        }

        @Override
        public void clearGuestAccountFlag(UUID accountId) {
          throw new UnsupportedOperationException();
        }
      };
    }

    @Bean
    StringRedisTemplate redis() {
      return new StringRedisTemplate();
    }

    @Bean
    AuthProperties authProperties() {
      return new AuthProperties();
    }
  }
}
