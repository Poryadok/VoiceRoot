package voice.backend.auth.mail;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class ResendMailSenderTest {
  private HttpServer server;
  private URI endpoint;
  private final AtomicReference<String> lastAuth = new AtomicReference<>();
  private final AtomicReference<String> lastBody = new AtomicReference<>();
  private int statusCode = 200;

  @BeforeEach
  void startServer() throws IOException {
    server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext(
        "/emails",
        exchange -> {
          lastAuth.set(exchange.getRequestHeaders().getFirst("Authorization"));
          lastBody.set(new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
          byte[] response = "{\"id\":\"re_test\"}".getBytes(StandardCharsets.UTF_8);
          exchange.sendResponseHeaders(statusCode, response.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(response);
          }
        });
    server.start();
    endpoint = URI.create("http://127.0.0.1:" + server.getAddress().getPort() + "/emails");
  }

  @AfterEach
  void stopServer() {
    if (server != null) {
      server.stop(0);
    }
  }

  @Test
  void postsJsonPayloadToResend() throws Exception {
    ResendMailSender sender =
        new ResendMailSender(
            "re_test_key",
            "Voice <noreply@example.com>",
            java.net.http.HttpClient.newHttpClient(),
            new ObjectMapper(),
            endpoint);

    sender.sendOtpEmail("user@example.com", "Reset your Voice password", "Your Voice verification code is 123456");

    assertThat(lastAuth.get()).isEqualTo("Bearer re_test_key");
    JsonNode body = new ObjectMapper().readTree(lastBody.get());
    assertThat(body.get("from").asText()).isEqualTo("Voice <noreply@example.com>");
    assertThat(body.get("to").get(0).asText()).isEqualTo("user@example.com");
    assertThat(body.get("subject").asText()).isEqualTo("Reset your Voice password");
    assertThat(body.get("text").asText()).contains("123456");
  }

  @Test
  void failsOnNonSuccessStatus() {
    statusCode = 401;
    ResendMailSender sender =
        new ResendMailSender(
            "re_bad",
            "Voice <noreply@example.com>",
            java.net.http.HttpClient.newHttpClient(),
            new ObjectMapper(),
            endpoint);

    assertThatThrownBy(() -> sender.sendOtpEmail("user@example.com", "subj", "body"))
        .isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("resend_send_failed");
  }
}
