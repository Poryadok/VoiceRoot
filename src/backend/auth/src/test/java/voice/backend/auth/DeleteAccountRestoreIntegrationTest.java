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
import java.util.concurrent.ExecutionException;
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
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.DeleteAccountResult;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException;
import voice.backend.auth.sessionepoch.SessionEpochFloorMissingException;

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
  @MockBean SessionEpochFloorStore sessionEpochFloors;
  @MockBean AuthEventPublisher authEventPublisher;

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
  void redisFloorFailureLeavesTheDeleteDurablySealedWithoutReportingSuccessOrPublishingEvent()
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
    assertThat(afterFailure.status()).isEqualTo("deleted");
    assertThat(afterFailure.deletedAt()).isNotNull();
    assertThat(afterFailure.sessionEpoch()).isEqualTo(before.sessionEpoch() + 1);
    verify(sessionEpochFloors).recordAtLeast(before.id(), afterFailure.sessionEpoch());
    verifyNoInteractions(authEventPublisher);
  }

  @Test
  void retryAfterRedisFloorFailureResealsTheEpochWithoutDuplicateDeleteOrEvent()
      throws Exception {
    JsonNode registered =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"delete-epoch-floor-retry@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));
    String accessToken = registered.get("access_token").asText();
    Account before = accounts.findByEmail("delete-epoch-floor-retry@example.com").orElseThrow();
    doThrow(new SessionEpochFloorUnavailableException("redis unavailable"))
        .doAnswer(invocation -> invocation.getArgument(1))
        .when(sessionEpochFloors)
        .recordAtLeast(any(UUID.class), anyLong());

    assertThatThrownBy(
            () -> authService.deleteAccount("Bearer " + accessToken, "Correct horse battery staple"))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
    Account afterFailure = accounts.findById(before.id().toString()).orElseThrow();
    assertThat(afterFailure.status()).isEqualTo("deleted");
    assertThat(afterFailure.deletedAt()).isNotNull();
    assertThat(afterFailure.sessionEpoch()).isEqualTo(before.sessionEpoch() + 1);
    verifyNoInteractions(authEventPublisher);

    DeleteAccountResult retry =
        authService.deleteAccount("Bearer " + accessToken, "Correct horse battery staple");

    assertThat(retry.restoreToken()).isNotBlank();
    Account afterRetry = accounts.findById(before.id().toString()).orElseThrow();
    assertThat(afterRetry.status()).isEqualTo("deleted");
    assertThat(afterRetry.deletedAt()).isEqualTo(afterFailure.deletedAt());
    assertThat(afterRetry.sessionEpoch()).isEqualTo(afterFailure.sessionEpoch());
    ArgumentCaptor<Long> sealedEpochs = ArgumentCaptor.forClass(Long.class);
    verify(sessionEpochFloors, times(2)).recordAtLeast(eq(before.id()), sealedEpochs.capture());
    assertThat(sealedEpochs.getAllValues())
        .containsExactly(afterFailure.sessionEpoch(), afterFailure.sessionEpoch());
    verify(authEventPublisher, times(1)).publishAccountDeleted(before.id());
  }

  @Test
  void concurrentDeleteCallsCompleteOnlyOneAccountDeletion() throws Exception {
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
    ExecutorService workers = Executors.newFixedThreadPool(2);
    try {
      var first =
          workers.submit(
              () -> authService.deleteAccount("Bearer " + firstAccessToken, "Correct horse battery staple"));
      var second =
          workers.submit(
              () -> authService.deleteAccount("Bearer " + secondAccessToken, "Correct horse battery staple"));

      int successes = 0;
      int inactive = 0;
      for (var future : java.util.List.of(first, second)) {
        try {
          assertThat(future.get().restoreToken()).isNotBlank();
          successes++;
        } catch (ExecutionException ex) {
          assertThat(ex.getCause()).isInstanceOf(AuthException.class).hasMessage("account_inactive");
          inactive++;
        }
      }
      assertThat(successes).isEqualTo(1);
      assertThat(inactive).isEqualTo(1);
    } finally {
      workers.shutdownNow();
    }

    assertThat(accounts.findById(before.id().toString()).orElseThrow().sessionEpoch()).isEqualTo(2L);
    verify(authEventPublisher, times(1)).publishAccountDeleted(before.id());
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
}
