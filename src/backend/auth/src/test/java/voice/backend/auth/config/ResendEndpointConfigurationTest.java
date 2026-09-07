package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThat;

import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.boot.context.properties.bind.Bindable;
import org.springframework.boot.context.properties.bind.Binder;
import org.springframework.boot.context.properties.source.MapConfigurationPropertySource;
import voice.backend.auth.mail.MailSender;

class ResendEndpointConfigurationTest {
  private HttpServer server;
  private URI endpoint;
  private final AtomicReference<String> requestMethod = new AtomicReference<>();
  private final AtomicReference<String> requestPath = new AtomicReference<>();
  private final AtomicReference<String> requestBody = new AtomicReference<>();

  @BeforeEach
  void startServer() throws IOException {
    server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext(
        "/resend/emails",
        exchange -> {
          requestMethod.set(exchange.getRequestMethod());
          requestPath.set(exchange.getRequestURI().getPath());
          requestBody.set(
              new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
          byte[] response = "{\"id\":\"re_compose\"}".getBytes(StandardCharsets.UTF_8);
          exchange.sendResponseHeaders(200, response.length);
          try (OutputStream output = exchange.getResponseBody()) {
            output.write(response);
          }
        });
    server.start();
    endpoint =
        URI.create("http://127.0.0.1:" + server.getAddress().getPort() + "/resend/emails");
  }

  @AfterEach
  void stopServer() {
    if (server != null) {
      server.stop(0);
    }
  }

  @Test
  void keepsTheProductionResendEndpointByDefault() {
    assertThat(new AuthProperties().getResend().getEndpoint())
        .isEqualTo("https://api.resend.com/emails");
  }

  @Test
  void bindsCustomEndpointAndUsesItForResendDelivery() {
    var source =
        new MapConfigurationPropertySource(
            Map.of(
                "auth.resend.api-key", "re_compose_key",
                "auth.resend.from", "Voice <noreply@voice.test>",
                "auth.resend.endpoint", endpoint.toString()));
    AuthProperties properties =
        new Binder(source).bind("auth", Bindable.of(AuthProperties.class)).get();

    MailSender sender = new OtpAndMailConfiguration().mailSender(properties);
    sender.sendOtpEmail(
        "verification-user@voice-qa.test",
        "Verify your Voice email",
        "Your Voice verification code is 654321");

    assertThat(properties.getResend().getEndpoint()).isEqualTo(endpoint.toString());
    assertThat(requestMethod.get()).isEqualTo("POST");
    assertThat(requestPath.get()).isEqualTo("/resend/emails");
    assertThat(requestBody.get())
        .contains("verification-user@voice-qa.test")
        .contains("654321");
  }
}
