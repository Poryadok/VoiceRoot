package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.hamcrest.Matchers;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import voice.backend.auth.support.CapturingMailSender;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class GuestPendingIdentityConstructionIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired CapturingMailSender mailSender;

  @Test
  void guestRegistrationRejectsClientSuppliedEmailOrPhoneIdentifiers() throws Exception {
    mockMvc
        .perform(
            post("/api/v1/auth/register")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"guest\":true,\"email\":\"guest-email@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isBadRequest())
        .andExpect(jsonPath("$.error", Matchers.is("validation_failed")));

    mockMvc
        .perform(
            post("/api/v1/auth/register")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"guest\":true,\"phone\":\"+15550123\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isBadRequest())
        .andExpect(jsonPath("$.error", Matchers.is("validation_failed")));
  }

  @Test
  void convertGuestRejectsPhoneOnlyOrMixedPhoneInput() throws Exception {
    JsonNode guest =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"guest\":true,\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));

    for (String identity :
        new String[] {
          "{\"phone\":\"+15550124\",\"password\":\"New account password 1\"}",
          "{\"email\":\"guest-mixed@example.com\",\"phone\":\"+15550125\",\"password\":\"New account password 1\"}"
        }) {
      mockMvc
          .perform(
              post("/api/v1/auth/convert-guest")
                  .header("Authorization", "Bearer " + guest.get("access_token").asText())
                  .contentType(MediaType.APPLICATION_JSON)
                  .content(identity))
          .andExpect(status().isBadRequest())
          .andExpect(jsonPath("$.error", Matchers.is("validation_failed")));
    }
  }

  @Test
  void emailVerificationCanBeRequestedOnlyAfterEmailConvertGuestBuildsThePendingIdentity() throws Exception {
    JsonNode guest =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"guest\":true,\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));

    mailSender.clear();
    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .header("Authorization", "Bearer " + guest.get("access_token").asText())
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isBadRequest())
        .andExpect(jsonPath("$.error", Matchers.is("validation_failed")));
    assertThat(mailSender.lastCode()).isNull();

    mockMvc
        .perform(
            post("/api/v1/auth/convert-guest")
                .header("Authorization", "Bearer " + guest.get("access_token").asText())
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"converted-pending@example.com\",\"password\":\"New account password 1\"}"))
        .andExpect(status().isOk());

    mailSender.clear();
    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"converted-pending@example.com\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isNoContent());
    assertThat(mailSender.lastCode()).matches("\\d{6}");
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
