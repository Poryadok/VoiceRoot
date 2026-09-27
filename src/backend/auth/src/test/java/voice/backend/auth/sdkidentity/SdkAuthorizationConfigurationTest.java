package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;

import java.nio.file.Files;
import java.nio.file.Path;
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
  void allPrincipalSettingsAbsentAreAllowedWhenAuthorizationIsOff() {
    context().withUserConfiguration(AuthUserPrincipalConfiguration.class)
        .withPropertyValues("auth.persistence=jdbc").run(ctx -> {
      assertThat(ctx).hasNotFailed()
          .doesNotHaveBean(AuthUserPrincipalIssuer.class)
          .doesNotHaveBean(io.grpc.ManagedChannel.class)
          .doesNotHaveBean(SdkProfileEligibility.class);
    });
  }

  @Test
  void partialPrincipalSettingsFailStartupWhenAuthorizationFlagIsAbsent() {
    context().withUserConfiguration(AuthUserPrincipalConfiguration.class).withPropertyValues(
        "auth.persistence=jdbc",
        "AUTH_PRINCIPAL_SIGNING_KEYS_DIR=/run/secrets/auth-principal")
        .run(ctx -> assertThat(ctx).hasFailed());
  }

  @Test
  void partialPrincipalSettingsFailStartupWhenAuthorizationIsExplicitlyDisabled() {
    context().withUserConfiguration(AuthUserPrincipalConfiguration.class).withPropertyValues(
        "auth.persistence=jdbc",
        "auth.sdk-authorization.enabled=false",
        "AUTH_USER_PRINCIPAL_GRPC_ADDR=user.internal:9094")
        .run(ctx -> assertThat(ctx).hasFailed());
  }

  @Test
  void enabledJdbcRequiresTheDedicatedAuthUserPrincipalConfiguration() {
    context().withPropertyValues("auth.sdk-authorization.enabled=true", "auth.persistence=jdbc")
        .run(ctx -> assertThat(ctx).hasFailed());
  }

  @Test
  void partialAuthUserPrincipalConfigurationFailsStartupInsteadOfUsingAnUnavailableAdapter() {
    context().withPropertyValues("auth.sdk-authorization.enabled=true", "auth.persistence=jdbc",
        "AUTH_PRINCIPAL_SIGNING_KEYS_DIR=/run/secrets/auth-principal",
        "AUTH_PRINCIPAL_ACTIVE_KID=current")
        .run(ctx -> assertThat(ctx).hasFailed());
  }

  @Test
  void configuredRegistryAndProfileAdapterCannotBypassMissingPrincipalConfiguration() {
    SdkAuthorizationPolicy policy = mock(SdkAuthorizationPolicy.class);
    SdkProfileEligibility profiles = mock(SdkProfileEligibility.class);
    context().withPropertyValues("auth.sdk-authorization.enabled=true", "auth.persistence=jdbc")
        .withBean(SdkAuthorizationPolicy.class, () -> policy)
        .withBean(SdkProfileEligibility.class, () -> profiles)
        .run(ctx -> assertThat(ctx).hasFailed());
  }

  @Test
  void registryOverrideDoesNotBypassMissingProfileAuthorityConfiguration() {
    SdkAuthorizationPolicy policy = mock(SdkAuthorizationPolicy.class);
    context().withPropertyValues("auth.sdk-authorization.enabled=true", "auth.persistence=jdbc")
        .withBean(SdkAuthorizationPolicy.class, () -> policy)
        .run(ctx -> {
          assertThat(ctx).hasFailed();
        });
  }

  @Test
  void malformedAuthPrincipalKeysetFailsStartup(@org.junit.jupiter.api.io.TempDir Path temp) throws Exception {
    Files.writeString(temp.resolve("current.pem"), "not a PKCS#8 private key");
    Files.writeString(temp.resolve("next.pem"), "also not a PKCS#8 private key");
    context().withPropertyValues("auth.sdk-authorization.enabled=true", "auth.persistence=jdbc",
        "AUTH_PRINCIPAL_SIGNING_KEYS_DIR=" + temp,
        "AUTH_PRINCIPAL_ACTIVE_KID=current",
        "AUTH_USER_PRINCIPAL_GRPC_ADDR=localhost:9094")
        .run(ctx -> assertThat(ctx).hasFailed());
  }

  @Test
  void completePrincipalKeysetWiresTheUserBackedProfileEligibilityClient(@org.junit.jupiter.api.io.TempDir Path temp)
      throws Exception {
    writePrivateKey(temp.resolve("current.pem"));
    writePrivateKey(temp.resolve("next.pem"));
    Class<?> principalConfiguration = Class.forName(
        "voice.backend.auth.sdkidentity.AuthUserPrincipalConfiguration");

    context().withPropertyValues("auth.sdk-authorization.enabled=true", "auth.persistence=jdbc",
            "AUTH_PRINCIPAL_SIGNING_KEYS_DIR=" + temp,
            "AUTH_PRINCIPAL_ACTIVE_KID=current",
            "AUTH_USER_PRINCIPAL_GRPC_ADDR=voice-user:9094")
        .run(ctx -> {
          assertThat(ctx).hasNotFailed().hasSingleBean(SdkAuthorizationService.class)
              .hasSingleBean(SdkProfileEligibility.class);
          assertThat(ctx.getBean(SdkProfileEligibility.class).getClass().getSimpleName())
              .isEqualTo("AuthUserProfileEligibilityClient");
        });
  }

  private static void writePrivateKey(Path path) throws Exception {
    var generator = java.security.KeyPairGenerator.getInstance("RSA");
    generator.initialize(2048);
    byte[] der = generator.generateKeyPair().getPrivate().getEncoded();
    Files.writeString(path, "-----BEGIN PRIVATE KEY-----\n"
        + java.util.Base64.getMimeEncoder(64, new byte[] {'\n'}).encodeToString(der)
        + "\n-----END PRIVATE KEY-----\n");
  }
}
