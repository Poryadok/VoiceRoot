package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
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

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class OtpRestIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired CapturingMailSender mailSender;

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
