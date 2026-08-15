package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.hamcrest.Matchers.is;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class ActiveSessionsIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;

  @Test
  void listAndRevokeSessionBlocksRefresh() throws Exception {
    JsonNode first =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"sessions-a@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{\\\"device\\\":\\\"a\\\"}\"}"));

    JsonNode second =
        session(
            postJson(
                "/api/v1/auth/login",
                "{\"email\":\"sessions-a@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{\\\"device\\\":\\\"b\\\"}\"}"));

    String listRaw =
        mockMvc
            .perform(
                get("/api/v1/auth/sessions")
                    .header("Authorization", "Bearer " + second.get("access_token").asText()))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.sessions").isArray())
            .andReturn()
            .getResponse()
            .getContentAsString();
    JsonNode sessions = objectMapper.readTree(listRaw).get("sessions");
    assertThat(sessions.size()).isGreaterThanOrEqualTo(2);

    String otherSessionId = null;
    for (JsonNode s : sessions) {
      if (!s.get("current").asBoolean()) {
        otherSessionId = s.get("id").asText();
        break;
      }
    }
    assertThat(otherSessionId).isNotBlank();

    mockMvc
        .perform(
            post("/api/v1/auth/sessions/" + otherSessionId + "/revoke")
                .header("Authorization", "Bearer " + second.get("access_token").asText()))
        .andExpect(status().isNoContent());

    mockMvc
        .perform(
            post("/api/v1/auth/refresh")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"refresh_token\":\""
                        + first.get("refresh_token").asText()
                        + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error", is("token_revoked")));
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
