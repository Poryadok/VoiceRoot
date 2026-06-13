package voice.backend.auth.service;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;
import voice.backend.auth.repository.BackupCodeRepository;

public class BackupCodeService {
  private static final int DEFAULT_CODES = 10;
  private static final int CODE_LEN = 10;

  private final BackupCodeRepository repository;
  private final SecureRandom secureRandom = new SecureRandom();

  public BackupCodeService(BackupCodeRepository repository) {
    this.repository = repository;
  }

  public List<String> generateAndStore(UUID accountId) {
    List<String> plain = new ArrayList<>(DEFAULT_CODES);
    List<String> hashes = new ArrayList<>(DEFAULT_CODES);
    for (int i = 0; i < DEFAULT_CODES; i++) {
      String code = randomCode();
      plain.add(code);
      hashes.add(sha256(code));
    }
    repository.replaceCodes(accountId, hashes);
    return plain;
  }

  public boolean consume(UUID accountId, String plainCode) {
    if (plainCode == null || plainCode.isBlank()) {
      return false;
    }
    return repository.consumeCode(accountId, sha256(plainCode.trim()));
  }

  private String randomCode() {
    final String alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
    StringBuilder out = new StringBuilder(CODE_LEN);
    for (int i = 0; i < CODE_LEN; i++) {
      out.append(alphabet.charAt(secureRandom.nextInt(alphabet.length())));
    }
    return out.toString();
  }

  private static String sha256(String value) {
    try {
      MessageDigest digest = MessageDigest.getInstance("SHA-256");
      return HexFormat.of().formatHex(digest.digest(value.getBytes()));
    } catch (NoSuchAlgorithmException ex) {
      throw new IllegalStateException("sha256 not available", ex);
    }
  }
}
