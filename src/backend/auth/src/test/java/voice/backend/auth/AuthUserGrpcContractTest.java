package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import app.voice.user.v1.Profile;
import com.google.protobuf.Timestamp;
import com.nimbusds.jwt.SignedJWT;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryOtpCodeRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.sessionepoch.InMemorySessionEpochFloorStore;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.service.AccountRestoreTokenStore;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.AuthSession;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.ConvertGuestCommand;
import voice.backend.auth.service.InMemoryAccountRestoreTokenStore;
import voice.backend.auth.service.InMemorySubscriptionTierStore;
import voice.backend.auth.service.LoginCommand;
import voice.backend.auth.service.InMemoryOtpThrottle;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.service.ProfileSwitchException;
import voice.backend.auth.service.RefreshCommand;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.service.VerifyOtpCommand;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.support.RecordingUserGrpcService;
import voice.backend.auth.userdb.GrpcUserVerificationSync;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

/** Contract boundary for the already-shipped User verification RPCs. */
class AuthUserGrpcContractTest {
  private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-09-04T12:00:00Z"), ZoneOffset.UTC);

  @Test
  void emailGuestLoginAndRefreshProvisionAUserProfileBeforeEachSession() throws Exception {
    Harness email = new Harness(UUID.randomUUID().toString());
    AuthSession registered = email.service.register(
        new RegisterCommand("contract@example.com", null, "Correct horse battery staple", false, "{}"));
    AuthSession loggedIn = email.service.login(
        new LoginCommand("contract@example.com", null, "Correct horse battery staple", null, "{}"));
    AuthSession refreshed = email.service.refresh(new RefreshCommand(loggedIn.refreshToken(), "{}"));

    assertThat(email.profiles.ensureAccountIds).containsOnly(
        UUID.fromString(registered.accountId()), UUID.fromString(registered.accountId()), UUID.fromString(registered.accountId()));
    assertThat(registered.profileId()).isEqualTo(email.profiles.profileId);
    assertThat(loggedIn.profileId()).isEqualTo(email.profiles.profileId);
    assertThat(refreshed.profileId()).isEqualTo(email.profiles.profileId);
    assertJwtProfile(registered, email.profiles.profileId);
    assertJwtProfile(loggedIn, email.profiles.profileId);
    assertJwtProfile(refreshed, email.profiles.profileId);

    Harness guest = new Harness(UUID.randomUUID().toString());
    AuthSession guestRegistered = guest.service.register(
        new RegisterCommand(null, null, "Correct horse battery staple", true, "{}"));
    AuthSession guestRefreshed =
        guest.service.refresh(new RefreshCommand(guestRegistered.refreshToken(), "{}"));
    assertThat(guest.profiles.guestFlags).containsExactly(true, true);
    assertThat(guestRegistered.profileId()).isEqualTo(guest.profiles.profileId);
    assertThat(guestRefreshed.profileId()).isEqualTo(guest.profiles.profileId);
    assertJwtProfile(guestRegistered, guest.profiles.profileId);
    assertJwtProfile(guestRefreshed, guest.profiles.profileId);
  }

  @Test
  void unavailableOrMalformedPrimaryProfilePreventsRefreshTokenIssue() {
    Harness unavailable = new Harness(UUID.randomUUID().toString());
    unavailable.profiles.failure = new AuthException("auth_unavailable");
    assertThatThrownBy(() -> unavailable.service.register(
        new RegisterCommand("unavailable@example.com", null, "Correct horse battery staple", false, "{}")))
        .isInstanceOf(AuthException.class).hasMessage("auth_unavailable");
    assertThat(unavailable.refreshTokens.listActiveByAccount(UUID.fromString(unavailable.accountId())))
        .isEmpty();

    Harness malformed = new Harness("not-a-user-profile-id");
    assertThatThrownBy(() -> malformed.service.register(
        new RegisterCommand("malformed@example.com", null, "Correct horse battery staple", false, "{}")))
        .isInstanceOf(AuthException.class);
    assertThat(malformed.refreshTokens.listActiveByAccount(UUID.fromString(malformed.accountId())))
        .isEmpty();

    Harness login = new Harness(UUID.randomUUID().toString());
    AuthSession original = login.service.register(
        new RegisterCommand("login-user-down@example.com", null,
            "Correct horse battery staple", false, "{}"));
    int sessionsBeforeLoginFailure =
        login.refreshTokens.listActiveByAccount(UUID.fromString(original.accountId())).size();
    login.profiles.failure = new AuthException("auth_unavailable");
    assertThatThrownBy(() -> login.service.login(
        new LoginCommand("login-user-down@example.com", null,
            "Correct horse battery staple", null, "{}")))
        .isInstanceOf(AuthException.class).hasMessage("auth_unavailable");
    assertThat(login.refreshTokens.listActiveByAccount(UUID.fromString(original.accountId())))
        .hasSize(sessionsBeforeLoginFailure);

    login.profiles.failure = null;
    AuthSession refreshCandidate = login.service.login(
        new LoginCommand("login-user-down@example.com", null,
            "Correct horse battery staple", null, "{}"));
    login.profiles.failure = new AuthException("auth_unavailable");
    assertThatThrownBy(() -> login.service.refresh(
        new RefreshCommand(refreshCandidate.refreshToken(), "{}")))
        .isInstanceOf(AuthException.class).hasMessage("auth_unavailable");
    assertThat(login.refreshTokens.listActiveByAccount(UUID.fromString(original.accountId())))
        .noneMatch(record -> !record.revoked()
            && record.tokenHash().equals(new RefreshTokenCodec().hash(refreshCandidate.refreshToken())));
  }

