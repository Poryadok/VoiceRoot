package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.hamcrest.Matchers.is;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.GuestConversionPendingUserRecoveryRunner;
import voice.backend.auth.support.CapturingMailSender;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class ConvertGuestIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired CapturingMailSender mailSender;
  @Autowired AccountRepository accounts;
  @Autowired GuestConversionOperationRepository operations;
  @Autowired GuestConversionPendingUserRecoveryRunner pendingUserRecovery;
  @Autowired BCryptPasswordHasher passwordHasher;
  @Autowired Clock clock;

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
  void convertGuestRejectsPasswordShorterThanEight() throws Exception {
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
                .content("{\"email\":\"short-pass@example.com\",\"password\":\"short\"}"))
        .andExpect(status().isBadRequest())
        .andExpect(jsonPath("$.error", is("validation_failed")));
  }

  @Test
  void convertGuestRejectsNonGuestAccount() throws Exception {
    JsonNode regular =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"email\":\"already-regular@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"));

    mockMvc
        .perform(
            post("/api/v1/auth/convert-guest")
                .header("Authorization", "Bearer " + regular.get("access_token").asText())
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"already-regular2@example.com\",\"password\":\"New account password 1\"}"))
        .andExpect(status().isBadRequest())
        .andExpect(jsonPath("$.error", is("validation_failed")));
  }

  @Test
  void convertGuestRejectsMissingEmailAndPhone() throws Exception {
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
                .content("{\"password\":\"New account password 1\"}"))
        .andExpect(status().isBadRequest())
        .andExpect(jsonPath("$.error", is("validation_failed")));
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
  void convertGuestMovesFromDurablePendingUserToPendingEventOnlyAfterRecoveryTick() throws Exception {
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
            .andExpect(jsonPath("$.session.account_type", is("guest")))
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
                    "{\"email\":\"guest-convert@example.com\",\"password\":\""
                        + newPassword
                        + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_type", is("guest")));

    String code = mailSender.lastCode();
    assertThat(code).matches("\\d{6}");

    mockMvc
        .perform(
            post("/api/v1/auth/otp/verify")
                .header("Authorization", "Bearer " + converted.get("access_token").asText())
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"code\":\""
                        + code
                        + "\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isNoContent());

    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"guest-convert@example.com\",\"password\":\"" + newPassword + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_id", is(guestAccountId)))
        .andExpect(jsonPath("$.session.account_type", is("guest")));

    pendingUserRecovery.tick();

    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"guest-convert@example.com\",\"password\":\"" + newPassword + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_id", is(guestAccountId)))
        .andExpect(jsonPath("$.session.account_type", is("regular")));

    Instant now = Instant.now(clock);
    assertThat(operations.leaseDue(1, now, now.plus(Duration.ofMinutes(1))))
        .singleElement()
        .satisfies(
            operation -> {
              assertThat(operation.accountId()).isEqualTo(UUID.fromString(guestAccountId));
              assertThat(operation.state()).isEqualTo(GuestConversionState.PENDING_EVENT);
            });
  }

  @Test
  void legacyConvertedAccountCanRecoverWithoutItsBootstrapPassword() throws Exception {
    String bootstrapPassword = UUID.randomUUID().toString();
    String recoveredPassword = "Recovered account password 1";
    String email = "legacy-convert-" + UUID.randomUUID() + "@example.com";
    JsonNode guest =
        session(
            postJson(
                "/api/v1/auth/register",
                "{\"password\":\""
                    + bootstrapPassword
                    + "\",\"guest\":true,\"device_info_json\":\"{}\"}"));
    String accountId = guest.get("account_id").asText();
    String profileId = guest.get("profile_id").asText();

    // Seed the pre-fix persisted shape without creating unrelated conversion-recovery work.
    var legacyAccount =
        accounts.convertGuest(
            UUID.fromString(accountId), email, null, passwordHasher.hash(bootstrapPassword));
    accounts.markGuestRegular(legacyAccount.id());

    mailSender.clear();
    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"" + email + "\",\"otp_type\":\"password_reset\"}"))
        .andExpect(status().isNoContent());
    String resetCode = mailSender.lastCode();
    assertThat(resetCode).matches("\\d{6}");

    mockMvc
        .perform(
            post("/api/v1/auth/password/reset")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\""
                        + email
                        + "\",\"code\":\""
                        + resetCode
                        + "\",\"new_password\":\""
                        + recoveredPassword
                        + "\"}"))
        .andExpect(status().isNoContent());

    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\""
                        + email
                        + "\",\"password\":\""
                        + bootstrapPassword
                        + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isUnauthorized());
    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\""
                        + email
                        + "\",\"password\":\""
                        + recoveredPassword
                        + "\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_id", is(accountId)))
        .andExpect(jsonPath("$.session.profile_id", is(profileId)))
        .andExpect(jsonPath("$.session.account_type", is("regular")));
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
