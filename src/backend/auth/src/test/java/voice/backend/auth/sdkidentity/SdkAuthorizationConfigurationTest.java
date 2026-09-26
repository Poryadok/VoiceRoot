package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;

import java.time.Clock;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;

class SdkAuthorizationConfigurationTest {
  private ApplicationContextRunner context() {
    return new ApplicationContextRunner()
        .withUserConfiguration(SdkAuthorizationConfiguration.class, SdkAuthorizationRestController.class)
        .withBean(Clock.class, Clock::systemUTC)
        .withBean(NamedParameterJdbcTemplate.class, () -> mock(NamedParameterJdbcTemplate.class))
        .withBean(PlatformTransactionManager.class, () -> mock(PlatformTransactionManager.class))
        .withBean(SdkIdentityService.class, () -> mock(SdkIdentityService.class))
        .withBean(AuthService.class, () -> mock(AuthService.class))
        .withBean(TokenBlacklist.class, () -> mock(TokenBlacklist.class));
  }

  @Test
  void defaultOffCreatesNeitherAuthorizationServiceNorController() {
    context().withPropertyValues("auth.persistence=jdbc").run(ctx -> {
      assertThat(ctx).hasNotFailed().doesNotHaveBean(SdkAuthorizationService.class)
          .doesNotHaveBean(SdkAuthorizationRestController.class);
    });
  }

  @Test
  void explicitFalseRemainsOffEvenWhenSdkIdentityIsEnabled() {
    context().withPropertyValues("auth.persistence=jdbc", "auth.sdk-identity.enabled=true",
        "auth.sdk-authorization.enabled=false").run(ctx -> {
          assertThat(ctx).hasNotFailed().doesNotHaveBean(SdkAuthorizationService.class)
              .doesNotHaveBean(SdkAuthorizationRestController.class);
        });
  }

  @Test
  void enabledJdbcCreatesServiceAndControllerWithClosedAuthorityAdapters() {
    context().withPropertyValues("auth.sdk-authorization.enabled=true", "auth.persistence=jdbc")
        .run(ctx -> {
          assertThat(ctx).hasNotFailed().hasSingleBean(SdkAuthorizationService.class)
              .hasSingleBean(SdkAuthorizationRestController.class)
              .hasSingleBean(SdkAuthorizationPolicy.class).hasSingleBean(SdkProfileEligibility.class);
          assertThatThrownBy(() -> ctx.getBean(SdkAuthorizationPolicy.class)
              .resolve(UUID.randomUUID(), UUID.randomUUID()))
              .isInstanceOf(SdkIdentityDeniedException.class).hasMessage("invalid_sdk_identity");
          assertThatThrownBy(() -> ctx.getBean(SdkProfileEligibility.class)
              .inspect(UUID.randomUUID(), UUID.randomUUID()))
              .isInstanceOf(SdkIdentityDeniedException.class).hasMessage("invalid_sdk_identity");
        });
  }

  @Test
  void verifiedRegistryAndUserAdaptersArePreservedWithoutCompetingDefaults() {
    SdkAuthorizationPolicy policy = mock(SdkAuthorizationPolicy.class);
    SdkProfileEligibility profiles = mock(SdkProfileEligibility.class);
    context().withPropertyValues("auth.sdk-authorization.enabled=true", "auth.persistence=jdbc")
        .withBean(SdkAuthorizationPolicy.class, () -> policy)
        .withBean(SdkProfileEligibility.class, () -> profiles)
        .run(ctx -> {
          assertThat(ctx).hasNotFailed().hasSingleBean(SdkAuthorizationService.class)
              .hasSingleBean(SdkAuthorizationPolicy.class).hasSingleBean(SdkProfileEligibility.class);
          assertThat(ctx.getBean(SdkAuthorizationPolicy.class)).isSameAs(policy);
          assertThat(ctx.getBean(SdkProfileEligibility.class)).isSameAs(profiles);
        });
  }

  @Test
  void providingRegistryAdapterDoesNotMakeMissingProfileAuthorityPermissive() {
    SdkAuthorizationPolicy policy = mock(SdkAuthorizationPolicy.class);
    context().withPropertyValues("auth.sdk-authorization.enabled=true", "auth.persistence=jdbc")
        .withBean(SdkAuthorizationPolicy.class, () -> policy)
        .run(ctx -> {
          assertThat(ctx).hasNotFailed().hasSingleBean(SdkAuthorizationPolicy.class)
              .hasSingleBean(SdkProfileEligibility.class);
          assertThat(ctx.getBean(SdkAuthorizationPolicy.class)).isSameAs(policy);
          assertThatThrownBy(() -> ctx.getBean(SdkProfileEligibility.class)
              .inspect(UUID.randomUUID(), UUID.randomUUID())).isInstanceOf(SdkIdentityDeniedException.class);
        });
  }
}