  @Test
  void oauthTokenExchangeRechecksCanonicalUserProfileAndFailsClosed() throws Exception {
    Harness harness = new Harness(UUID.randomUUID().toString());
    AuthSession registered = harness.service.register(
        new RegisterCommand(
            "oauth-contract@example.com",
            null,
            "Correct horse battery staple",
            false,
            "{}"));

    String oauthToken =
        harness.service.issueOAuthAccessToken(registered.accountId(), registered.profileId());
    assertThat(SignedJWT.parse(oauthToken).getJWTClaimsSet().getStringClaim("profile_id"))
        .isEqualTo(registered.profileId());
    assertThat(harness.profiles.ensureAccountIds).hasSize(2);

    harness.profiles.failure = new AuthException("auth_unavailable");
    assertThatThrownBy(
            () ->
                harness.service.issueOAuthAccessToken(
                    registered.accountId(), registered.profileId()))
        .isInstanceOf(AuthException.class)
        .hasMessage("auth_unavailable");
  }

  @Test
  void convertGuestSubmitRemainsPendingAndDoesNotPromoteOrPublish() {
    Harness harness = new Harness(UUID.randomUUID().toString());
    AuthSession guest = harness.service.register(
        new RegisterCommand(null, null, "Correct horse battery staple", true, "{}"));

    AuthSession pending = harness.service.convertGuest(
        guest.accessToken(), new ConvertGuestCommand("pending@example.com", null, "New account password 1"));

    assertThat(pending.accountType()).isEqualTo("guest");
    assertThat(harness.profiles.clearGuestAccountCalls).isZero();
    assertThat(harness.events.guestConvertedAccountIds).isEmpty();
  }

  @Test
  void successfulEmailVerificationDelegatesToDurableAcceptanceWithoutDirectUserOrEventContinuation() {
    Harness harness = new Harness(UUID.randomUUID().toString());
    AuthSession guest = harness.service.register(
        new RegisterCommand(null, null, "Correct horse battery staple", true, "{}"));
    harness.service.convertGuest(
        guest.accessToken(), new ConvertGuestCommand("verified@example.com", null, "New account password 1"));
    harness.profiles.clearGuestAccountCalls = 0;
    harness.events.guestConvertedAccountIds.clear();
    harness.acceptanceCalls.clear();
    harness.order.clear();

    harness.verifyEmailOtp("verified@example.com", "123456");

    assertThat(harness.acceptanceCalls).containsExactly(UUID.fromString(guest.accountId()));
    assertThat(harness.profiles.clearGuestAccountCalls).isZero();
    assertThat(harness.events.guestConvertedAccountIds).isEmpty();
    assertThat(harness.order).isEmpty();
  }

  @Test
  void emailVerificationDoesNotInvokeUserPromotionOnTheRequestPath() {
    Harness harness = new Harness(UUID.randomUUID().toString());
    AuthSession guest = harness.service.register(
        new RegisterCommand(null, null, "Correct horse battery staple", true, "{}"));
    harness.service.convertGuest(
        guest.accessToken(), new ConvertGuestCommand("promotion-failure@example.com", null, "New account password 1"));
    harness.profiles.clearGuestAccountCalls = 0;
    harness.events.guestConvertedAccountIds.clear();
    harness.acceptanceCalls.clear();
    harness.profiles.promotionFailure = new AuthException("auth_unavailable");
    UUID accountId = UUID.fromString(guest.accountId());
    var sessionsBeforeFailure = List.copyOf(harness.refreshTokens.listActiveByAccount(accountId));

    harness.verifyEmailOtp("promotion-failure@example.com", "123456");

    assertThat(harness.acceptanceCalls).containsExactly(accountId);
    assertThat(harness.profiles.clearGuestAccountCalls).isZero();
    assertThat(harness.events.guestConvertedAccountIds).isEmpty();
    assertThat(harness.refreshTokens.listActiveByAccount(accountId)).isEqualTo(sessionsBeforeFailure);
    assertThat(harness.accounts.findByEmail("promotion-failure@example.com")).get()
        .extracting(account -> account.type()).isEqualTo("guest");
  }

