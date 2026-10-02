package voice.backend.auth.authoritysource;

import static org.assertj.core.api.Assertions.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.*;
import org.junit.jupiter.api.Test;

class AuthSourceStateParityTest {
  @SuppressWarnings("unchecked")
  @Test void grantsMustMatchIdentityEnvironmentAndExactAuthorizationSource()throws Exception {
    var raw=Files.readAllBytes(Path.of("../pkg/authoritysource/testdata/auth-state-v1.json"));
    for(String field:List.of("environment_id","authorization_request_id")) {
      Map<String,Object> state=new ObjectMapper().readValue(raw,LinkedHashMap.class);
      var requested=AuthSourceState.group(state,"accounts").stream().map(row->AuthSourceState.text(row,"account_id")).toList();
      AuthSourceState.validate(state,requested);
      AuthSourceState.group(state,"sdk_message_grants").get(0).put(field,"10000000-0000-4000-8000-000000000099");
      assertThatThrownBy(()->AuthSourceState.validate(state,requested)).as(field).isInstanceOf(IllegalStateException.class);
    }
  }
}
