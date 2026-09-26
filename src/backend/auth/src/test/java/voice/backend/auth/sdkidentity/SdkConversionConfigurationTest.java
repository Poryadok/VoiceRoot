package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

import java.time.Clock;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.RegistrationIntentBinder;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

class SdkConversionConfigurationTest {
  private ApplicationContextRunner contextWithoutEligibility() {
    return new ApplicationContextRunner()
        .withUserConfiguration(SdkConversionConfiguration.class, SdkConversionRestController.class)
        .withBean(Clock.class, Clock::systemUTC)
        .withBean(NamedParameterJdbcTemplate.class, () -> mock(NamedParameterJdbcTemplate.class))
        .withBean(PlatformTransactionManager.class, () -> mock(PlatformTransactionManager.class))
        .withBean(SdkIdentityService.class, () -> mock(SdkIdentityService.class))
        .withBean(SdkAuthorizationService.class, () -> mock(SdkAuthorizationService.class))
        .withBean(AuthService.class, () -> mock(AuthService.class))
        .withBean(PrimaryProfileProvisioner.class, () -> mock(PrimaryProfileProvisioner.class))
        .withBean(TokenBlacklist.class, () -> mock(TokenBlacklist.class));
  }

  private ApplicationContextRunner context() {
    return contextWithoutEligibility().withBean(SdkProfileEligibility.class, () -> mock(SdkProfileEligibility.class));
  }

  @Test
  void defaultOffCreatesNoConversionServiceControllerOrRegistrationBinder() {
    context().withPropertyValues("auth.persistence=jdbc").run(ctx -> {
      assertThat(ctx).hasNotFailed().doesNotHaveBean(SdkConversionService.class)
          .doesNotHaveBean(SdkConversionRestController.class).doesNotHaveBean(RegistrationIntentBinder.class);
    });
  }

  @Test
  void otherSdkFeaturesDoNotImplicitlyEnableConversion() {
    context().withPropertyValues("auth.persistence=jdbc", "auth.sdk-identity.enabled=true",
        "auth.sdk-authorization.enabled=true", "auth.sdk-conversion.enabled=false").run(ctx -> {
          assertThat(ctx).hasNotFailed().doesNotHaveBean(SdkConversionService.class)
              .doesNotHaveBean(SdkConversionRestController.class).doesNotHaveBean(RegistrationIntentBinder.class);
        });
  }

  @Test
  void enabledJdbcCreatesServiceControllerAndConcreteAtomicIntentBinder() {
    context().withPropertyValues("auth.persistence=jdbc", "auth.sdk-conversion.enabled=true").run(ctx -> {
      assertThat(ctx).hasNotFailed().hasSingleBean(SdkConversionService.class)
          .hasSingleBean(SdkConversionRestController.class).hasSingleBean(RegistrationIntentBinder.class);
      assertThat(ctx.getBean(RegistrationIntentBinder.class)).isInstanceOf(JdbcSdkRegistrationIntentBinder.class);
    });
  }

  @Test
  void memoryPersistenceCannotEnableDurableConversion() {
    context().withPropertyValues("auth.persistence=memory", "auth.sdk-conversion.enabled=true").run(ctx -> {
      assertThat(ctx).hasNotFailed().doesNotHaveBean(SdkConversionService.class)
          .doesNotHaveBean(SdkConversionRestController.class).doesNotHaveBean(RegistrationIntentBinder.class);
    });
  }

  @Test
  void missingProfileAuthorityCannotBeReplacedWithPermissiveFallback() {
    contextWithoutEligibility().withPropertyValues("auth.persistence=jdbc", "auth.sdk-conversion.enabled=true")
        .run(ctx -> assertThat(ctx).hasFailed());
  }

  @Test
  void authorizationProfileAuthorityBeanIsPreserved() {
    SdkProfileEligibility profiles = mock(SdkProfileEligibility.class);
    contextWithoutEligibility().withBean(SdkProfileEligibility.class, () -> profiles)
        .withPropertyValues("auth.persistence=jdbc", "auth.sdk-conversion.enabled=true")
        .run(ctx -> {
          assertThat(ctx).hasNotFailed().hasSingleBean(SdkProfileEligibility.class)
              .hasSingleBean(SdkConversionService.class);
          assertThat(ctx.getBean(SdkProfileEligibility.class)).isSameAs(profiles);
        });
  }
}
