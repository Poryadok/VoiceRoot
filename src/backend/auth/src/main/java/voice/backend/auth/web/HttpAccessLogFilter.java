package voice.backend.auth.web;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import jakarta.servlet.http.HttpServletResponseWrapper;
import java.io.IOException;
import java.time.Duration;
import java.time.Instant;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;
import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

@Component
@Order(Ordered.HIGHEST_PRECEDENCE + 1)
public class HttpAccessLogFilter extends OncePerRequestFilter {
  private static final Logger log = LoggerFactory.getLogger(HttpAccessLogFilter.class);
  private static final String[] HTTP_MDC_KEYS = {
    "event", "method", "path", "status", "duration_ms"
  };

  @Override
  protected void doFilterInternal(
      HttpServletRequest request, HttpServletResponse response, FilterChain filterChain)
      throws ServletException, IOException {
    Instant start = Instant.now();
    StatusCaptureResponse wrapped = new StatusCaptureResponse(response);
    try {
      filterChain.doFilter(request, wrapped);
    } finally {
      long durationMs = Duration.between(start, Instant.now()).toMillis();
      MDC.put("event", "http_access");
      MDC.put("method", request.getMethod());
      MDC.put("path", request.getRequestURI());
      MDC.put("status", Integer.toString(wrapped.captureStatus()));
      MDC.put("duration_ms", Long.toString(durationMs));
      try {
        log.info("http request");
      } finally {
        clearHttpMdc();
      }
    }
  }

  private static void clearHttpMdc() {
    for (String key : HTTP_MDC_KEYS) {
      MDC.remove(key);
    }
  }

  private static final class StatusCaptureResponse extends HttpServletResponseWrapper {
    private int status = HttpServletResponse.SC_OK;

    private StatusCaptureResponse(HttpServletResponse response) {
      super(response);
    }

    @Override
    public void setStatus(int sc) {
      status = sc;
      super.setStatus(sc);
    }

    @Override
    public void sendError(int sc) throws IOException {
      status = sc;
      super.sendError(sc);
    }

    @Override
    public void sendError(int sc, String msg) throws IOException {
      status = sc;
      super.sendError(sc, msg);
    }

    int captureStatus() {
      return status;
    }
  }
}
