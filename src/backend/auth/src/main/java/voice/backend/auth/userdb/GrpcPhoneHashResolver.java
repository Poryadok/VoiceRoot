package voice.backend.auth.userdb;

import app.voice.user.v1.ResolvePrimaryProfileIDsRequest;
import app.voice.user.v1.UserServiceGrpc;
import io.grpc.StatusRuntimeException;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.service.AuthException;

/** Resolves Auth-owned phone hashes through the User Service account-to-primary-profile seam. */
public final class GrpcPhoneHashResolver implements PhoneHashResolver {
  private final UserServiceGrpc.UserServiceBlockingStub stub;
  private final AccountRepository accounts;

  public GrpcPhoneHashResolver(
      UserServiceGrpc.UserServiceBlockingStub stub, AccountRepository accounts) {
    this.stub = stub;
    this.accounts = accounts;
  }

  @Override
  public Map<String, String> resolvePrimaryProfileIdsByPhoneHashes(Collection<String> phoneHashes) {
    if (phoneHashes == null || phoneHashes.isEmpty()) {
      return Map.of();
    }

    Map<String, UUID> accountByHash = new LinkedHashMap<>();
    for (String rawHash : phoneHashes) {
      if (rawHash == null || rawHash.isBlank()) {
        continue;
      }
      String hash = rawHash.trim();
      accounts
          .findByPhone(hash)
          .filter(GrpcPhoneHashResolver::isActive)
          .ifPresent(account -> accountByHash.putIfAbsent(hash, account.id()));
    }
    if (accountByHash.isEmpty()) {
      return Map.of();
    }

    try {
      var request = ResolvePrimaryProfileIDsRequest.newBuilder();
      accountByHash.values().stream().distinct().map(UUID::toString).forEach(request::addAccountIds);
      Map<String, String> profileByAccount =
          stub.resolvePrimaryProfileIDs(request.build()).getPrimaryProfileIdsMap();
      Map<String, String> resolved = new LinkedHashMap<>();
      for (Map.Entry<String, UUID> entry : accountByHash.entrySet()) {
        String profileId = profileByAccount.get(entry.getValue().toString());
        if (profileId != null) {
          resolved.put(entry.getKey(), UUID.fromString(profileId).toString());
        }
      }
      return resolved;
    } catch (StatusRuntimeException ex) {
      throw new AuthException("auth_unavailable");
    } catch (IllegalArgumentException ex) {
      throw new AuthException("malformed_user_response");
    }
  }

  private static boolean isActive(Account account) {
    return "active".equals(account.status()) && account.deletedAt() == null;
  }
}
