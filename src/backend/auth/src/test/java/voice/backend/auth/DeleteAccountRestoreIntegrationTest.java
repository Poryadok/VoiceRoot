package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import org.mockito.ArgumentCaptor;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.AccountDeletionEventPublisher;
import voice.backend.auth.service.AccountDeletionRecoveryRunner;
import voice.backend.auth.service.DeleteAccountResult;
import voice.backend.auth.service.GuestConversionPublishAck;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException;
import voice.backend.auth.sessionepoch.SessionEpochFloorMissingException;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class DeleteAccountRestoreIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired AuthService authService;
  @Autowired AccountRepository accounts;
  @Autowired AccountDeletionOperationRepository deletionOperations;
  @MockBean SessionEpochFloorStore sessionEpochFloors;
  @MockBean AuthEventPublisher authEventPublisher;
  @MockBean AccountDeletionEventPublisher deletionEventPublisher;
  @MockBean AccountDeletionRecoveryRunner deletionRecoveryRunner;

  @BeforeEach
  void setUpEpochFloor() {
    Map<UUID, Long> floors = new ConcurrentHashMap<>();
    when(sessionEpochFloors.recordAtLeast(any(UUID.class), anyLong()))
        .thenAnswer(
            invocation ->
                floors.merge(invocation.getArgument(0), invocation.getArgument(1), Math::max));
    when(sessionEpochFloors.requireFloor(any(UUID.class)))
        .thenAnswer(
            invocation -> {
              Long floor = floors.get(invocation.getArgument(0));
              if (floor == null) {
                throw new SessionEpochFloorMissingException("session epoch floor missing");
              }
              return floor;
            });
    when(deletionEventPublisher.publishAccountDeleted(any(), any(), any()))
        .thenReturn(new GuestConversionPublishAck("user_events", 1L));
  }

  @Test
  void deleteImmediatelyAdvancesDurableEpochAndPublishesItsMonotonicFloor() throws Exception {
    JsonNode registered =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"delete-epoch@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));
    String accessToken = registered.get("access_token").asText();
    JsonNode otherSession =
        session(
            postJson(
                "/api/v1/auth/login",
                "{\"email\":\"delete-epoch@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"other-device\"}"));
    assertThat(otherSession.get("access_token").asText()).isNotEqualTo(accessToken);
    Account before = accounts.findByEmail("delete-epoch@example.com").orElseThrow();

    authService.deleteAccount("Bearer " + accessToken, "Correct horse battery staple");

    Account deleted = accounts.findById(before.id().toString()).orElseThrow();
    long expectedEpoch = before.sessionEpoch() + 1;
    assertThat(deleted.sessionEpoch()).isEqualTo(expectedEpoch);
    verify(sessionEpochFloors).recordAtLeast(before.id(), expectedEpoch);
  }

  @Test
  void redisFloorFailureRollsBackDeletionBeforeAnOldFloorCanRemainVisible()
      throws Exception {
    JsonNode registered =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"delete-epoch-floor-failure@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));
    String accessToken = registered.get("access_token").asText();
    Account before = accounts.findByEmail("delete-epoch-floor-failure@example.com").orElseThrow();
    doThrow(new SessionEpochFloorUnavailableException("redis unavailable"))
        .when(sessionEpochFloors)
        .recordAtLeast(any(UUID.class), anyLong());

    assertThatThrownBy(
            () -> authService.deleteAccount("Bearer " + accessToken, "Correct horse battery staple"))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    Account afterFailure = accounts.findById(before.id().toString()).orElseThrow();
    assertThat(afterFailure.status()).isEqualTo("active");
    assertThat(afterFailure.deletedAt()).isNull();
    assertThat(afterFailure.sessionEpoch()).isEqualTo(before.sessionEpoch());
    assertThat(deletionOperations.findByAccountAndEpoch(before.id(), before.sessionEpoch() + 1))
        .isEmpty();
    assertThat(authService.validate("Bearer " + accessToken).userId()).isEqualTo(before.id().toString());
    verify(sessionEpochFloors).recordAtLeast(before.id(), before.sessionEpoch() + 1);
    verifyNoInteractions(authEventPublisher);
    verifyNoInteractions(deletionEventPublisher);
  }

  @Test
  void retryAfterRedisFloorFailureStartsAndCompletesANewDeletion()
      throws Exception {
    JsonNode registered =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"delete-epoch-floor-retry@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));
    String accessToken = registered.get("access_token").asText();
    Account before = accounts.findByEmail("delete-epoch-floor-retry@example.com").orElseThrow();
    org.mockito.Mockito.clearInvocations(sessionEpochFloors);
    doThrow(new SessionEpochFloorUnavailableException("redis unavailable"))
        .when(sessionEpochFloors)
        .recordAtLeast(any(UUID.class), anyLong());

    assertThatThrownBy(
            () -> authService.deleteAccount("Bearer " + accessToken, "Correct horse battery staple"))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
    Account afterFailure = accounts.findById(before.id().toString()).orElseThrow();
    assertThat(afterFailure.status()).isEqualTo("active");
    assertThat(afterFailure.deletedAt()).isNull();
    assertThat(afterFailure.sessionEpoch()).isEqualTo(before.sessionEpoch());
    assertThat(deletionOperations.findByAccountAndEpoch(before.id(), before.sessionEpoch() + 1))
        .isEmpty();
    assertThat(authService.validate("Bearer " + accessToken).userId()).isEqualTo(before.id().toString());
    verifyNoInteractions(authEventPublisher);
    verifyNoInteractions(deletionEventPublisher);

    org.mockito.Mockito.doAnswer(invocation -> invocation.getArgument(1))
        .when(sessionEpochFloors)
        .recordAtLeast(any(UUID.class), anyLong());

    DeleteAccountResult retry =
        authService.deleteAccount("Bearer " + accessToken, "Correct horse battery staple");

    assertThat(retry.restoreToken()).isNotBlank();
    Account afterRetry = accounts.findById(before.id().toString()).orElseThrow();
    assertThat(afterRetry.status()).isEqualTo("deleted");
    assertThat(afterRetry.deletedAt()).isNotNull();
    assertThat(afterRetry.sessionEpoch()).isEqualTo(before.sessionEpoch() + 1);
    ArgumentCaptor<Long> sealedEpochs = ArgumentCaptor.forClass(Long.class);
    verify(sessionEpochFloors, times(2)).recordAtLeast(eq(before.id()), sealedEpochs.capture());
    assertThat(sealedEpochs.getAllValues())
        .containsExactly(before.sessionEpoch() + 1, before.sessionEpoch() + 1);
    verifyDeletionEvent(before.id());
  }

  @Test
  void twoAuthInstancesResumeOneDeletionWithTheSameRestoreTokenAndEvent() throws Exception {
    JsonNode registered =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"delete-epoch-concurrent@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));
    String firstAccessToken = registered.get("access_token").asText();
    String secondAccessToken =
        session(
                postJson(
                    "/api/v1/auth/login",
                    "{\"email\":\"delete-epoch-concurrent@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"other-device\"}"))
            .get("access_token")
            .asText();
    Account before = accounts.findByEmail("delete-epoch-concurrent@example.com").orElseThrow();
    AuthService secondAuthInstance = authService.withClock(java.time.Clock.systemUTC());
    ExecutorService workers = Executors.newFixedThreadPool(2);
    try {
      var first =
          workers.submit(
              () -> authService.deleteAccount("Bearer " + firstAccessToken, "Correct horse battery staple"));
      var second =
          workers.submit(
              () ->
                  secondAuthInstance.deleteAccount(
                      "Bearer " + secondAccessToken, "Correct horse battery staple"));

      String firstToken = first.get().restoreToken();
      String secondToken = second.get().restoreToken();
      assertThat(firstToken).isNotBlank();
      assertThat(secondToken).isEqualTo(firstToken);
    } finally {
      workers.shutdownNow();
    }

    assertThat(accounts.findById(before.id().toString()).orElseThrow().sessionEpoch()).isEqualTo(2L);
    verifyDeletionEvent(before.id());
  }

  @Test
  void deleteAndRestoreAccountWithinGracePeriod() throws Exception {
    JsonNode registered =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"delete-restore@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));
    String accessToken = registered.get("access_token").asText();

    DeleteAccountResult deleted =
        authService.deleteAccount("Bearer " + accessToken, "Correct horse battery staple");
    assertThat(deleted.restoreToken()).isNotBlank();

    assertThatThrownBy(() -> authService.validate("Bearer " + accessToken))
        .isInstanceOf(AuthException.class)
        .hasMessage("token_revoked");

    mockMvc
        .perform(
            post("/api/v1/auth/restore-account")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"token\":\"" + deleted.restoreToken() + "\"}"))
        .andExpect(status().isOk());

    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"delete-restore@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk());
  }

  private JsonNode postJson(String path, String body) throws Exception {
    String response =
        mockMvc
            .perform(post(path).contentType(MediaType.APPLICATION_JSON).content(body))
            .andExpect(status().isOk())
            .andReturn()
            .getResponse()
            .getContentAsString();
    return objectMapper.readTree(response);
  }

  private static JsonNode session(JsonNode envelope) {
    assertThat(envelope.has("session")).isTrue();
    return envelope.get("session");
  }

  private void verifyDeletionEvent(UUID accountId) {
    ArgumentCaptor<UserStreamEvent> envelope = ArgumentCaptor.forClass(UserStreamEvent.class);
    ArgumentCaptor<String> natsMessageId = ArgumentCaptor.forClass(String.class);
    verify(deletionEventPublisher, times(1))
        .publishAccountDeleted(eq("user.account_deleted"), envelope.capture(), natsMessageId.capture());
    assertThat(envelope.getValue().getEventId()).isEqualTo(natsMessageId.getValue());
    assertThat(UUID.fromString(envelope.getValue().getEventId())).isNotNull();
    assertThat(envelope.getValue().getUserAccountDeleted().getAccountId()).isEqualTo(accountId.toString());
  }
}
