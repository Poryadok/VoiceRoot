package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.DeleteAccountRequest;
import app.voice.auth.v1.Enable2FARequest;
import app.voice.auth.v1.RegisterRequest;
import app.voice.auth.v1.Verify2FARequest;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import java.lang.reflect.Method;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.boot.test.mock.mockito.SpyBean;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.service.AccountDeletionEventPublisher;
import voice.backend.auth.service.AccountDeletionPendingEventWorker;
import voice.backend.auth.service.AccountDeletionPendingFloorWorker;
import voice.backend.auth.service.AccountDeletionRecoveryRunner;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.GuestConversionPublishAck;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.sessionepoch.SessionEpochFloorMissingException;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

/**
 * RED contract for docs/features/auth-and-contacts.md: deletion needs a second factor when
 * TOTP is enabled. The source checks deliberately compile before the generated proto changes.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class AccountDeletionTwoFactorRedTest {
  private static final String PASSWORD = "Correct horse battery staple";

  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired AuthService authService;
  @Autowired AuthGrpcService grpcService;
  @Autowired AccountRepository accounts;
  @Autowired AccountDeletionOperationRepository deletionOperations;
  @Autowired RefreshTokenRepository refreshTokens;
  @MockBean SessionEpochFloorStore sessionEpochFloors;
  @MockBean AuthEventPublisher authEventPublisher;
  @MockBean AccountDeletionEventPublisher deletionEventPublisher;
  @MockBean AccountDeletionRecoveryRunner deletionRecoveryRunner;
  @SpyBean TotpService totpService;
  @SpyBean BackupCodeService backupCodeService;
  @SpyBean AccountDeletionPendingFloorWorker deletionFloorWorker;
  @SpyBean AccountDeletionPendingEventWorker deletionEventWorker;

  @BeforeEach
  void setUpEpochFloor() {
    org.mockito.Mockito.reset(
        sessionEpochFloors, authEventPublisher, deletionEventPublisher, deletionRecoveryRunner);
    org.mockito.Mockito.clearInvocations(
        totpService, backupCodeService, deletionFloorWorker, deletionEventWorker);
    Map<UUID, Long> floors = new ConcurrentHashMap<>();
    org.mockito.Mockito.when(sessionEpochFloors.recordAtLeast(
            org.mockito.ArgumentMatchers.any(UUID.class), org.mockito.ArgumentMatchers.anyLong()))
        .thenAnswer(
            invocation ->
                floors.merge(invocation.getArgument(0), invocation.getArgument(1), Math::max));
    org.mockito.Mockito.when(sessionEpochFloors.requireFloor(org.mockito.ArgumentMatchers.any(UUID.class)))
        .thenAnswer(
            invocation -> {
              Long floor = floors.get(invocation.getArgument(0));
              if (floor == null) {
                throw new SessionEpochFloorMissingException("session epoch floor missing");
              }
              return floor;
            });
    org.mockito.Mockito.when(deletionEventPublisher.publishAccountDeleted(
            org.mockito.ArgumentMatchers.any(),
            org.mockito.ArgumentMatchers.any(UserStreamEvent.class),
            org.mockito.ArgumentMatchers.any()))
        .thenReturn(new GuestConversionPublishAck("user_events", 1L));
  }

  @Test
  void totpEnabledDeletionRejectsMissingFactorWithoutAnyDeletionSideEffect() throws Exception {
    EnrolledAccount account = registerAndEnableTotp("delete-2fa-missing@voice-qa.test");
    prepareForDeletionAssertions();

    delete(account.accessToken(), "{\"password\":\"" + PASSWORD + "\"}")
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error").value("totp_required"));

    assertDeletionNeverStarted(account);
  }

  @Test
  void totpEnabledDeletionRejectsInvalidFactorWithoutAnyDeletionSideEffect() throws Exception {
    EnrolledAccount account = registerAndEnableTotp("delete-2fa-invalid@voice-qa.test");
    prepareForDeletionAssertions();

    delete(account.accessToken(), "{\"password\":\"" + PASSWORD + "\",\"totp_code\":\"invalid\"}")
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error").value("invalid_totp"));

    assertDeletionNeverStarted(account);
  }

  @Test
  void totpEnabledDeletionAcceptsTheExistingTestBypassCode() throws Exception {
    EnrolledAccount account = registerAndEnableTotp("delete-2fa-totp@voice-qa.test");
    prepareForDeletionAssertions();

    delete(account.accessToken(), "{\"password\":\"" + PASSWORD + "\",\"totp_code\":\"000000\"}")
        .andExpect(status().isNoContent());

    assertThat(accounts.findById(account.account().id().toString()).orElseThrow().status())
        .isEqualTo("deleted");
    org.mockito.Mockito.verify(totpService)
        .verifyEncrypted(account.account().totpSecret(), "000000");
  }

  @Test
  void totpEnabledDeletionAcceptsAOneTimeBackupCode() throws Exception {
    EnrolledAccount account = registerAndEnableTotp("delete-2fa-backup@voice-qa.test");
    prepareForDeletionAssertions();

    delete(
            account.accessToken(),
            "{\"password\":\"" + PASSWORD + "\",\"totp_code\":\"" + account.backupCode() + "\"}")
        .andExpect(status().isNoContent());

    assertThat(accounts.findById(account.account().id().toString()).orElseThrow().status())
        .isEqualTo("deleted");
    org.mockito.Mockito.verify(backupCodeService)
        .consume(account.account().id(), account.backupCode());
  }

  @Test
  void consumedBackupCodeCannotAuthorizeDeletionWhileTheAccountRemainsActive() throws Exception {
    EnrolledAccount account = registerAndEnableTotp("delete-2fa-backup-reuse@voice-qa.test");
    assertThat(backupCodeService.consume(account.account().id(), account.backupCode())).isTrue();
    prepareForDeletionAssertions();

    delete(
            account.accessToken(),
            "{\"password\":\"" + PASSWORD + "\",\"totp_code\":\"" + account.backupCode() + "\"}")
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error").value("invalid_totp"));

    assertDeletionNeverStarted(account);
    org.mockito.Mockito.verify(backupCodeService)
        .consume(account.account().id(), account.backupCode());
  }

  @Test
  void passwordOnlyDeletionRemainsCompatibleWhenTotpIsDisabled() throws Exception {
    JsonNode session = register("delete-password-only@voice-qa.test");
    Account account = accounts.findByEmail("delete-password-only@voice-qa.test").orElseThrow();

    delete(session.get("access_token").asText(), "{\"password\":\"" + PASSWORD + "\"}")
        .andExpect(status().isNoContent());

    assertThat(accounts.findById(account.id().toString()).orElseThrow().status()).isEqualTo("deleted");
  }

  @Test
  void grpcDeletionRejectsMissingFactorWithoutAnyDeletionSideEffect() throws Exception {
    try (GrpcFixture fixture = registerAndEnableTotpOverGrpc("grpc-delete-2fa-missing@voice-qa.test")) {
      prepareForDeletionAssertions();

      assertGrpcError(
          () -> fixture.client().deleteAccount(DeleteAccountRequest.newBuilder().setPassword(PASSWORD).build()),
          "totp_required");

      assertDeletionNeverStarted(fixture.account());
    }
  }

  @Test
  void grpcDeletionRejectsInvalidFactorWithoutAnyDeletionSideEffect() throws Exception {
    try (GrpcFixture fixture = registerAndEnableTotpOverGrpc("grpc-delete-2fa-invalid@voice-qa.test")) {
      prepareForDeletionAssertions();

      assertGrpcError(
          () -> fixture.client().deleteAccount(deleteRequestWithTotpCode("invalid")), "invalid_totp");

      assertDeletionNeverStarted(fixture.account());
    }
  }

  @Test
  void canonicalAndServiceProtoCopiesKeepAnOptionalSecondFactorAndTransportMappings()
      throws Exception {
    Path root = Path.of("").toAbsolutePath();
    String canonical = Files.readString(root.resolve("../../../protos/voice/auth/v1/auth.proto").normalize());
    String authCopy = Files.readString(root.resolve("src/main/proto/voice/auth/v1/auth.proto"));
    String rest = Files.readString(root.resolve("src/main/java/voice/backend/auth/rest/AuthRestController.java"));
    String grpc = Files.readString(root.resolve("src/main/java/voice/backend/auth/grpc/AuthGrpcService.java"));
    String expectedField = "optional string totp_code = 2;";

    assertThat(canonical).contains(expectedField);
    assertThat(authCopy).contains(expectedField);
    assertThat(rest)
        .contains("@JsonProperty(\"totp_code\") String totpCode")
        .contains("authService.deleteAccount(authorization, request.password(), request.totpCode())");
    assertThat(grpc)
        .contains("request.getTotpCode()")
        .contains("authService.deleteAccount(lastAccessToken(), request.getPassword(), request.getTotpCode())");
  }

  private EnrolledAccount registerAndEnableTotp(String email) throws Exception {
    JsonNode session = register(email);
    String accessToken = session.get("access_token").asText();
    MvcResult enrollment =
        mockMvc
            .perform(
                post("/api/v1/auth/2fa/enable")
                    .header("Authorization", "Bearer " + accessToken)
                    .contentType(MediaType.APPLICATION_JSON)
                    .content("{\"password\":\"" + PASSWORD + "\"}"))
            .andExpect(status().isOk())
            .andReturn();
    String backupCode =
        objectMapper.readTree(enrollment.getResponse().getContentAsString()).get("backup_codes").get(0).asText();
    mockMvc
        .perform(
            post("/api/v1/auth/2fa/verify")
                .header("Authorization", "Bearer " + accessToken)
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"totp_code\":\"000000\"}"))
        .andExpect(status().isOk());
    Account account = accounts.findByEmail(email).orElseThrow();
    return new EnrolledAccount(
        accessToken,
        backupCode,
        account,
        refreshTokens.listActiveByAccount(account.id()).size());
  }

  private GrpcFixture registerAndEnableTotpOverGrpc(String email) throws Exception {
    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName).directExecutor().addService(grpcService).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    var registered =
        client
            .register(RegisterRequest.newBuilder().setEmail(email).setPassword(PASSWORD).build())
            .getSession();
    client.enable2FA(Enable2FARequest.newBuilder().setPassword(PASSWORD).build());
    client.verify2FA(Verify2FARequest.newBuilder().setTotpCode("000000").build());
    Account account = accounts.findByEmail(email).orElseThrow();
    return new GrpcFixture(
        server,
        channel,
        client,
        new EnrolledAccount(
            registered.getAccessToken(),
            null,
            account,
            refreshTokens.listActiveByAccount(account.id()).size()));
  }

  private JsonNode register(String email) throws Exception {
    MvcResult result =
        mockMvc
            .perform(
                post("/api/v1/auth/register")
                    .contentType(MediaType.APPLICATION_JSON)
                    .content(
                        "{\"email\":\""
                            + email
                            + "\",\"password\":\""
                            + PASSWORD
                            + "\",\"device_info_json\":\"{}\"}"))
            .andExpect(status().isOk())
            .andReturn();
    return objectMapper.readTree(result.getResponse().getContentAsString()).get("session");
  }

  private org.springframework.test.web.servlet.ResultActions delete(String accessToken, String body)
      throws Exception {
    return mockMvc.perform(
        post("/api/v1/auth/delete-account")
            .header("Authorization", "Bearer " + accessToken)
            .contentType(MediaType.APPLICATION_JSON)
            .content(body));
  }

  private void prepareForDeletionAssertions() {
    org.mockito.Mockito.clearInvocations(
        sessionEpochFloors,
        authEventPublisher,
        deletionEventPublisher,
        deletionRecoveryRunner,
        totpService,
        backupCodeService,
        deletionFloorWorker,
        deletionEventWorker);
  }

  private void assertDeletionNeverStarted(EnrolledAccount enrolled) {
    Account account = enrolled.account();
    Account current = accounts.findById(account.id().toString()).orElseThrow();
    assertThat(current.status()).isEqualTo("active");
    assertThat(current.deletedAt()).isNull();
    assertThat(current.sessionEpoch()).isEqualTo(account.sessionEpoch());
    assertThat(deletionOperations.findByAccountAndEpoch(account.id(), account.sessionEpoch() + 1))
        .isEmpty();
    assertThat(refreshTokens.listActiveByAccount(account.id())).hasSize(enrolled.activeSessions());
    assertThat(authService.validate("Bearer " + enrolled.accessToken()).userId())
        .isEqualTo(account.id().toString());
    org.mockito.Mockito.verifyNoInteractions(
        authEventPublisher,
        deletionEventPublisher,
        sessionEpochFloors,
        deletionRecoveryRunner,
        deletionFloorWorker,
        deletionEventWorker);
  }

  private static DeleteAccountRequest deleteRequestWithTotpCode(String code) {
    try {
      Object builder = DeleteAccountRequest.newBuilder().setPassword(PASSWORD);
      Method setTotpCode = builder.getClass().getMethod("setTotpCode", String.class);
      Object configured = setTotpCode.invoke(builder, code);
      return (DeleteAccountRequest) configured.getClass().getMethod("build").invoke(configured);
    } catch (ReflectiveOperationException ex) {
      throw new AssertionError("DeleteAccountRequest must expose optional totp_code", ex);
    }
  }

  private static void assertGrpcError(Runnable call, String expectedDescription) {
    assertThatThrownBy(call::run)
        .isInstanceOf(StatusRuntimeException.class)
        .satisfies(
            failure -> {
              StatusRuntimeException grpc = (StatusRuntimeException) failure;
              assertThat(grpc.getStatus().getCode()).isEqualTo(Status.Code.UNAUTHENTICATED);
              assertThat(grpc.getStatus().getDescription()).isEqualTo(expectedDescription);
            });
  }

  private record EnrolledAccount(
      String accessToken, String backupCode, Account account, int activeSessions) {}

  private record GrpcFixture(
      Server server,
      ManagedChannel channel,
      AuthServiceGrpc.AuthServiceBlockingStub client,
      EnrolledAccount account)
      implements AutoCloseable {
    @Override
    public void close() {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }
}
