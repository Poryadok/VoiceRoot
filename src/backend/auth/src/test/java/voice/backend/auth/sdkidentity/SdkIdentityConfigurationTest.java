package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import com.nimbusds.jose.jwk.Curve;
import com.nimbusds.jose.jwk.gen.ECKeyGenerator;
import com.nimbusds.jose.jwk.gen.RSAKeyGenerator;
import java.time.Clock;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;

class SdkIdentityConfigurationTest {
  private ApplicationContextRunner context() {
    return context(mock(NamedParameterJdbcTemplate.class));
  }

  private ApplicationContextRunner context(NamedParameterJdbcTemplate jdbc) {
    return new ApplicationContextRunner().withUserConfiguration(
            SdkIdentityConfiguration.class, SdkGameIntegrationPolicyConfiguration.class)
        .withBean(Clock.class, Clock::systemUTC)
        .withBean(NamedParameterJdbcTemplate.class, () -> jdbc)
        .withBean(PlatformTransactionManager.class, () -> mock(PlatformTransactionManager.class));
  }

  private static SdkAuthorizationPolicy.Policy policy(UUID application, UUID environment) {
    return new SdkAuthorizationPolicy.Policy(application, environment, 1, "Example Game",
        java.util.Set.of("voicegame://auth/callback"), java.util.Set.of("game.identity.read"),
        java.util.Set.of("google"));
  }

  @Test void defaultOffCreatesNoIdentityService() {
    context().run(ctx -> assertThat(ctx).doesNotHaveBean(SdkIdentityService.class));
  }

  @Test void enabledJdbcCreatesWorkingServiceWithNoAdmittedAppsByDefault() {
    context().withPropertyValues("auth.sdk-identity.enabled=true", "auth.persistence=jdbc")
        .run(ctx -> {
          assertThat(ctx).hasSingleBean(SdkIdentityService.class);
          assertThat(ctx).hasSingleBean(SdkBindingAuthority.class);
          assertThat(ctx.getBean(SdkBindingAuthority.class)).isInstanceOf(JdbcSdkBindingAuthority.class);
        });
  }

  @Test void invalidOperatorProviderConfigurationFailsStartup() {
    context().withPropertyValues("auth.sdk-identity.enabled=true", "auth.persistence=jdbc",
        "auth.sdk-identity.applications[0].application-id=aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
        "auth.sdk-identity.applications[0].environment-id=bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
        "auth.sdk-identity.applications[0].client-id=voice-client",
        "auth.sdk-identity.applications[0].game-public-jwk=developer-secret")
        .run(ctx -> assertThat(ctx).hasFailed());
  }

  @Test void challengeFailsClosedWhenNoTrustedRegistryPolicyIsConfigured() throws Exception {
    UUID application = UUID.fromString("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa");
    UUID environment = UUID.fromString("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb");
    NamedParameterJdbcTemplate jdbc = mock(NamedParameterJdbcTemplate.class);
    String gameKey = new RSAKeyGenerator(2048).generate().toPublicJWK().toJSONString();
    String deviceKey = new ECKeyGenerator(Curve.P_256).generate().toPublicJWK().toJSONString();

    context(jdbc).withPropertyValues("auth.sdk-identity.enabled=true", "auth.persistence=jdbc",
        "auth.sdk-identity.applications[0].application-id=" + application,
        "auth.sdk-identity.applications[0].environment-id=" + environment,
        "auth.sdk-identity.applications[0].client-id=voice-owned-client",
        "auth.sdk-identity.applications[0].game-public-jwk=" + gameKey)
        .run(ctx -> {
          assertThat(ctx).hasSingleBean(SdkIdentityService.class);
          assertThatThrownBy(() -> ctx.getBean(SdkIdentityService.class)
              .challenge(application, environment, deviceKey))
              .isInstanceOf(SdkIdentityDeniedException.class);
          verifyNoInteractions(jdbc);
        });
  }