  private static void assertJwtProfile(AuthSession session, String profileId) throws Exception {
    assertThat(SignedJWT.parse(session.accessToken()).getJWTClaimsSet().getStringClaim("profile_id"))
        .isEqualTo(profileId);
  }

  @Test
  void switchOnlyIssuesReplacementSessionForOwnedUsableProfile() {
    Harness success = new Harness(UUID.randomUUID().toString());
    AuthSession session = success.service.register(
        new RegisterCommand("switch@example.com", null, "Correct horse battery staple", false, "{}"));
    UUID target = UUID.randomUUID();

    AuthSession switched = success.service.switchActiveProfile(session.accessToken(), target.toString(), "{}");

    assertThat(switched.profileId()).isEqualTo(target.toString());
    assertThat(success.switches.calls).containsExactly(target);

    for (String failure : List.of("profile_not_found", "profile_forbidden", "profile_deleted", "profile_frozen", "auth_unavailable", "malformed_user_response")) {
      Harness rejected = new Harness(UUID.randomUUID().toString());
      AuthSession original = rejected.service.register(
          new RegisterCommand(failure + "@example.com", null, "Correct horse battery staple", false, "{}"));
      rejected.switches.failure = new ProfileSwitchException(failure, ProfileSwitchException.Kind.PRECONDITION);

      assertThatThrownBy(() -> rejected.service.switchActiveProfile(original.accessToken(), UUID.randomUUID().toString(), "{}"))
          .isInstanceOf(ProfileSwitchException.class).hasMessage(failure);
      assertThat(rejected.refreshTokens.listActiveByAccount(UUID.fromString(original.accountId()))).hasSize(1);
    }
  }

  @Test
  void phoneResolutionReturnsOnlyUsableUserMappingsAndFailsClosedOnBadUserOutput() {
    Harness success = new Harness(UUID.randomUUID().toString());
    String accountId = UUID.randomUUID().toString();
    String profileId = UUID.randomUUID().toString();
    success.phone.mappings = Map.of("auth-owned-hash", profileId);

    assertThat(success.service.resolvePhoneHashes(List.of("auth-owned-hash", "missing")))
        .containsOnly(Map.entry("auth-owned-hash", profileId));
    assertThat(success.phone.requests).containsExactly(List.of("auth-owned-hash", "missing"));

    Harness unavailable = new Harness(UUID.randomUUID().toString());
    unavailable.phone.failure = new AuthException("auth_unavailable");
    assertThatThrownBy(() -> unavailable.service.resolvePhoneHashes(List.of("auth-owned-hash")))
        .isInstanceOf(AuthException.class).hasMessage("auth_unavailable");

    Harness malformed = new Harness(UUID.randomUUID().toString());
    malformed.phone.mappings = Map.of("auth-owned-hash", "not-a-profile-id");
    assertThatThrownBy(() -> malformed.service.resolvePhoneHashes(List.of("auth-owned-hash")))
        .isInstanceOf(AuthException.class);
  }

  @Test
  void setAndClearVerificationUseAuthProfileIdAndDocumentedValues() throws Exception {
    RecordingUserGrpcService user = new RecordingUserGrpcService();
    try (UserServer server = UserServer.start(user)) {
      UUID profileId = UUID.randomUUID();
      GrpcUserVerificationSync sync = new GrpcUserVerificationSync(server.channel());

      sync.setPersonalVerification(profileId, "twitch");
      sync.clearVerification(profileId);

      assertThat(user.setVerificationRequests()).singleElement().satisfies(request -> {
        assertThat(request.getProfileId()).isEqualTo(profileId.toString());
        assertThat(request.getVerificationType()).isEqualTo("personal");
        assertThat(request.getBadge()).isEqualTo("twitch");
      });
      assertThat(user.clearVerificationRequests()).singleElement()
          .extracting(request -> request.getProfileId()).isEqualTo(profileId.toString());
    }
  }

