package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.time.ZoneOffset;
import java.util.*;
import org.junit.jupiter.api.Test;

class AuthModerationPrincipalTest {
  @Test void exactServiceCallerMatrixNeverAcceptsDelegatedOrRawIdentity() {
    String prefix="/voice.automod.v1.AuthModerationAuthorityService/";
    for(String method:List.of("ResolveModerationSubject","ListTerminalModerationSubjects","ReconcileModerationRestriction")) {
      for(String issuer:List.of("messaging","moderation","gateway","space","voice")) {
        var verifier=new AuthPrincipalVerifier((i,k)->(RSAPublicKey)AuthPrincipalVerifierTest.KEY.getPublic(),
            account->{throw new AssertionError("service authority must not look up delegated epoch");},
            (i,j,e)->{},Clock.fixed(AuthPrincipalVerifierTest.NOW,ZoneOffset.UTC));
        Map<String,Object> claims=AuthPrincipalVerifierTest.claims();
        claims.put("iss",issuer);claims.put("sub","service:"+issuer);claims.put("principal_type","service");
        claims.remove("account_id");claims.remove("profile_id");claims.remove("session_epoch");claims.put("rpc",prefix+method);
        boolean allowed=issuer.equals("moderation") || issuer.equals("messaging") && !method.equals("ReconcileModerationRestriction");
        String token=AuthPrincipalVerifierTest.token(claims);
        if(allowed) assertEquals(issuer,verifier.verify(token,prefix+method,"request-1","sha256:"+"0".repeat(64)).issuer());
        else assertEquals(Status.Code.PERMISSION_DENIED,assertThrows(StatusRuntimeException.class,
            ()->verifier.verify(token,prefix+method,"request-1","sha256:"+"0".repeat(64))).getStatus().getCode());
        claims.put("account_id",UUID.randomUUID().toString());
        if(issuer.equals("messaging") || issuer.equals("moderation")) assertEquals(Status.Code.UNAUTHENTICATED,
            assertThrows(StatusRuntimeException.class,()->verifier.verify(AuthPrincipalVerifierTest.token(claims),prefix+method,"request-1","sha256:"+"0".repeat(64))).getStatus().getCode());
      }
    }
  }
}
