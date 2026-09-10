package voice.backend.auth.principal;

import com.google.protobuf.CodedOutputStream;
import com.google.protobuf.MessageLite;
import io.grpc.*;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.Set;

/** Buffers the unary message before creating any protected handler. */
public final class AuthPrincipalServerInterceptor implements ServerInterceptor, AutoCloseable {
  public static final String ISSUE_RPC = "/voice.auth.v1.AuthService/IssueOwnershipTransferProof";
  public static final String CONSUME_RPC = "/voice.auth.v1.AuthService/ConsumeOwnershipTransferProof";
  public static final String LOOKUP_RPC = "/voice.auth.v1.AuthService/GetOwnershipTransferReceipt";
  private static final Set<String> RAW = Set.of("x-profile-id", "x-account-id", "x-user-id", "x-actor-id", "x-internal-caller");
  private final AuthPrincipalVerifier verifier;
  private final Runnable cleanup;

  public AuthPrincipalServerInterceptor(AuthPrincipalVerifier verifier) { this(verifier, () -> {}); }
  AuthPrincipalServerInterceptor(AuthPrincipalVerifier verifier, Runnable cleanup) {
    this.verifier = verifier;
    this.cleanup = cleanup;
  }
  @Override public void close() { cleanup.run(); }
  public boolean enabled() { return verifier != null; }

  @Override public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
      ServerCall<ReqT, RespT> call, Metadata headers, ServerCallHandler<ReqT, RespT> next) {
    String rpc = "/" + call.getMethodDescriptor().getFullMethodName();
    if (!ISSUE_RPC.equals(rpc) && !CONSUME_RPC.equals(rpc) && !LOOKUP_RPC.equals(rpc)) return next.startCall(call, headers);
    final String token, requestId;
    try {
      if (verifier == null) throw new IllegalArgumentException();
      for (String key : headers.keys()) {
        if (key.startsWith("x-voice-") || RAW.contains(key)) throw new IllegalArgumentException();
      }
      String authorization = single(headers, "authorization");
      requestId = single(headers, "x-request-id");
      if (!authorization.startsWith("Bearer ") || authorization.length() <= 7
          || authorization.substring(7).chars().anyMatch(Character::isWhitespace)) throw new IllegalArgumentException();
      token = authorization.substring(7);
    } catch (RuntimeException ex) {
      call.close(Status.UNAUTHENTICATED.withDescription("invalid principal"), new Metadata());
      return new ServerCall.Listener<>() {};
    }
    Context parent = Context.current();
    // Request one message plus an excess-message sentinel before creating the generated handler.
    call.request(2);
    return new ServerCall.Listener<>() {
      private ServerCall.Listener<ReqT> delegate;
      private boolean closed, received;
      private ReqT pending;
      @Override public void onMessage(ReqT request) {
        if (closed) return;
        if (received || !(request instanceof MessageLite)) { deny(Status.UNAUTHENTICATED); return; }
        received = true;
        pending = request;
      }
      @Override public void onHalfClose() {
        if (closed) return;
        if (!received || pending == null) { deny(Status.UNAUTHENTICATED); return; }
        ReqT request = pending;
        pending = null;
        final VerifiedPrincipal verified;
        try {
          verified = verifier.verify(token, rpc, requestId, requestHash((MessageLite) request));
        } catch (RuntimeException ex) {
          Status status = Status.fromThrowable(ex);
          deny(status.getCode() == Status.Code.PERMISSION_DENIED ? Status.PERMISSION_DENIED : Status.UNAUTHENTICATED);
          return;
        }
        delegate = Contexts.interceptCall(parent.withValue(VerifiedPrincipal.CONTEXT, verified), call, headers, next);
        delegate.onMessage(request);
        delegate.onHalfClose();
      }
      @Override public void onCancel() { closed = true; pending = null; if (delegate != null) delegate.onCancel(); }
      @Override public void onComplete() { closed = true; if (delegate != null) delegate.onComplete(); }
      @Override public void onReady() { if (!closed && delegate != null) delegate.onReady(); }
      private void deny(Status status) {
        closed = true;
        call.close(status.withDescription("principal rejected"), new Metadata());
      }
    };
  }

  private static String single(Metadata headers, String name) {
    var values = headers.getAll(Metadata.Key.of(name, Metadata.ASCII_STRING_MARSHALLER));
    if (values == null) throw new IllegalArgumentException();
    var iterator = values.iterator();
    if (!iterator.hasNext()) throw new IllegalArgumentException();
    String value = iterator.next();
    if (iterator.hasNext() || value == null || value.isBlank()) throw new IllegalArgumentException();
    return value;
  }

  public static String requestHash(MessageLite message) {
    try {
      byte[] bytes = new byte[message.getSerializedSize()];
      CodedOutputStream output = CodedOutputStream.newInstance(bytes);
      output.useDeterministicSerialization();
      message.writeTo(output);
      output.checkNoSpaceLeft();
      return "sha256:" + HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes));
    } catch (Exception ex) {
      throw new IllegalArgumentException("invalid protobuf request");
    }
  }
}
