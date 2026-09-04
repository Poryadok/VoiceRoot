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
import org.springframework.context.ApplicationContext;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import voice.backend.auth.support.CapturingMailSender;
import voice.backend.auth.support.RecordingAuthEventPublisher;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.service.GuestConversionPendingUserRecoveryRunner;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class GuestConvertNatsEventIntegrationTest {
  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired ApplicationContext applicationContext;
  @Autowired CapturingMailSender mailSender;
  @Autowired GuestConversionOperationRepository operations;
  @Autowired GuestConversionPendingUserRecoveryRunner pendingUserRecovery;
  @Autowired Clock clock;

  @Test
  void emailOtpCompletionDefersEventUntilTheSeparatePendingEventPublisher() throws Exception {
    RecordingAuthEventPublisher events = findRecordingPublisher(applicationContext);
    assertThat(events).isNotNull();
    events.clear();

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
                    "{\"email\":\"nats-guest@example.com\",\"password\":\"New account password 1\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_id", is(guest.get("account_id").asText())))
        .andExpect(jsonPath("$.session.account_type", is("guest")));

    assertThat(events.publishedSubjects())
        .as("convert-guest submit must not publish before email verification")
        .doesNotContain("user.guest_converted");

    mailSender.clear();
    mockMvc
        .perform(
            post("/api/v1/auth/otp/send")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"nats-guest@example.com\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isNoContent());
    String code = mailSender.lastCode();
    assertThat(code).matches("\\d{6}");

    mockMvc
        .perform(
            post("/api/v1/auth/otp/verify")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"nats-guest@example.com\",\"code\":\""
                        + code
                        + "\",\"otp_type\":\"email_verify\"}"))
        .andExpect(status().isNoContent());

    assertThat(events.publishedSubjects())
        .as("verified email OTP creates PENDING_USER work but does not publish from the request path")
        .doesNotContain("user.guest_converted");

    pendingUserRecovery.tick();

    mockMvc
        .perform(
            post("/api/v1/auth/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content(
                    "{\"email\":\"nats-guest@example.com\",\"password\":\"New account password 1\",\"device_info_json\":\"{}\"}"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.session.account_id", is(guest.get("account_id").asText())))
        .andExpect(jsonPath("$.session.account_type", is("regular")));

    Instant now = Instant.now(clock);
    assertThat(operations.leaseDue(1, now, now.plus(Duration.ofMinutes(1))))
        .singleElement()
        .satisfies(
            operation -> {
              assertThat(operation.accountId()).isEqualTo(UUID.fromString(guest.get("account_id").asText()));
              assertThat(operation.state()).isEqualTo(GuestConversionState.PENDING_EVENT);
            });
    assertThat(events.publishedSubjects())
        .as("PENDING_USER recovery must not leak into the future PENDING_EVENT publisher")
        .doesNotContain("user.guest_converted");
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

  private static RecordingAuthEventPublisher findRecordingPublisher(ApplicationContext ctx) {
    for (String name : ctx.getBeanDefinitionNames()) {
      Object bean = ctx.getBean(name);
      if (bean instanceof RecordingAuthEventPublisher recording) {
        return recording;
      }
    }
    return null;
  }
}