  @Test
  void verificationUnavailableFailsClosed() throws Exception {
    RecordingUserGrpcService user = new RecordingUserGrpcService();
    user.setSetVerificationOutcome(RecordingUserGrpcService.Outcome.UNAVAILABLE);
    user.setClearVerificationOutcome(RecordingUserGrpcService.Outcome.UNAVAILABLE);
    try (UserServer server = UserServer.start(user)) {
      GrpcUserVerificationSync sync = new GrpcUserVerificationSync(server.channel());

      assertThatThrownBy(() -> sync.setPersonalVerification(UUID.randomUUID(), "twitch"))
          .isInstanceOf(AuthException.class).hasMessage("verification_sync_failed");
      assertThatThrownBy(() -> sync.clearVerification(UUID.randomUUID()))
          .isInstanceOf(AuthException.class).hasMessage("verification_sync_failed");
    }
  }

  @Test
  void verificationRejectsMalformedUserResponsesInsteadOfSilentlyAcceptingThem() throws Exception {
    RecordingUserGrpcService user = new RecordingUserGrpcService();
    user.setSetVerificationOutcome(RecordingUserGrpcService.Outcome.MALFORMED);
    user.setClearVerificationOutcome(RecordingUserGrpcService.Outcome.MALFORMED);
    try (UserServer server = UserServer.start(user)) {
      GrpcUserVerificationSync sync = new GrpcUserVerificationSync(server.channel());

      assertThatThrownBy(() -> sync.setPersonalVerification(UUID.randomUUID(), "twitch"))
          .isInstanceOf(AuthException.class).hasMessage("verification_sync_failed");
      assertThatThrownBy(() -> sync.clearVerification(UUID.randomUUID()))
          .isInstanceOf(AuthException.class).hasMessage("verification_sync_failed");
    }
  }

  @Test
  void fixtureSupportsOnlyCurrentPrimaryProfileAndPromotionContracts() throws Exception {
    UUID accountId = UUID.randomUUID();
    UUID profileId = UUID.randomUUID();
    RecordingUserGrpcService user = new RecordingUserGrpcService();
    user.setEnsuredProfile(primaryProfile(accountId, profileId));
    user.resolvedPrimaryProfileIds().put(accountId.toString(), profileId.toString());
    try (UserServer server = UserServer.start(user)) {
      var client = app.voice.user.v1.UserServiceGrpc.newBlockingStub(server.channel());
      var ensured = client.ensurePrimaryProfile(
          app.voice.user.v1.EnsurePrimaryProfileRequest.newBuilder()
              .setAccountId(accountId.toString()).setDisplayHint("test@example.com").build());
      var resolved = client.resolvePrimaryProfileIDs(
          app.voice.user.v1.ResolvePrimaryProfileIDsRequest.newBuilder().addAccountIds(accountId.toString()).build());
      client.markAccountRegular(
          app.voice.user.v1.MarkAccountRegularRequest.newBuilder().setAccountId(accountId.toString()).build());

      assertThat(ensured.getProfile().getId()).isEqualTo(profileId.toString());
      assertThat(resolved.getPrimaryProfileIdsMap()).containsEntry(accountId.toString(), profileId.toString());
      assertThat(user.markRegularRequests()).singleElement()
          .extracting(request -> request.getAccountId()).isEqualTo(accountId.toString());
    }
  }

  private static Profile primaryProfile(UUID accountId, UUID profileId) {
    return Profile.newBuilder().setId(profileId.toString()).setAccountId(accountId.toString())
        .setIsPrimary(true).setCreatedAt(Timestamp.getDefaultInstance())
        .setUpdatedAt(Timestamp.getDefaultInstance()).build();
  }

  static final class UserServer implements AutoCloseable {
    private final Server server;
    private final ManagedChannel channel;

    private UserServer(Server server, ManagedChannel channel) { this.server = server; this.channel = channel; }

    static UserServer start(RecordingUserGrpcService service) throws Exception {
      String name = InProcessServerBuilder.generateName();
      Server server = InProcessServerBuilder.forName(name).directExecutor().addService(service).build().start();
      return new UserServer(server, InProcessChannelBuilder.forName(name).directExecutor().build());
    }

    ManagedChannel channel() { return channel; }

    @Override public void close() {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }

