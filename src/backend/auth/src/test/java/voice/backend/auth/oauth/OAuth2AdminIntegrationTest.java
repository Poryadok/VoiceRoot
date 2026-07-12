package voice.backend.auth.oauth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.hamcrest.Matchers.is;
import static org.hamcrest.Matchers.not;
import static org.hamcrest.Matchers.blankOrNullString;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.util.Base64;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.TestPropertySource;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
@TestPropertySource(
    properties = {
      "auth.oauth.developer-portal.enabled=false",
      "auth.oauth.admin.enabled=true",
      "auth.oauth.admin.client-id=voice-admin",
      "auth.oauth.admin.redirect-uris=http://localhost:9081/callback,http://127.0.0.1:9081/callback",
      "auth.oauth.admin.authorization-code-ttl=PT60S",
      "auth.oauth.public-api-base-url=http://127.0.0.1:18080"
    })
class OAuth2AdminIntegrationTest {
  private static final String CLIENT_ID = "voice-admin";
  private static final String REDIRECT_URI = "http://localhost:9081/callback";
  private static final Pattern CODE_PATTERN = Pattern.compile("[?&]code=([^&]+)");

  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;

  private String codeVerifier;
  private String codeChallenge;

  @BeforeEach
  void pkce() {
    codeVerifier = randomCodeVerifier();
    codeChallenge = PkceVerifier.s256Challenge(codeVerifier);
  }

  @Test
  void adminClientAuthorizeCodeTokenFlow() throws Exception {
    registerUser("admin-oauth@example.com");

    String code = authorizeAndLogin("admin-oauth@example.com", "Correct horse battery staple", null);
    JsonNode token = exchangeCode(code, codeVerifier);

    String accessToken = token.get("access_token").asText();
    assertThat(token.get("token_type").asText()).isEqualTo("Bearer");

    mockMvc
        .perform(post("/api/v1/auth/validate").header("Authorization", "Bearer " + accessToken))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.user_id", not(blankOrNullString())))
        .andExpect(jsonPath("$.profile_id", not(blankOrNullString())));
  }

  @Test
  void developerPortalClientRejectedWhenOnlyAdminEnabled() throws Exception {
    mockMvc
        .perform(
            get("/api/v1/auth/oauth2/authorize")
                .queryParam("response_type", "code")
                .queryParam("client_id", "voice-developer-portal")
                .queryParam("redirect_uri", REDIRECT_URI)
                .queryParam("state", "s1")
                .queryParam("code_challenge", codeChallenge)
                .queryParam("code_challenge_method", "S256"))
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error", is("invalid_client")));
  }

  private void registerUser(String email) throws Exception {
    mockMvc
        .perform(
            post("/api/v1/auth/register")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\""
                        + email
                        + "\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk());
  }

  private String authorizeAndLogin(String email, String password, String totp) throws Exception {
    mockMvc
        .perform(
            get("/api/v1/auth/oauth2/authorize")
                .queryParam("response_type", "code")
                .queryParam("client_id", CLIENT_ID)
                .queryParam("redirect_uri", REDIRECT_URI)
                .queryParam("state", "test-state")
                .queryParam("code_challenge", codeChallenge)
                .queryParam("code_challenge_method", "S256"))
        .andExpect(status().isOk());

    MultiValueMap<String, String> login = new LinkedMultiValueMap<>();
    login.add("email", email);
    login.add("password", password);
    login.add("response_type", "code");
    login.add("client_id", CLIENT_ID);
    login.add("redirect_uri", REDIRECT_URI);
    login.add("state", "test-state");
    login.add("code_challenge", codeChallenge);
    login.add("code_challenge_method", "S256");
    if (totp != null) {
      login.add("totp_code", totp);
    }

    String location =
        mockMvc
            .perform(
                FormUrlEncodedTestSupport.postForm(
                    post("/api/v1/auth/oauth2/authorize"), login))
            .andExpect(status().isFound())
            .andReturn()
            .getResponse()
            .getHeader("Location");

    assertThat(location).startsWith(REDIRECT_URI);
    Matcher matcher = CODE_PATTERN.matcher(location);
    assertThat(matcher.find()).isTrue();
    return URLDecoder.decode(matcher.group(1), StandardCharsets.UTF_8);
  }

  private JsonNode exchangeCode(String code, String verifier) throws Exception {
    MultiValueMap<String, String> form = baseTokenForm(code);
    form.set("code_verifier", verifier);
    String body =
        mockMvc
            .perform(
                FormUrlEncodedTestSupport.postForm(
                    post("/api/v1/auth/oauth2/token"), form))
            .andExpect(status().isOk())
            .andReturn()
            .getResponse()
            .getContentAsString();
    return objectMapper.readTree(body);
  }

  private MultiValueMap<String, String> baseTokenForm(String code) {
    MultiValueMap<String, String> form = new LinkedMultiValueMap<>();
    form.add("grant_type", "authorization_code");
    form.add("code", code);
    form.add("redirect_uri", REDIRECT_URI);
    form.add("client_id", CLIENT_ID);
    return form;
  }

  private static String randomCodeVerifier() {
    byte[] bytes = new byte[32];
    new SecureRandom().nextBytes(bytes);
    return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
  }
}
