package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import app.voice.user.v1.Profile;
import com.google.protobuf.Timestamp;
import io.grpc.Context;
import io.grpc.Metadata;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.ServerInterceptors;
import io.grpc.ServerCall.Listener;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.stream.Stream;
import javax.sql.DataSource;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Bean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.config.JdbcPersistenceConfiguration;
import voice.backend.auth.config.PrimaryProfileBeansConfiguration;
import voice.backend.auth.config.UserGrpcClientConfiguration;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.support.RecordingUserGrpcService;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AccountDeletionOperationStarter;
import voice.backend.auth.service.ProfileSwitchException;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;
import voice.backend.auth.userdb.ProfileSwitchValidator;
import voice.backend.auth.userdb.UserVerificationSync;

/** Uses the production Spring gRPC client configuration and an actual TCP gRPC endpoint. */
class UserGrpcClientConfigurationContractTest {
  private final ApplicationContextRunner contextRunner = new ApplicationContextRunner()
      .withConfiguration(AutoConfigurations.of(UserGrpcClientConfiguration.class, PropertiesConfiguration.class));

  @Test
  void jdbcAuthRuntimeStartsWithOnlyAuthDataSourceAndUserGrpc() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      new ApplicationContextRunner()
          .withConfiguration(AutoConfigurations.of(
              UserGrpcClientConfiguration.class,
              JdbcPersistenceConfiguration.class,
              PrimaryProfileBeansConfiguration.class,
              JdbcRuntimePropertiesConfiguration.class))
          .withPropertyValues(
              "auth.persistence=jdbc",
              "auth.user-grpc.addr=localhost:" + fake.server.getPort())
          .run(context -> {
            assertThat(context).hasNotFailed();
            assertThat(context).hasBean("dataSource");
            assertThat(context).hasBean("authJdbc");
            assertThat(context).doesNotHaveBean("userDataSource");
            assertThat(context).doesNotHaveBean("userJdbc");
            assertThat(context).doesNotHaveBean(AccountDeletionOperationStarter.class);
            assertThat(context).hasSingleBean(PrimaryProfileProvisioner.class);
            assertThat(context).hasSingleBean(ProfileSwitchValidator.class);
            assertThat(context).hasSingleBean(PhoneHashResolver.class);
            assertThat(context).hasSingleBean(UserVerificationSync.class);
          });
    }
  }

  @Test
  void jdbcAuthRuntimeFailsClosedWithoutUserGrpcAddress() {
    jdbcRuntimeContext()
        .withPropertyValues("auth.persistence=jdbc")
        .run(context -> {
          assertThat(context).hasFailed();
          assertThat(context.getStartupFailure())
              .hasMessageContaining("UserVerificationSync");
        });
  }

  @Test
  void productionUserGrpcConfigurationUsesTcpAndWiresEveryAuthUserPort() throws Exception {
    RecordingUserGrpcService user = new RecordingUserGrpcService();
    MetadataCapture metadata = new MetadataCapture();
    Server server = ServerBuilder.forPort(0)
        .addService(ServerInterceptors.intercept(user, metadata)).build().start();
    try {
      contextRunner.withPropertyValues("auth.user-grpc.addr=localhost:" + server.getPort())
          .run(context -> {
            UserVerificationSync verification = context.getBean(UserVerificationSync.class);
            UUID profileId = UUID.randomUUID();
            verification.setPersonalVerification(profileId, "twitch");
            verification.clearVerification(profileId);

            assertThat(user.rpcCalls()).containsExactly("SetVerification", "ClearVerification");
            assertThat(metadata.internalCallers).containsOnly("auth");
            assertThat(context.getBeansOfType(PrimaryProfileProvisioner.class)).hasSize(1);
            assertThat(context.getBeansOfType(ProfileSwitchValidator.class)).hasSize(1);
            assertThat(context.getBeansOfType(PhoneHashResolver.class)).hasSize(1);
          });
    } finally {
      server.shutdownNow();
    }
  }

  @Test
  void configuredPositiveDeadlineBoundsEveryBlockingAuthToUserRpc() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      Duration configuredDeadline = Duration.ofSeconds(5);
      invokeEveryAuthUserRpc(fake, "auth.user-grpc.deadline=" + configuredDeadline);

      fake.deadlines.assertEveryAuthUserRpcIsBoundedBy(configuredDeadline);
    }
  }

  @Test
  void absentDeadlineUsesTheDocumentedFifteenSecondDefault() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      invokeEveryAuthUserRpc(fake, null);

      fake.deadlines.assertEveryAuthUserRpcIsBoundedBy(Duration.ofSeconds(15));
    }
  }

  @ParameterizedTest(name = "deadline={0} fails Auth startup")
  @MethodSource("invalidUserGrpcDeadlines")
  void explicitInvalidDeadlineFailsStartupInsteadOfFallingBack(String configuredDeadline)
      throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      runner(fake)
          .withPropertyValues("auth.user-grpc.deadline=" + configuredDeadline)
          .run(context -> assertThat(context).hasFailed());
    }
  }

  @ParameterizedTest(name = "EnsurePrimaryProfile {0} maps fail-closed to {1}")
  @MethodSource("ensurePrimaryProfileFailures")
  void ensurePrimaryProfileMapsAuditedUserFailuresFailClosed(
      RecordingUserGrpcService.Outcome failure, String expectedMessage) throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      fake.user.setEnsureOutcome(failure);
      runner(fake).run(context -> {
        PrimaryProfileProvisioner port = context.getBean(PrimaryProfileProvisioner.class);

        assertThatThrownBy(() -> port.ensurePrimaryProfile(UUID.randomUUID(), "x", false))
            .isExactlyInstanceOf(AuthException.class)
            .hasMessage(expectedMessage);
      });
    }
  }

  @ParameterizedTest(name = "SwitchProfile {0} maps to {2}")
  @MethodSource("switchProfileFailures")
  void switchProfileMapsAuditedUserFailuresFailClosed(
      RecordingUserGrpcService.Outcome failure,
      Class<? extends RuntimeException> expectedType,
      String expectedMessage,
      ProfileSwitchException.Kind expectedKind) throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      fake.user.setSwitchOutcome(failure);
      runner(fake).run(context -> {
        ProfileSwitchValidator port = context.getBean(ProfileSwitchValidator.class);

        assertThatThrownBy(() -> port.validateOwnedSwitchable(UUID.randomUUID(), UUID.randomUUID()))
            .isExactlyInstanceOf(expectedType)
            .hasMessage(expectedMessage)
            .satisfies(error -> {
              if (expectedKind != null) {
                assertThat(((ProfileSwitchException) error).kind()).isEqualTo(expectedKind);
              }
            });
      });
    }
  }

  @Test
  void deadlineExceededFailsClosedWithEachExistingAuthUserSemanticMapping() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      runner(fake).run(context -> {
        UUID accountId = UUID.randomUUID();
        UUID profileId = UUID.randomUUID();
        InMemoryAccountRepository accounts = context.getBean(InMemoryAccountRepository.class);
        accounts.create("deadline@example.com", "deadline-hash", "hash", "regular");

        fake.user.setEnsureOutcome(RecordingUserGrpcService.Outcome.DEADLINE_EXCEEDED);
        assertThatThrownBy(() -> context.getBean(PrimaryProfileProvisioner.class)
                .ensurePrimaryProfile(accountId, "deadline@example.com", false))
            .isExactlyInstanceOf(AuthException.class)
            .hasMessage("auth_unavailable");

        fake.user.setResolveOutcome(RecordingUserGrpcService.Outcome.DEADLINE_EXCEEDED);
        assertThatThrownBy(() -> context.getBean(PhoneHashResolver.class)
                .resolvePrimaryProfileIdsByPhoneHashes(List.of("deadline-hash")))
            .isExactlyInstanceOf(AuthException.class)
            .hasMessage("auth_unavailable");

        fake.user.setSwitchOutcome(RecordingUserGrpcService.Outcome.DEADLINE_EXCEEDED);
        assertThatThrownBy(() -> context.getBean(ProfileSwitchValidator.class)
                .validateOwnedSwitchable(accountId, profileId))
            .isExactlyInstanceOf(AuthException.class)
            .hasMessage("auth_unavailable");

        UserVerificationSync verification = context.getBean(UserVerificationSync.class);
        fake.user.setSetVerificationOutcome(RecordingUserGrpcService.Outcome.DEADLINE_EXCEEDED);
        assertThatThrownBy(() -> verification.setPersonalVerification(profileId, "verified"))
            .isExactlyInstanceOf(AuthException.class)
            .hasMessage("verification_sync_failed");
        fake.user.setClearVerificationOutcome(RecordingUserGrpcService.Outcome.DEADLINE_EXCEEDED);
        assertThatThrownBy(() -> verification.clearVerification(profileId))
            .isExactlyInstanceOf(AuthException.class)
            .hasMessage("verification_sync_failed");

        fake.user.setMarkRegularOutcome(RecordingUserGrpcService.Outcome.DEADLINE_EXCEEDED);
        assertThatThrownBy(() -> context.getBean(PrimaryProfileProvisioner.class)
                .clearGuestAccountFlag(accountId))
            .isExactlyInstanceOf(AuthException.class)
            .hasMessage("auth_unavailable");
      });
    }
  }

  @Test
  void ensurePrimaryProfilePortCallsUserBeforeSessionAndRejectsUnusableResponses() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      UUID accountId = UUID.randomUUID();
      UUID profileId = UUID.randomUUID();
      fake.user.setEnsuredProfile(profile(accountId, profileId, true, false));
      runner(fake).run(context -> {
        assertThat(context).hasSingleBean(PrimaryProfileProvisioner.class);
        PrimaryProfileProvisioner port = context.getBean(PrimaryProfileProvisioner.class);
        assertThat(port.ensurePrimaryProfile(accountId, "contract@example.com", false)).isEqualTo(profileId.toString());
        assertThat(fake.user.ensureRequests()).singleElement().satisfies(request -> {
          assertThat(request.getAccountId()).isEqualTo(accountId.toString());
          assertThat(request.getDisplayHint()).isEqualTo("contract@example.com");
          assertThat(request.getIsGuestAccount()).isFalse();
        });
        assertThat(fake.metadata.internalCallers).contains("auth");
      });
    }
  }

  @Test
  void ensurePrimaryProfilePortForwardsGuestAccountFlag() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      UUID accountId = UUID.randomUUID();
      fake.user.setEnsuredProfile(profile(accountId, UUID.randomUUID(), true, false));
      runner(fake).run(context -> {
        assertThat(context).hasSingleBean(PrimaryProfileProvisioner.class);
        PrimaryProfileProvisioner port = context.getBean(PrimaryProfileProvisioner.class);
        port.ensurePrimaryProfile(accountId, "Guest", true);
        assertThat(fake.user.ensureRequests()).singleElement().satisfies(request -> {
          assertThat(request.getAccountId()).isEqualTo(accountId.toString());
          assertThat(request.getDisplayHint()).isEqualTo("Guest");
          assertThat(request.getIsGuestAccount()).isTrue();
        });
      });
    }
  }

  @Test
  void ensurePrimaryProfileRejectsEmptyMismatchedNonPrimaryFrozenAndUnavailableUserResults() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      UUID accountId = UUID.randomUUID();
      fake.user.setEnsureOutcome(RecordingUserGrpcService.Outcome.MALFORMED);
      runner(fake).run(context -> {
        assertThat(context).hasSingleBean(PrimaryProfileProvisioner.class);
        PrimaryProfileProvisioner port = context.getBean(PrimaryProfileProvisioner.class);
        assertThatThrownBy(() -> port.ensurePrimaryProfile(accountId, "x", false))
            .isInstanceOf(RuntimeException.class);

        fake.user.setEnsureOutcome(RecordingUserGrpcService.Outcome.SUCCESS);
        for (Profile invalid : List.of(
            profile(UUID.randomUUID(), UUID.randomUUID(), true, false),
            profile(accountId, UUID.randomUUID(), false, false),
            profile(accountId, UUID.randomUUID(), true, true),
            Profile.newBuilder().setId("not-a-uuid").setAccountId(accountId.toString())
                .setIsPrimary(true).build())) {
          fake.user.setEnsuredProfile(invalid);
          assertThatThrownBy(() -> port.ensurePrimaryProfile(accountId, "x", false))
              .isInstanceOf(RuntimeException.class);
        }

        for (RecordingUserGrpcService.Outcome failure : List.of(
            RecordingUserGrpcService.Outcome.FAILED_PRECONDITION,
            RecordingUserGrpcService.Outcome.UNAVAILABLE)) {
          fake.user.setEnsureOutcome(failure);
          assertThatThrownBy(() -> port.ensurePrimaryProfile(accountId, "x", false))
              .isInstanceOf(RuntimeException.class);
        }
      });
    }
  }

  @Test
  void ensurePrimaryProfileTreatsDeletedOrMissingPrimaryAsNotFound() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      UUID accountId = UUID.randomUUID();
      // The existing contract omits deleted/no-primary profiles and reports NOT_FOUND; it does not
      // expose a deleted flag. Keep this mapping explicit instead of inventing a wire field.
      fake.user.setEnsureOutcome(RecordingUserGrpcService.Outcome.NOT_FOUND);
      runner(fake).run(context -> {
        assertThat(context).hasSingleBean(PrimaryProfileProvisioner.class);
        PrimaryProfileProvisioner port = context.getBean(PrimaryProfileProvisioner.class);
        assertThatThrownBy(() -> port.ensurePrimaryProfile(accountId, "deleted@example.com", false))
            .isInstanceOf(RuntimeException.class);
        assertThat(fake.user.ensureRequests()).singleElement()
            .extracting(request -> request.getAccountId()).isEqualTo(accountId.toString());
      });
    }
  }

  @Test
  void profileSwitchPortCallsUserWithAuthenticatedAccountAndFailsClosed() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      UUID accountId = UUID.randomUUID();
      UUID currentProfileId = UUID.randomUUID();
      UUID profileId = UUID.randomUUID();
      fake.user.setSwitchedProfile(profile(accountId, profileId, false, false));
      runner(fake).run(context -> {
        assertThat(context).hasSingleBean(ProfileSwitchValidator.class);
        ProfileSwitchValidator port = context.getBean(ProfileSwitchValidator.class);
        port.validateOwnedSwitchable(accountId, currentProfileId, profileId, "premium");
        assertThat(fake.user.switchRequests()).singleElement()
            .extracting(request -> request.getProfileId()).isEqualTo(profileId.toString());
        assertThat(fake.metadata.userIds).contains(accountId.toString());
        assertThat(fake.metadata.profileIds).contains(currentProfileId.toString());
        assertThat(fake.metadata.subscriptionTiers).contains("premium");

        for (Profile invalid : List.of(
            profile(UUID.randomUUID(), profileId, false, false),
            profile(accountId, UUID.randomUUID(), false, false),
            profile(accountId, profileId, false, true),
            Profile.getDefaultInstance())) {
          fake.user.setSwitchedProfile(invalid);
          assertThatThrownBy(() -> port.validateOwnedSwitchable(accountId, profileId))
              .isInstanceOf(RuntimeException.class);
        }
        // User maps deleted/missing switch targets to NOT_FOUND; no deleted field exists on Profile.
        for (RecordingUserGrpcService.Outcome failure : List.of(
            RecordingUserGrpcService.Outcome.NOT_FOUND,
            RecordingUserGrpcService.Outcome.FAILED_PRECONDITION,
            RecordingUserGrpcService.Outcome.UNAVAILABLE,
            RecordingUserGrpcService.Outcome.MALFORMED)) {
          fake.user.setSwitchOutcome(failure);
          assertThatThrownBy(() -> port.validateOwnedSwitchable(accountId, profileId))
              .isInstanceOf(RuntimeException.class);
        }
      });
    }
  }

  @Test
  void phoneResolverSendsOnlyAuthOwnedAccountIdsAndOmitsUnresolvedProfiles() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      UUID profileId = UUID.randomUUID();
      runner(fake).run(context -> {
        InMemoryAccountRepository accounts = context.getBean(InMemoryAccountRepository.class);
        var account = accounts.create("phone@example.com", "auth-owned-hash", "hash", "regular");
        fake.user.resolvedPrimaryProfileIds().clear();
        fake.user.resolvedPrimaryProfileIds().put(account.id().toString(), profileId.toString());
        assertThat(context).hasSingleBean(PhoneHashResolver.class);
        PhoneHashResolver port = context.getBean(PhoneHashResolver.class);
        assertThat(port.resolvePrimaryProfileIdsByPhoneHashes(List.of("auth-owned-hash", "missing")))
            .containsEntry("auth-owned-hash", profileId.toString());
        assertThat(fake.user.resolveRequests()).singleElement().satisfies(request -> {
          assertThat(request.getAccountIdsList()).containsOnly(account.id().toString());
          assertThat(request.getAccountIdsList()).doesNotContain("auth-owned-hash");
        });

        fake.user.resolvedPrimaryProfileIds().clear();
        assertThat(port.resolvePrimaryProfileIdsByPhoneHashes(List.of("auth-owned-hash"))).isEmpty();

        fake.user.resolvedPrimaryProfileIds().put(account.id().toString(), "not-a-profile-id");
        assertThatThrownBy(
                () -> port.resolvePrimaryProfileIdsByPhoneHashes(List.of("auth-owned-hash")))
            .isInstanceOf(RuntimeException.class);

        fake.user.setResolveOutcome(RecordingUserGrpcService.Outcome.UNAVAILABLE);
        assertThatThrownBy(
                () -> port.resolvePrimaryProfileIdsByPhoneHashes(List.of("auth-owned-hash")))
            .isInstanceOf(RuntimeException.class);
      });
    }
  }

  @Test
  void markAccountRegularPortUsesExistingRpcAndPropagatesUserFailure() throws Exception {
    try (TcpFake fake = TcpFake.start()) {
      UUID accountId = UUID.randomUUID();
      runner(fake).run(context -> {
        assertThat(context).hasSingleBean(PrimaryProfileProvisioner.class);
        PrimaryProfileProvisioner port = context.getBean(PrimaryProfileProvisioner.class);
        port.clearGuestAccountFlag(accountId);
        assertThat(fake.user.markRegularRequests()).singleElement()
            .extracting(request -> request.getAccountId()).isEqualTo(accountId.toString());
        fake.user.setMarkRegularOutcome(RecordingUserGrpcService.Outcome.UNAVAILABLE);
        assertThatThrownBy(() -> port.clearGuestAccountFlag(accountId))
            .isInstanceOf(RuntimeException.class);
      });
    }
  }

  private void invokeEveryAuthUserRpc(TcpFake fake, String deadlineProperty) {
    UUID accountId = UUID.randomUUID();
    UUID profileId = UUID.randomUUID();
    fake.user.setEnsuredProfile(profile(accountId, profileId, true, false));
    fake.user.setSwitchedProfile(profile(accountId, profileId, false, false));
    String addressProperty = "auth.user-grpc.addr=localhost:" + fake.server.getPort();
    String[] properties = deadlineProperty == null
        ? new String[] {addressProperty}
        : new String[] {addressProperty, deadlineProperty};
    contextRunner.withPropertyValues(properties).run(context -> {
          assertThat(context).hasNotFailed();
          InMemoryAccountRepository accounts = context.getBean(InMemoryAccountRepository.class);
          var account = accounts.create("deadline@example.com", "deadline-hash", "hash", "regular");
          fake.user.resolvedPrimaryProfileIds().put(account.id().toString(), profileId.toString());

          context.getBean(PrimaryProfileProvisioner.class)
              .ensurePrimaryProfile(accountId, "deadline@example.com", false);
          context.getBean(PhoneHashResolver.class)
              .resolvePrimaryProfileIdsByPhoneHashes(List.of("deadline-hash"));
          context.getBean(ProfileSwitchValidator.class)
              .validateOwnedSwitchable(accountId, profileId);
          UserVerificationSync verification = context.getBean(UserVerificationSync.class);
          verification.setPersonalVerification(profileId, "verified");
          verification.clearVerification(profileId);
          context.getBean(PrimaryProfileProvisioner.class).clearGuestAccountFlag(accountId);
        });
  }

  private static Stream<Arguments> invalidUserGrpcDeadlines() {
    return Stream.of(
        Arguments.of(""),
        Arguments.of("not-a-duration"),
        Arguments.of("PT0S"),
        Arguments.of("PT-1S"));
  }

  private static Stream<Arguments> ensurePrimaryProfileFailures() {
    return Stream.of(
        Arguments.of(RecordingUserGrpcService.Outcome.INVALID_ARGUMENT, "auth_unavailable"),
        Arguments.of(RecordingUserGrpcService.Outcome.PERMISSION_DENIED, "auth_unavailable"),
        Arguments.of(RecordingUserGrpcService.Outcome.DEADLINE_EXCEEDED, "auth_unavailable"),
        Arguments.of(RecordingUserGrpcService.Outcome.MALFORMED, "malformed_user_response"));
  }

  private static Stream<Arguments> switchProfileFailures() {
    return Stream.of(
        Arguments.of(
            RecordingUserGrpcService.Outcome.NOT_FOUND,
            ProfileSwitchException.class,
            "profile_not_found",
            ProfileSwitchException.Kind.NOT_FOUND),
        Arguments.of(
            RecordingUserGrpcService.Outcome.PERMISSION_DENIED,
            ProfileSwitchException.class,
            "profile_forbidden",
            ProfileSwitchException.Kind.FORBIDDEN),
        Arguments.of(
            RecordingUserGrpcService.Outcome.UNAUTHENTICATED,
            ProfileSwitchException.class,
            "profile_forbidden",
            ProfileSwitchException.Kind.FORBIDDEN),
        Arguments.of(
            RecordingUserGrpcService.Outcome.FAILED_PRECONDITION,
            ProfileSwitchException.class,
            "profile_frozen",
            ProfileSwitchException.Kind.PRECONDITION),
        Arguments.of(
            RecordingUserGrpcService.Outcome.DEADLINE_EXCEEDED,
            AuthException.class,
            "auth_unavailable",
            null),
        Arguments.of(
            RecordingUserGrpcService.Outcome.MALFORMED,
            ProfileSwitchException.class,
            "malformed_user_response",
            ProfileSwitchException.Kind.PRECONDITION));
  }

  private ApplicationContextRunner runner(TcpFake fake) {
    return contextRunner.withPropertyValues("auth.user-grpc.addr=localhost:" + fake.server.getPort());
  }

  private static ApplicationContextRunner jdbcRuntimeContext() {
    return new ApplicationContextRunner()
        .withConfiguration(AutoConfigurations.of(
            UserGrpcClientConfiguration.class,
            JdbcPersistenceConfiguration.class,
            PrimaryProfileBeansConfiguration.class,
            JdbcRuntimePropertiesConfiguration.class));
  }

  private static Profile profile(UUID accountId, UUID profileId, boolean primary, boolean frozen) {
    Profile.Builder profile = Profile.newBuilder().setId(profileId.toString()).setAccountId(accountId.toString())
        .setIsPrimary(primary).setCreatedAt(Timestamp.getDefaultInstance()).setUpdatedAt(Timestamp.getDefaultInstance());
    if (frozen) profile.setFrozenAt(Timestamp.getDefaultInstance());
    return profile.build();
  }

  @Configuration(proxyBeanMethods = false)
  @EnableConfigurationProperties(AuthProperties.class)
  static class PropertiesConfiguration {
    @Bean
    InMemoryAccountRepository accountRepository() {
      return new InMemoryAccountRepository();
    }
  }

  @Configuration(proxyBeanMethods = false)
  @EnableConfigurationProperties(AuthProperties.class)
  static class JdbcRuntimePropertiesConfiguration {
    @Bean
    DataSource dataSource() {
      return org.mockito.Mockito.mock(DataSource.class);
    }

    @Bean
    NamedParameterJdbcTemplate authJdbc(DataSource dataSource) {
      return new NamedParameterJdbcTemplate(dataSource);
    }

    @Bean
    StringRedisTemplate stringRedisTemplate() {
      return org.mockito.Mockito.mock(StringRedisTemplate.class);
    }
  }

  private static final class TcpFake implements AutoCloseable {
    final RecordingUserGrpcService user;
    final MetadataCapture metadata;
    final DeadlineCapture deadlines;
    final Server server;

    private TcpFake(
        RecordingUserGrpcService user, MetadataCapture metadata, DeadlineCapture deadlines, Server server) {
      this.user = user;
      this.metadata = metadata;
      this.deadlines = deadlines;
      this.server = server;
    }
    static TcpFake start() throws Exception {
      RecordingUserGrpcService user = new RecordingUserGrpcService();
      MetadataCapture metadata = new MetadataCapture();
      DeadlineCapture deadlines = new DeadlineCapture();
      Server server = ServerBuilder.forPort(0)
          .addService(ServerInterceptors.intercept(user, metadata, deadlines)).build().start();
      return new TcpFake(user, metadata, deadlines, server);
    }
    @Override public void close() { server.shutdownNow(); }
  }

  private static final class DeadlineCapture implements ServerInterceptor {
    private static final List<String> AUTH_TO_USER_METHODS = List.of(
        "EnsurePrimaryProfile",
        "ResolvePrimaryProfileIDs",
        "SwitchProfile",
        "SetVerification",
        "ClearVerification",
        "MarkAccountRegular");
    final List<DeadlineObservation> observations = new ArrayList<>();

    @Override
    public <ReqT, RespT> Listener<ReqT> interceptCall(
        ServerCall<ReqT, RespT> call, Metadata headers, ServerCallHandler<ReqT, RespT> next) {
      var deadline = Context.current().getDeadline();
      observations.add(
          new DeadlineObservation(
              call.getMethodDescriptor().getBareMethodName(),
              deadline == null ? null : deadline.timeRemaining(TimeUnit.MILLISECONDS)));
      return next.startCall(call, headers);
    }

    void assertEveryAuthUserRpcIsBoundedBy(Duration expected) {
      assertThat(observations)
          .extracting(DeadlineObservation::method)
          .containsExactlyInAnyOrderElementsOf(AUTH_TO_USER_METHODS);
      assertThat(observations).allSatisfy(observation -> {
        assertThat(observation.remainingMillis())
            .as("%s must receive a gRPC deadline", observation.method())
            .isNotNull()
            .isPositive()
            .isLessThanOrEqualTo(expected.toMillis());
      });
    }
  }

  private record DeadlineObservation(String method, Long remainingMillis) {}

  private static final class MetadataCapture implements ServerInterceptor {
    private static final Metadata.Key<String> INTERNAL_CALLER =
        Metadata.Key.of("x-voice-internal-caller", Metadata.ASCII_STRING_MARSHALLER);
    final List<String> internalCallers = new ArrayList<>();
    private static final Metadata.Key<String> USER_ID =
        Metadata.Key.of("x-voice-user-id", Metadata.ASCII_STRING_MARSHALLER);
    final List<String> userIds = new ArrayList<>();
    private static final Metadata.Key<String> PROFILE_ID =
        Metadata.Key.of("x-voice-profile-id", Metadata.ASCII_STRING_MARSHALLER);
    final List<String> profileIds = new ArrayList<>();
    private static final Metadata.Key<String> SUBSCRIPTION_TIER =
        Metadata.Key.of("x-voice-subscription-tier", Metadata.ASCII_STRING_MARSHALLER);
    final List<String> subscriptionTiers = new ArrayList<>();

    @Override
    public <ReqT, RespT> Listener<ReqT> interceptCall(
        ServerCall<ReqT, RespT> call, Metadata headers, ServerCallHandler<ReqT, RespT> next) {
      internalCallers.add(headers.get(INTERNAL_CALLER));
      userIds.add(headers.get(USER_ID));
      profileIds.add(headers.get(PROFILE_ID));
      subscriptionTiers.add(headers.get(SUBSCRIPTION_TIER));
      return next.startCall(call, headers);
    }
  }
}
