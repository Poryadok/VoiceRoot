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
class GuestReminderIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;

  @Test
  void guestReminderGetMarkRoundTripAndSameDaySuppress() throws Exception {
    JsonNode guest =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"password\":\"Correct horse battery staple\",\"guest\":true,\"device_info_json\":\"{}\"}"));
    String token = guest.get("access_token").asText();

    String before =
        mockMvc
            .perform(get("/api/v1/auth/guest-reminder").header("Authorization", "Bearer " + token))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.should_show", is(true)))
            .andReturn()
            .getResponse()
            .getContentAsString();
    assertThat(objectMapper.readTree(before).get("last_shown_at").isNull()).isTrue();

    mockMvc
        .perform(post("/api/v1/auth/guest-reminder/mark").header("Authorization", "Bearer " + token))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.last_shown_at").isNotEmpty());

    mockMvc
        .perform(get("/api/v1/auth/guest-reminder").header("Authorization", "Bearer " + token))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.should_show", is(false)))
        .andExpect(jsonPath("$.last_shown_at").isNotEmpty());
  }

  @Test
  void guestReminderRejectedForRegularAccount() throws Exception {
    JsonNode regular =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"reminder-regular@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));

    mockMvc
        .perform(
            get("/api/v1/auth/guest-reminder")
                .header("Authorization", "Bearer " + regular.get("access_token").asText()))
        .andExpect(status().isBadRequest())
        .andExpect(jsonPath("$.error", is("validation_failed")));
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