  private static final class Harness {
    final InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    final InMemoryRefreshTokenRepository refreshTokens = new InMemoryRefreshTokenRepository();
    final RecordingProvisioner profiles;
    final RecordingPhoneResolver phone = new RecordingPhoneResolver();
    final RecordingSwitchValidator switches = new RecordingSwitchValidator();
    final List<String> order = new ArrayList<>();
    final List<UUID> acceptanceCalls = new ArrayList<>();
    final RecordingEvents events = new RecordingEvents(order);
    final AuthService service;

    Harness(String profileId) {
      profiles = new RecordingProvisioner(profileId, order);
      var props = new voice.backend.auth.config.AuthProperties();
      props.setPersistence(voice.backend.auth.config.AuthProperties.PersistenceMode.MEMORY);
      service = new AuthService(
          accounts, refreshTokens, new RefreshTokenCodec(), new BCryptPasswordHasher(),
          JwtService.forTests("voice-auth", "voice-client", "contract-key", Duration.ofMinutes(15), CLOCK),
          new InMemoryTokenBlacklist(CLOCK), new TotpService(props),
          new BackupCodeService(new InMemoryBackupCodeRepository()), CLOCK, Duration.ofDays(30), profiles,
          phone, new InMemorySubscriptionTierStore(), switches, new InMemoryE2EKeyBackupRepository(), events,
          new SimpleMeterRegistry(), new InMemoryAccountRestoreTokenStore(), new NoopMailSender(),
          new InMemorySessionEpochFloorStore());
    }

    String accountId() { return accounts.findByEmail("unavailable@example.com")
        .or(() -> accounts.findByEmail("malformed@example.com")).orElseThrow().id().toString(); }

    void verifyEmailOtp(String email, String code) {
      RefreshTokenCodec codec = new RefreshTokenCodec();
      InMemoryOtpCodeRepository codes = new InMemoryOtpCodeRepository();
      UUID accountId = accounts.findByEmail(email).orElseThrow().id();
      codes.create(accountId, codec.hash(code), "email_verify", Instant.now(CLOCK).plus(Duration.ofMinutes(10)), Instant.now(CLOCK));
      GuestConversionOtpAcceptance acceptance =
          (acceptedAccountId, ignoredOtp, ignoredNow) -> acceptanceCalls.add(acceptedAccountId);
      OtpService otp = new OtpService(accounts, codes, refreshTokens, codec, new BCryptPasswordHasher(),
          new NoopMailSender(), new InMemoryOtpThrottle(), CLOCK, acceptance);
      otp.verifyOtp(new VerifyOtpCommand(email, null, code, "email_verify", null), service);
    }
  }

  private static final class RecordingProvisioner implements PrimaryProfileProvisioner {
    final String profileId;
    final List<String> order;
    final List<UUID> ensureAccountIds = new ArrayList<>();
    final List<Boolean> guestFlags = new ArrayList<>();
    RuntimeException failure;
    RuntimeException promotionFailure;
    int clearGuestAccountCalls;

    RecordingProvisioner(String profileId, List<String> order) { this.profileId = profileId; this.order = order; }

    @Override public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
      ensureAccountIds.add(accountId);
      guestFlags.add(guestAccount);
      if (failure != null) throw failure;
      return profileId;
    }

    @Override public void clearGuestAccountFlag(UUID accountId) {
      clearGuestAccountCalls++;
      order.add("mark-account-regular");
      if (promotionFailure != null) throw promotionFailure;
    }
  }

  private static final class RecordingEvents implements AuthEventPublisher {
    final List<String> order;
    final List<UUID> guestConvertedAccountIds = new ArrayList<>();
    RecordingEvents(List<String> order) { this.order = order; }
    @Override public void publishGuestConverted(UUID accountId) { guestConvertedAccountIds.add(accountId); order.add("guest-converted-event"); }
    @Override public void publishAccountDeleted(UUID accountId) {}
    @Override public void publishAccountRestored(UUID accountId) {}
  }

  private static final class RecordingPhoneResolver implements PhoneHashResolver {
    final List<List<String>> requests = new ArrayList<>();
    Map<String, String> mappings = Map.of();
    RuntimeException failure;

    @Override public Map<String, String> resolvePrimaryProfileIdsByPhoneHashes(java.util.Collection<String> hashes) {
      requests.add(List.copyOf(hashes));
      if (failure != null) throw failure;
      return mappings;
    }
  }

  private static final class RecordingSwitchValidator implements voice.backend.auth.userdb.ProfileSwitchValidator {
    final List<UUID> calls = new ArrayList<>();
    RuntimeException failure;

    @Override public void validateOwnedSwitchable(UUID accountId, UUID profileId) {
      calls.add(profileId);
      if (failure != null) throw failure;
    }
  }
}
