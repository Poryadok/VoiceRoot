package voice.backend.auth.oauth;

import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import org.springframework.http.MediaType;
import org.springframework.util.MultiValueMap;

public final class FormUrlEncodedTestSupport {
  private FormUrlEncodedTestSupport() {}

  public static org.springframework.test.web.servlet.request.MockHttpServletRequestBuilder postForm(
      org.springframework.test.web.servlet.request.MockHttpServletRequestBuilder builder,
      MultiValueMap<String, String> form) {
    return builder.contentType(MediaType.APPLICATION_FORM_URLENCODED).content(encode(form));
  }

  public static String encode(MultiValueMap<String, String> form) {
    StringBuilder body = new StringBuilder();
    form.forEach(
        (key, values) -> {
          for (String value : values) {
            if (body.length() > 0) {
              body.append('&');
            }
            body.append(URLEncoder.encode(key, StandardCharsets.UTF_8));
            body.append('=');
            body.append(URLEncoder.encode(value == null ? "" : value, StandardCharsets.UTF_8));
          }
        });
    return body.toString();
  }
}
