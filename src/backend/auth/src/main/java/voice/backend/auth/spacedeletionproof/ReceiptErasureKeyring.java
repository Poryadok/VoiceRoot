package voice.backend.auth.spacedeletionproof;

import java.util.List;

/** Auth-only key provider. Deployments must back this with the dedicated KMS/HSM family. */
public interface ReceiptErasureKeyring {
  int activeVersion();
  List<Integer> retainedVersions();
  byte[] requireKey(int version);

  static ReceiptErasureKeyring unavailable() {
    return new ReceiptErasureKeyring() {
      @Override public int activeVersion() { throw new ReceiptErasureKeyUnavailableException(0); }
      @Override public List<Integer> retainedVersions() { return List.of(); }
      @Override public byte[] requireKey(int version) {
        throw new ReceiptErasureKeyUnavailableException(version);
      }
    };
  }
}
