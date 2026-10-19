package voice.backend.auth.roleidentityauthority;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.UUID;
import javax.sql.DataSource;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.config.JdbcPersistenceConfiguration;
import voice.backend.auth.repository.AccountRepository;

@EnabledIfEnvironmentVariable(named = "AUTH_ROLE_IDENTITY_TEST_JDBC_URL", matches = ".+")
class AuthIdentitySpringCaptureJdbcTest {
  @TempDir Path keys;
  @Test void springCaptureCutoverAndMissingRequiredKeysAreHonest() throws Exception {
    assertThat(System.getProperty("os.name")).isEqualTo("Linux");
    var source = new DriverManagerDataSource(System.getenv("AUTH_ROLE_IDENTITY_TEST_JDBC_URL"), "postgres", "local-proof");
    var locations = new java.util.ArrayList<>(java.util.List.of("filesystem:src/main/resources/db/migration"));
    String compatible = System.getenv("AUTH_ROLE_IDENTITY_COMPAT_MIGRATIONS");
    if (compatible != null) locations.add("filesystem:" + compatible);
    Flyway.configure().dataSource(source).locations(locations.toArray(String[]::new)).load().migrate();
    var jdbc = new JdbcTemplate(source);
    var context = new ApplicationContextRunner()
        .withUserConfiguration(RoleIdentityOwnerConfiguration.class, JdbcPersistenceConfiguration.class)
        .withBean(DataSource.class, () -> source)
        .withBean(NamedParameterJdbcTemplate.class, () -> new NamedParameterJdbcTemplate(source))
        .withBean(StringRedisTemplate.class, () -> org.mockito.Mockito.mock(StringRedisTemplate.class))
        .withBean(AuthProperties.class, AuthProperties::new)
        .withPropertyValues("auth.persistence=jdbc");
    context.run(application -> {
      assertThat(application).hasNotFailed();
      var account = application.getBean(AccountRepository.class).create(UUID.randomUUID() + "@example.test", null, "fixture", "regular");
      assertThat((Boolean) jdbc.queryForObject("SELECT selector_complete FROM role_identity_lifetimes WHERE owner_handle="
          + "(SELECT role_identity_handle FROM accounts WHERE id=?)", Boolean.class, account.id())).isFalse();
    });
    byte[] secret = new byte[32]; Arrays.fill(secret, (byte) 87); Files.write(keys.resolve("v1.key"), secret);
    context.withPropertyValues("AUTH_ROLE_IDENTITY_SELECTOR_KEYS_DIR=" + keys,
        "AUTH_ROLE_IDENTITY_SELECTOR_KEYSET_REVISION=1", "AUTH_ROLE_IDENTITY_SELECTOR_ACTIVE_VERSION=1")
        .run(application -> {
          assertThat(application).hasNotFailed();
          var repository = application.getBean(AccountRepository.class);
          for (var account : java.util.List.of(
              repository.create(UUID.randomUUID() + "@example.test", null, "fixture", "regular"),
              repository.createRegularEmailPending(UUID.randomUUID() + "@example.test", "fixture"))) {
            assertThat((Boolean) jdbc.queryForObject("SELECT selector_complete FROM role_identity_lifetimes WHERE owner_handle="
                + "(SELECT role_identity_handle FROM accounts WHERE id=?)", Boolean.class, account.id())).isTrue();
          }
        });
    context.run(application -> assertThat(application).hasFailed());
  }
}
