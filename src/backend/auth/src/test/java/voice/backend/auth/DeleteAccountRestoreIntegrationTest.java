package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.UUID;
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
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.DeleteAccountResult;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.verify;
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

  @BeforeEach
  void setUpEpochFloor() {
    when(sessionEpochFloors.recordAtLeast(any(UUID.class), anyLong()))
        .thenAnswer(invocation -> invocation.getArgument(1));
  }

  @Test
  void deleteImmediatelyAdvancesDurableEpochAndPublishesItsMonotonicFloor() throws Exception {
    JsonNode registered =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"delete-epoch@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));
    String accessToken = registered.get("access_token").asText();
    Account before = accounts.findByEmail("delete-epoch@example.com").orElseThrow();

    authService.deleteAccount("Bearer " + accessToken, "Correct horse battery staple");

    Account deleted = accounts.findById(before.id().toString()).orElseThrow();
    long expectedEpoch = before.sessionEpoch() + 1;
    assertThat(deleted.sessionEpoch()).isEqualTo(expectedEpoch);
    verify(sessionEpochFloors).recordAtLeast(before.id(), expectedEpoch);
  }

  @Test
  void deleteFailsClosedWhenTheAccountWideEpochFloorCannotBeRecorded() throws Exception {
    JsonNode registered =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"delete-epoch-floor-failure@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));
    String accessToken = registered.get("access_token").asText();
    doThrow(new SessionEpochFloorUnavailableException("redis unavailable"))
        .when(sessionEpochFloors)
        .recordAtLeast(any(UUID.class), anyLong());

    assertThatThrownBy(
            () -> authService.deleteAccount("Bearer " + accessToken, "Correct horse battery staple"))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
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
