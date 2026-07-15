package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.hamcrest.Matchers.is;
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
class ConvertGuestIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;

  @Test
  void registerGuestWithoutEmailOrPhoneSucceeds() throws Exception {
    JsonNode registered =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"password\":\"Correct horse battery staple\",\"guest\":true,\"device_info_json\":\"{}\"}"));

    assertThat(registered.get("account_id").asText())
        .matches("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}");
    assertThat(registered.get("profile_id").asText())
        .matches("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}");
    assertThat(registered.get("access_token").asText()).isNotBlank();
  }

  @Test
  void convertGuestRejectsDuplicateEmail() throws Exception {
    JsonNode existing =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"taken@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));
    assertThat(existing.get("account_id").asText()).isNotBlank();

    JsonNode guest =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"password\":\"Correct horse battery staple\",\"guest\":true,\"device_info_json\":\"{}\"}"));

    mockMvc
        .perform(
            post("/api/v1/auth/convert-guest")
                .header("Authorization", "Bearer " + guest.get("access_token").asText())
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"taken@example.com\",\"password\":\"New account password 1\"}"))
        .andExpect(status().isBadRequest())
        .andExpect(jsonPath("$.error", is("registration_conflict")));
  }

  @Test
  void convertGuestToRegularKeepsAccountIdAndSetsNewPassword() throws Exception {
    JsonNode guest =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"password\":\"Correct horse battery staple\",\"guest\":true,\"device_info_json\":\"{}\"}"));
    String guestAccountId = guest.get("account_id").asText();
    String accessToken = guest.get("access_token").asText();
    String newPassword = "New account password 1";

    String response =
        mockMvc
            .perform(
                post("/api/v1/auth/convert-guest")
                    .header("Authorization", "Bearer " + accessToken)
                    .contentType(MediaType.APPLICATION_JSON)
                    .content(
                        "{\"email\":\"guest-convert@example.com\",\"password\":\"" + newPassword + "\"}"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.session.account_id", is(guestAccountId)))
            .andReturn()
            .getResponse()
            .getContentAsString();

    JsonNode converted = objectMapper.readTree(response).get("session");
    assertThat(converted.get("account_id").asText()).isEqualTo(guestAccountId);
    assertThat(converted.get("profile_id").asText()).isEqualTo(guest.get("profile_id").asText());

    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"guest-convert@example.com\",\"password\":\"" + newPassword + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_id", is(guestAccountId)));
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
