package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.hamcrest.Matchers.is;
import static org.hamcrest.Matchers.not;
import static org.hamcrest.Matchers.blankOrNullString;
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
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class AuthRestIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;

  @Test
  void registerLoginRefreshValidateLogoutAndJwksWorkOverRest() throws Exception {
    JsonNode registered = postJson("/api/v1/auth/register",
        "{\"email\":\"rest@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}");

    String access = registered.get("access_token").asText();
    String refresh = registered.get("refresh_token").asText();
    assertThat(access).contains(".");
    assertThat(refresh).isNotBlank().doesNotContain(".");
    assertThat(registered.get("expires_in_seconds").asLong()).isEqualTo(900);
    assertThat(registered.get("profile_id").asText())
        .matches("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}");

    mockMvc.perform(post("/api/v1/auth/validate")
            .header("Authorization", "Bearer " + access))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.user_id", is(registered.get("account_id").asText())))
        .andExpect(jsonPath("$.profile_id", is(registered.get("profile_id").asText())))
        .andExpect(jsonPath("$.jti", not(blankOrNullString())));

    JsonNode login = postJson("/api/v1/auth/login",
        "{\"email\":\"rest@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}");
    assertThat(login.get("refresh_token").asText()).isNotEqualTo(refresh);

    JsonNode rotated = postJson("/api/v1/auth/refresh",
        "{\"refresh_token\":\"" + refresh + "\",\"device_info_json\":\"{}\"}");
    assertThat(rotated.get("refresh_token").asText()).isNotEqualTo(refresh);

    mockMvc.perform(post("/api/v1/auth/logout")
            .header("Authorization", "Bearer " + rotated.get("access_token").asText())
            .contentType("application/json")
            .content("{\"refresh_token\":\"" + rotated.get("refresh_token").asText() + "\"}"))
        .andExpect(status().isNoContent());

    mockMvc.perform(post("/api/v1/auth/validate")
            .header("Authorization", "Bearer " + rotated.get("access_token").asText()))
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error", is("token_revoked")));

    mockMvc.perform(get("/api/v1/auth/.well-known/jwks.json"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.keys[0].kid", is("test-key")));
  }

  @Test
  void restErrorsAreStableAndDoNotRevealAccountExistence() throws Exception {
    mockMvc.perform(post("/api/v1/auth/login")
            .contentType("application/json")
            .content("{\"email\":\"missing@example.com\",\"password\":\"wrong\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error", is("invalid_credentials")));

    mockMvc.perform(post("/api/v1/auth/refresh")
            .contentType("application/json")
            .content("{\"refresh_token\":\"short\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error", is("invalid_token")));
  }

  private JsonNode postJson(String path, String body) throws Exception {
    String response = mockMvc.perform(post(path).contentType("application/json").content(body))
        .andExpect(status().isOk())
        .andReturn()
        .getResponse()
        .getContentAsString();
    return objectMapper.readTree(response);
  }
}
