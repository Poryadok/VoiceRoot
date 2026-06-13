package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
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
import org.springframework.test.web.servlet.MvcResult;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class TwoFactorAuthTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;

  @Test
  void enable2FAReturnsTotpUriAndBackupCodes() throws Exception {
    JsonNode registered = register("2fa-enable@voice-qa.test", "Correct horse battery staple");
    String access = registered.get("access_token").asText();

    MvcResult result = mockMvc.perform(post("/api/v1/auth/2fa/enable")
            .header("Authorization", "Bearer " + access)
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"password\":\"Correct horse battery staple\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.totp_uri").isNotEmpty())
        .andExpect(jsonPath("$.backup_codes").isArray())
        .andReturn();

    JsonNode body = objectMapper.readTree(result.getResponse().getContentAsString());
    assertThat(body.get("backup_codes")).hasSizeGreaterThanOrEqualTo(8);
  }

  @Test
  void verify2FAActivatesTotp() throws Exception {
    JsonNode registered = register("2fa-verify@voice-qa.test", "Correct horse battery staple");
    String access = registered.get("access_token").asText();

    MvcResult enroll = mockMvc.perform(post("/api/v1/auth/2fa/enable")
            .header("Authorization", "Bearer " + access)
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"password\":\"Correct horse battery staple\"}"))
        .andExpect(status().isOk())
        .andReturn();
    JsonNode enrollBody = objectMapper.readTree(enroll.getResponse().getContentAsString());

    mockMvc.perform(post("/api/v1/auth/2fa/verify")
            .header("Authorization", "Bearer " + access)
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"totp_code\":\"000000\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.access_token").isNotEmpty());

    assertThat(enrollBody.get("totp_uri").asText()).contains("otpauth://");
  }

  @Test
  void loginRequiresTotpWhenEnabled() throws Exception {
    String email = "2fa-login-gate@voice-qa.test";
    String password = "Correct horse battery staple";
    JsonNode registered = register(email, password);
    String access = registered.get("access_token").asText();

    mockMvc.perform(post("/api/v1/auth/2fa/enable")
            .header("Authorization", "Bearer " + access)
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"password\":\"" + password + "\"}"))
        .andExpect(status().isOk());

    mockMvc.perform(post("/api/v1/auth/2fa/verify")
            .header("Authorization", "Bearer " + access)
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"totp_code\":\"000000\"}"))
        .andExpect(status().isOk());

    mockMvc.perform(post("/api/v1/auth/login")
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"email\":\"" + email + "\",\"password\":\"" + password + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error").value("totp_required"));
  }

  @Test
  void loginWithBackupCodeSucceedsWhenTotpMissing() throws Exception {
    String email = "2fa-backup@voice-qa.test";
    String password = "Correct horse battery staple";
    JsonNode registered = register(email, password);
    String access = registered.get("access_token").asText();

    MvcResult enroll = mockMvc.perform(post("/api/v1/auth/2fa/enable")
            .header("Authorization", "Bearer " + access)
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"password\":\"" + password + "\"}"))
        .andExpect(status().isOk())
        .andReturn();
    JsonNode enrollBody = objectMapper.readTree(enroll.getResponse().getContentAsString());
    String backupCode = enrollBody.get("backup_codes").get(0).asText();

    mockMvc.perform(post("/api/v1/auth/2fa/verify")
            .header("Authorization", "Bearer " + access)
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"totp_code\":\"000000\"}"))
        .andExpect(status().isOk());

    mockMvc.perform(post("/api/v1/auth/login")
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"email\":\"" + email + "\",\"password\":\"" + password + "\",\"totp_code\":\"" + backupCode + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.access_token").isNotEmpty());
  }

  private JsonNode register(String email, String password) throws Exception {
    MvcResult result = mockMvc.perform(post("/api/v1/auth/register")
            .contentType(MediaType.APPLICATION_JSON)
            .content("{\"email\":\"" + email + "\",\"password\":\"" + password + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk())
        .andReturn();
    JsonNode root = objectMapper.readTree(result.getResponse().getContentAsString());
    return root.get("session");
  }
}
