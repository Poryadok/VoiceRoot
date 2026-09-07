package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.support.CapturingMailSender;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class PendingEmailVerificationUserFailureIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired CapturingMailSender mailSender;
  @Autowired AccountRepository accounts;
  @Autowired GuestConversionOperationRepository operations;
  @MockBean PrimaryProfileProvisioner primaryProfiles;

  @BeforeEach
  void provisionProfilesForRegistration() {
    when(primaryProfiles.ensurePrimaryProfile(any(), any(), anyBoolean()))
        .thenAnswer(invocation -> UUID.randomUUID().toString());
  }

  @Test
  void userPromotionFailureReturnsPendingAndKeepsFreshEmailAccountGuestForDurableRetry()
      throws Exception {
    String email = "pending-user-failure@example.com";
    JsonNode registered =
        objectMapper.readTree(
            mockMvc
                .perform(
                    post("/api/v1/auth/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(
                            "{\"email\":\""
                                + email
                                + "\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
                .andExpect(status().isOk())
                .andReturn()
                .getResponse()
                .getContentAsString());
    UUID accountId = UUID.fromString(registered.path("session").path("account_id").asText());

    mailSender.clear();
    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"" + email + "\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isNoContent());

    doThrow(new AuthException("auth_unavailable"))
        .when(primaryProfiles)
        .clearGuestAccountFlag(accountId);

    mockMvc
        .perform(
            post("/api/v1/auth/otp/verify")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\""
                        + email
                        + "\",\"code\":\""
                        + mailSender.lastCode()
                        + "\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isServiceUnavailable())
        .andExpect(jsonPath("$.error").value("verification_pending"));

    assertThat(accounts.findById(accountId.toString())).get().extracting(account -> account.type())
        .isEqualTo("guest");
    assertThat(accounts.isRegularEmailVerificationPending(accountId)).isTrue();
    assertThat(operations.findByAccountId(accountId))
        .get()
        .satisfies(
            operation -> {
              assertThat(operation.state()).isEqualTo(GuestConversionState.PENDING_USER);
              assertThat(operation.attemptCount()).isEqualTo(1);
              assertThat(operation.lockedUntil()).isNull();
              assertThat(operation.nextAttemptAt()).isAfter(operation.createdAt());
            });
  }
}
