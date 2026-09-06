package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThatThrownBy;
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
import voice.backend.auth.support.CapturingMailSender;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class OtpRestIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired CapturingMailSender mailSender;
  @Autowired GuestConversionOperationRepository operations;
  @Autowired InMemoryPrimaryProfileProvisioner profiles;

  @Test
  void freshEmailRegistrationStaysGuestUntilEmailOtpVerificationThenBecomesRegular() throws Exception {
    JsonNode pending =
        objectMapper.readTree(
            mockMvc
                .perform(
                    post("/api/v1/auth/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(
                            "{\"email\":\"pending-email@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.session.account_type").value("guest"))
                .andReturn()
                .getResponse()
                .getContentAsString());
    String accountId = pending.path("session").path("account_id").asText();
    org.assertj.core.api.Assertions.assertThat(profiles.isGuestAccount(java.util.UUID.fromString(accountId))).isTrue();

    mailSender.clear();
    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"pending-email@example.com\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isNoContent());

    mockMvc
        .perform(
            post("/api/v1/auth/otp/verify")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"pending-email@example.com\",\"code\":\""
                        + mailSender.lastCode()
                        + "\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_type").value("regular"));

    org.assertj.core.api.Assertions.assertThat(profiles.isGuestAccount(java.util.UUID.fromString(accountId))).isFalse();
    org.assertj.core.api.Assertions.assertThat(
            operations.findByAccountId(java.util.UUID.fromString(accountId)))
        .get()
        .extracting(operation -> operation.state())
        .isEqualTo(GuestConversionState.PENDING_EVENT);

    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"pending-email@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_type").value("regular"));
  }

  @Test
  void sendAndVerifyPasswordResetOtp() throws Exception {
    session(
        postJson(
            "/api/v1/auth/register",
            "{\"email\":\"otp-user@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));

    mailSender.clear();
    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"otp-user@example.com\",\"otp_type\":\"password_reset\"}"))
        .andExpect(status().isNoContent());

    String code = mailSender.lastCode();
    org.assertj.core.api.Assertions.assertThat(code).matches("\\d{6}");

    mockMvc
        .perform(
            post("/api/v1/auth/otp/verify")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"otp-user@example.com\",\"code\":\""
                        + code
                        + "\",\"otp_type\":\"password_reset\"}"))
        .andExpect(status().isNoContent());
  }

  @Test
  void passwordResetE2EAllowsLoginWithNewPassword() throws Exception {
    session(
        postJson(
            "/api/v1/auth/register",
            "{\"email\":\"reset-e2e@example.com\",\"password\":\"OldPassword99!\",\"device_info_json\":\"{}\"}"));

    mailSender.clear();
    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"reset-e2e@example.com\",\"otp_type\":\"password_reset\"}"))
        .andExpect(status().isNoContent());
    String code = mailSender.lastCode();
    org.assertj.core.api.Assertions.assertThat(code).matches("\\d{6}");

    mockMvc
        .perform(
            post("/api/v1/auth/password/reset")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"reset-e2e@example.com\",\"code\":\""
                        + code
                        + "\",\"new_password\":\"NewPassword99!\"}"))
        .andExpect(status().isNoContent());

    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"reset-e2e@example.com\",\"password\":\"OldPassword99!\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isUnauthorized());

    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"reset-e2e@example.com\",\"password\":\"NewPassword99!\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk());
  }

  @Test
  void otpSendIsRateLimited() throws Exception {
    mockMvc
        .perform(
            post("/api/v1/auth/register")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"otp-rate@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk());

    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"otp-rate@example.com\",\"otp_type\":\"password_reset\"}"))
        .andExpect(status().isNoContent());

    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"otp-rate@example.com\",\"otp_type\":\"password_reset\"}"))
        .andExpect(status().isTooManyRequests());
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
    return envelope.get("session");
  }
}
