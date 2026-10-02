package voice.backend.auth.authoritysource;

import com.google.protobuf.MessageLite;
import io.grpc.*;
import java.util.Set;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;

final class AuthSourceInterceptor implements ServerInterceptor {
  private final AuthSourceVerifier verifier;
  AuthSourceInterceptor(AuthSourceVerifier verifier){this.verifier=verifier;}
  @Override public <ReqT,RespT> ServerCall.Listener<ReqT> interceptCall(ServerCall<ReqT,RespT> call,Metadata headers,ServerCallHandler<ReqT,RespT> next) {
    String rpc="/"+call.getMethodDescriptor().getFullMethodName();
    final String credential,requestId;
    try {
      if(!Set.of(AuthSourceVerifier.SNAPSHOT,AuthSourceVerifier.REVISION).contains(rpc))throw AuthSourceVerifier.invalid();
      for(String key:headers.keys()) if(key.startsWith("x-voice-") || Set.of("x-account-id","x-profile-id","x-user-id","x-actor-id","x-internal-caller").contains(key))throw AuthSourceVerifier.invalid();
      String authorization=single(headers,"authorization");requestId=single(headers,"x-request-id");
      if(!authorization.startsWith("Bearer ") || authorization.length()<=7 || authorization.substring(7).chars().anyMatch(Character::isWhitespace))throw AuthSourceVerifier.invalid();
      credential=authorization.substring(7);
    } catch(RuntimeException ex){call.close(Status.UNAUTHENTICATED.withDescription("Auth source principal rejected"),new Metadata());return new ServerCall.Listener<>(){};}
    Context parent=Context.current();call.request(2);
    return new ServerCall.Listener<>() {
      ReqT pending;boolean received,closed;ServerCall.Listener<ReqT> delegate;
      @Override public void onMessage(ReqT message){if(closed)return;if(received || !(message instanceof MessageLite)){deny();return;}received=true;pending=message;}
      @Override public void onHalfClose(){
        if(closed)return;if(!received || pending==null){deny();return;}
        ReqT message=pending;pending=null;
        try {
          var verified=verifier.verify(credential,rpc,requestId,AuthPrincipalServerInterceptor.requestHash((MessageLite)message));
          delegate=Contexts.interceptCall(parent.withValue(AuthSourceVerifier.Verified.CONTEXT,verified),call,headers,next);
          delegate.onMessage(message);delegate.onHalfClose();
        } catch(RuntimeException ex){deny();}
      }
      @Override public void onCancel(){closed=true;pending=null;if(delegate!=null)delegate.onCancel();}
      @Override public void onComplete(){closed=true;if(delegate!=null)delegate.onComplete();}
      @Override public void onReady(){if(!closed && delegate!=null)delegate.onReady();}
      private void deny(){closed=true;pending=null;call.close(Status.UNAUTHENTICATED.withDescription("Auth source principal rejected"),new Metadata());}
    };
  }
  private static String single(Metadata headers,String name) {
    var values=headers.getAll(Metadata.Key.of(name,Metadata.ASCII_STRING_MARSHALLER));if(values==null)throw AuthSourceVerifier.invalid();
    var iterator=values.iterator();if(!iterator.hasNext())throw AuthSourceVerifier.invalid();String value=iterator.next();
    if(iterator.hasNext() || value==null || value.isBlank())throw AuthSourceVerifier.invalid();return value;
  }
}
