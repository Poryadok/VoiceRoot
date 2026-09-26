package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

import java.time.Clock;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;

class SdkIdentityConfigurationTest {
  private ApplicationContextRunner context() {
    return new ApplicationContextRunner().withUserConfiguration(SdkIdentityConfiguration.class)
        .withBean(Clock.class, Clock::systemUTC)
        .withBean(NamedParameterJdbcTemplate.class, () -> mock(NamedParameterJdbcTemplate.class))
        .withBean(PlatformTransactionManager.class, () -> mock(PlatformTransactionManager.class));
  }

  @Test void defaultOffCreatesNoIdentityService() {
    context().run(ctx -> assertThat(ctx).doesNotHaveBean(SdkIdentityService.class));
  }

  @Test void enabledJdbcCreatesWorkingServiceWithNoAdmittedAppsByDefault() {
    context().withPropertyValues("auth.sdk-identity.enabled=true", "auth.persistence=jdbc")
        .run(ctx -> assertThat(ctx).hasSingleBean(SdkIdentityService.class));
  }

  @Test void invalidOperatorProviderConfigurationFailsStartup() {
    context().withPropertyValues("auth.sdk-identity.enabled=true", "auth.persistence=jdbc",
        "auth.sdk-identity.applications[0].application-id=aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
        "auth.sdk-identity.applications[0].environment-id=bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
        "auth.sdk-identity.applications[0].client-id=voice-client",
        "auth.sdk-identity.applications[0].game-public-jwk=developer-secret")
        .run(ctx -> assertThat(ctx).hasFailed());
  }
}
