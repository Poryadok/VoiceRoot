package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Map;
import org.junit.jupiter.api.Test;
import org.springframework.boot.env.YamlPropertySourceLoader;
import org.springframework.core.env.PropertySource;
import org.springframework.core.env.SystemEnvironmentPropertySource;
import org.springframework.core.io.ClassPathResource;
import org.springframework.mock.env.MockEnvironment;

/** application.yml contract for the documented Auth-to-User deadline environment variable. */
class UserGrpcDeadlineConfigurationDataContractTest {
  @Test
  void documentedEnvironmentVariableFeedsCanonicalDeadlineProperty() throws Exception {
    ConfigData configData = configDataWith(Map.of("AUTH_USER_GRPC_DEADLINE", "PT5S"));

    assertThat(configData.application().getProperty("auth.user-grpc.deadline")).isNotNull();
    assertThat(configData.environment().resolvePlaceholders(
            (String) configData.application().getProperty("auth.user-grpc.deadline")))
        .isEqualTo("PT5S");
  }

  @Test
  void absentEnvironmentVariableSuppliesDocumentedFifteenSecondDefault() throws Exception {
    ConfigData configData = configDataWith(Map.of());

    assertThat(configData.application().getProperty("auth.user-grpc.deadline")).isNotNull();
    assertThat(configData.environment().resolvePlaceholders(
            (String) configData.application().getProperty("auth.user-grpc.deadline")))
        .isEqualTo("PT15S");
  }

  private static ConfigData configDataWith(Map<String, Object> variables) throws Exception {
    MockEnvironment environment = new MockEnvironment();
    environment.getPropertySources().addFirst(
        new SystemEnvironmentPropertySource("testEnvironment", variables));
    PropertySource<?> application = new YamlPropertySourceLoader()
        .load("application", new ClassPathResource("application.yml"))
        .getFirst();
    environment.getPropertySources().addLast(application);
    return new ConfigData(environment, application);
  }

  private record ConfigData(MockEnvironment environment, PropertySource<?> application) {}
}
