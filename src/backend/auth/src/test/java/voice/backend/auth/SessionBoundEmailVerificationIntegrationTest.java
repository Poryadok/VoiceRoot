package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
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
import voice.backend.auth.support.CapturingMailSender;
import voice.backend.auth.rest.AuthRestController;
import voice.backend.auth.service.AuthSession;

/** Contract proof for the A1 restricted-session email-verification recovery flow. */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class SessionBoundEmailVerificationIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired CapturingMailSender mailSender;

  @Test
  void legacySessionOmitsUnknownMarkerInRestJson() throws Exception {
    var legacy = new AuthSession("access", "refresh", 900, "account", "profile", "guest");
    JsonNode json = objectMapper.readTree(
        objectMapper.writeValueAsString(AuthRestController.SessionBody.from(legacy)));
    assertThat(json.has("email_verification_required")).isFalse();
  }

  @Test
  void registerCreatesRestrictedSessionAndSendsExactlyOneVerificationEmail() throws Exception {
    mailSender.clear();

    JsonNode envelope =
        postJson(
            "/api/v1/auth/register",
            "{\"email\":\"session-bound-register@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}");

    assertThat(envelope.path("session").path("account_type").asText()).isEqualTo("guest");
    assertThat(envelope.path("session").path("email_verification_required").asBoolean()).isTrue();
    JsonNode login = postJson("/api/v1/auth/login",
        "{\"email\":\"session-bound-register@example.com\",\"password\":\"Correct horse battery staple\"}")
        .path("session");
    assertThat(login.path("email_verification_required").asBoolean()).isTrue();
    String switchedBody = mockMvc.perform(post("/api/v1/auth/switch-profile")
            .header("Authorization", "Bearer " + login.path("access_token").asText())
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"profile_id\":\"" + login.path("profile_id").asText() + "\"}"))
        .andExpect(status().isOk())
        .andReturn().getResponse().getContentAsString();
    assertThat(objectMapper.readTree(switchedBody).path("email_verification_required").asBoolean()).isTrue();
    assertThat(mailSender.lastCode()).matches("\\d{6}");
    mockMvc
        .perform(
            get("/api/v1/auth/verification-status")
                .header("Authorization", "Bearer " + envelope.path("session").path("access_token").asText()))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.state").value("EMAIL_PENDING"))
        .andExpect(jsonPath("$.code_state").value("ACTIVE"));
  }

  @Test
  void convertedGuestRemainsEmailPendingAcrossStatusAndRefresh() throws Exception {
    JsonNode guest = postJson(
        "/api/v1/auth/register",
        "{\"password\":\"Correct horse battery staple\",\"guest\":true,\"device_info_json\":\"{}\"}");
    String guestToken = guest.path("session").path("access_token").asText();
    assertThat(guest.path("session").path("email_verification_required").asBoolean()).isFalse();
    String convertedBody = mockMvc.perform(post("/api/v1/auth/convert-guest")
            .header("Authorization", "Bearer " + guestToken)
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"email\":\"converted-status@example.com\",\"password\":\"New account password 1\"}"))
        .andExpect(status().isOk())
        .andReturn().getResponse().getContentAsString();
    JsonNode converted = objectMapper.readTree(convertedBody).path("session");
    assertThat(converted.path("email_verification_required").asBoolean()).isTrue();

    JsonNode refreshed = postJson("/api/v1/auth/refresh",
        "{\"refresh_token\":\"" + converted.path("refresh_token").asText() + "\",\"device_info_json\":\"{}\"}")
        .path("session");
    assertThat(refreshed.path("email_verification_required").asBoolean()).isTrue();

    mockMvc.perform(get("/api/v1/auth/verification-status")
            .header("Authorization", "Bearer " + refreshed.path("access_token").asText()))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.state").value("EMAIL_PENDING"))
        .andExpect(jsonPath("$.code_state").value("ACTIVE"));
  }

  @Test
  void emailVerificationRejectsRawEmailAndAcceptsOnlyTheRestrictedSession() throws Exception {
    mailSender.clear();
    JsonNode envelope =
        postJson(
            "/api/v1/auth/register",
            "{\"email\":\"session-bound-otp@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}");
    String token = envelope.path("session").path("access_token").asText();

    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"session-bound-otp@example.com\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isUnauthorized());

    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .header("Authorization", "Bearer " + token)
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isTooManyRequests())
        .andExpect(header().string("Retry-After", "600"));
  }

  private JsonNode postJson(String path, String body) throws Exception {
    String response =
        mockMvc
            .perform(post(path).contentType(MediaType.APPLICATION_JSON).content(body))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.session.access_token").isNotEmpty())
            .andReturn()
            .getResponse()
            .getContentAsString();
    return objectMapper.readTree(response);
  }
}