  @Test void matchingTrustedPolicyAdmitsChallengeAndWritesOnlyAfterResolution() throws Exception {
    UUID application = UUID.fromString("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa");
    UUID environment = UUID.fromString("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb");
    NamedParameterJdbcTemplate jdbc = mock(NamedParameterJdbcTemplate.class);
    SdkAuthorizationPolicy policies = mock(SdkAuthorizationPolicy.class);
    when(policies.resolve(application, environment)).thenReturn(policy(application, environment));
    String gameKey = new RSAKeyGenerator(2048).generate().toPublicJWK().toJSONString();
    String deviceKey = new ECKeyGenerator(Curve.P_256).generate().toPublicJWK().toJSONString();

    context(jdbc).withBean(SdkAuthorizationPolicy.class, () -> policies)
        .withPropertyValues("auth.sdk-identity.enabled=true", "auth.persistence=jdbc",
            "auth.sdk-identity.applications[0].application-id=" + application,
            "auth.sdk-identity.applications[0].environment-id=" + environment,
            "auth.sdk-identity.applications[0].client-id=voice-owned-client",
            "auth.sdk-identity.applications[0].game-public-jwk=" + gameKey)
        .run(ctx -> {
          assertThat(ctx.getBean(SdkIdentityService.class).challenge(application, environment, deviceKey))
              .isNotNull();
          verify(policies).resolve(application, environment);
          verify(jdbc).update(org.mockito.ArgumentMatchers.contains("INSERT INTO sdk_challenges"),
              org.mockito.ArgumentMatchers.any(java.util.Map.class));
        });
  }

  @Test void mismatchedTrustedPolicyDeniesChallengeBeforeDatabaseEffects() throws Exception {
    UUID application = UUID.fromString("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa");
    UUID environment = UUID.fromString("bbbbbbbb-bbbb-4000-8000-000000000001");
    NamedParameterJdbcTemplate jdbc = mock(NamedParameterJdbcTemplate.class);
    SdkAuthorizationPolicy policies = mock(SdkAuthorizationPolicy.class);
    when(policies.resolve(application, environment)).thenReturn(policy(application, UUID.randomUUID()));
    String gameKey = new RSAKeyGenerator(2048).generate().toPublicJWK().toJSONString();
    String deviceKey = new ECKeyGenerator(Curve.P_256).generate().toPublicJWK().toJSONString();

    context(jdbc).withBean(SdkAuthorizationPolicy.class, () -> policies)
        .withPropertyValues("auth.sdk-identity.enabled=true", "auth.persistence=jdbc",
            "auth.sdk-identity.applications[0].application-id=" + application,
            "auth.sdk-identity.applications[0].environment-id=" + environment,
            "auth.sdk-identity.applications[0].client-id=voice-owned-client",
            "auth.sdk-identity.applications[0].game-public-jwk=" + gameKey)
        .run(ctx -> {
          assertThatThrownBy(() -> ctx.getBean(SdkIdentityService.class)
              .challenge(application, environment, deviceKey))
              .isInstanceOf(SdkIdentityDeniedException.class);
          verifyNoInteractions(jdbc);
        });
  }

  @Test void policyWithoutGoogleProviderDeniesChallengeBeforeDatabaseEffects() throws Exception {
    UUID application = UUID.fromString("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa");
    UUID environment = UUID.fromString("bbbbbbbb-bbbb-4000-8000-000000000001");
    NamedParameterJdbcTemplate jdbc = mock(NamedParameterJdbcTemplate.class);
    SdkAuthorizationPolicy policies = (app, env) -> new SdkAuthorizationPolicy.Policy(
        application, environment, 1, "Example Game", java.util.Set.of("voicegame://auth/callback"),
        java.util.Set.of("game.identity.read"), java.util.Set.of("apple"));
    String gameKey = new RSAKeyGenerator(2048).generate().toPublicJWK().toJSONString();
    String deviceKey = new ECKeyGenerator(Curve.P_256).generate().toPublicJWK().toJSONString();

    context(jdbc).withBean(SdkAuthorizationPolicy.class, () -> policies)
        .withPropertyValues("auth.sdk-identity.enabled=true", "auth.persistence=jdbc",
            "auth.sdk-identity.applications[0].application-id=" + application,
            "auth.sdk-identity.applications[0].environment-id=" + environment,
            "auth.sdk-identity.applications[0].client-id=voice-owned-client",
            "auth.sdk-identity.applications[0].game-public-jwk=" + gameKey)
        .run(ctx -> {
          assertThatThrownBy(() -> ctx.getBean(SdkIdentityService.class)
              .challenge(application, environment, deviceKey))
              .isInstanceOf(SdkIdentityDeniedException.class);
          verifyNoInteractions(jdbc);
        });
  }

  @Test void identityRoutesOffDoNotCreatePolicyOrIdentityServices() {
    context().run(ctx -> {
      assertThat(ctx).doesNotHaveBean(SdkIdentityService.class);
      assertThat(ctx).doesNotHaveBean(SdkAuthorizationPolicy.class);
    });
  }
}
